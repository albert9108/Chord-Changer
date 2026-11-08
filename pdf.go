package main

// Minimal PDF reader/writer (stdlib only). Reads classic + hybrid + xref-stream
// files (incl. object streams), lets us replace objects, and rewrites the file.

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
)

type Name string
type Num string
type Bool bool
type Null struct{}
type Str struct{ Raw []byte } // raw incl. delimiters
type Ref struct{ N, G int }
type Arr []any
type Dict struct {
	Keys []string
	M    map[string]any
}
type Stream struct {
	D    *Dict
	Data []byte // raw (still encoded)
}

func NewDict() *Dict             { return &Dict{M: map[string]any{}} }
func (d *Dict) Get(k string) any { return d.M[k] }
func (d *Dict) Set(k string, v any) {
	if _, ok := d.M[k]; !ok {
		d.Keys = append(d.Keys, k)
	}
	d.M[k] = v
}
func (d *Dict) Del(k string) {
	if _, ok := d.M[k]; !ok {
		return
	}
	delete(d.M, k)
	for i, x := range d.Keys {
		if x == k {
			d.Keys = append(d.Keys[:i], d.Keys[i+1:]...)
			break
		}
	}
}
func (d *Dict) Clone() *Dict {
	n := NewDict()
	for _, k := range d.Keys {
		n.Set(k, d.M[k])
	}
	return n
}

// ---------- lexer / parser ----------

type parser struct {
	b   []byte
	pos int
}

func isWS(c byte) bool { return c == 0 || c == 9 || c == 10 || c == 12 || c == 13 || c == 32 }
func isDelim(c byte) bool {
	return c == '(' || c == ')' || c == '<' || c == '>' || c == '[' || c == ']' || c == '{' || c == '}' || c == '/' || c == '%'
}

func (p *parser) skipWS() {
	for p.pos < len(p.b) {
		c := p.b[p.pos]
		if isWS(c) {
			p.pos++
		} else if c == '%' {
			for p.pos < len(p.b) && p.b[p.pos] != '\n' && p.b[p.pos] != '\r' {
				p.pos++
			}
		} else {
			return
		}
	}
}

func (p *parser) token() string {
	p.skipWS()
	s := p.pos
	for p.pos < len(p.b) && !isWS(p.b[p.pos]) && !isDelim(p.b[p.pos]) {
		p.pos++
	}
	return string(p.b[s:p.pos])
}

func isNumTok(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c == '.' || c == '-' || c == '+') {
			return false
		}
	}
	return true
}
func isIntTok(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

var errEOF = errors.New("unexpected end of PDF data")

func (p *parser) parse() (any, error) {
	p.skipWS()
	if p.pos >= len(p.b) {
		return nil, errEOF
	}
	c := p.b[p.pos]
	switch {
	case c == '/':
		p.pos++
		s := p.pos
		for p.pos < len(p.b) && !isWS(p.b[p.pos]) && !isDelim(p.b[p.pos]) {
			p.pos++
		}
		return Name(p.b[s:p.pos]), nil
	case c == '<' && p.pos+1 < len(p.b) && p.b[p.pos+1] == '<':
		p.pos += 2
		d := NewDict()
		for {
			p.skipWS()
			if p.pos+1 < len(p.b) && p.b[p.pos] == '>' && p.b[p.pos+1] == '>' {
				p.pos += 2
				return d, nil
			}
			k, err := p.parse()
			if err != nil {
				return nil, err
			}
			kn, ok := k.(Name)
			if !ok {
				return nil, fmt.Errorf("bad dict key at %d", p.pos)
			}
			v, err := p.parse()
			if err != nil {
				return nil, err
			}
			d.Set(string(kn), v)
		}
	case c == '<':
		s := p.pos
		e := bytes.IndexByte(p.b[s:], '>')
		if e < 0 {
			return nil, errEOF
		}
		p.pos = s + e + 1
		return Str{Raw: p.b[s:p.pos]}, nil
	case c == '(':
		s := p.pos
		depth := 0
		for p.pos < len(p.b) {
			ch := p.b[p.pos]
			if ch == '\\' {
				p.pos += 2
				continue
			}
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
				if depth == 0 {
					p.pos++
					return Str{Raw: p.b[s:p.pos]}, nil
				}
			}
			p.pos++
		}
		return nil, errEOF
	case c == '[':
		p.pos++
		a := Arr{}
		for {
			p.skipWS()
			if p.pos < len(p.b) && p.b[p.pos] == ']' {
				p.pos++
				return a, nil
			}
			v, err := p.parse()
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
	}
	t := p.token()
	if t == "" {
		p.pos++
		return nil, fmt.Errorf("unexpected byte %q at %d", c, p.pos-1)
	}
	switch t {
	case "true":
		return Bool(true), nil
	case "false":
		return Bool(false), nil
	case "null":
		return Null{}, nil
	}
	if isIntTok(t) {
		save := p.pos
		t2 := p.token()
		if isIntTok(t2) {
			t3 := p.token()
			if t3 == "R" {
				n, _ := strconv.Atoi(t)
				g, _ := strconv.Atoi(t2)
				return Ref{n, g}, nil
			}
		}
		p.pos = save
		return Num(t), nil
	}
	if isNumTok(t) {
		return Num(t), nil
	}
	return Name("__kw:" + t), nil // unexpected keyword
}

// ---------- document ----------

type xent struct {
	typ int // 1 = offset, 2 = in objstm
	off int
	stm int
	idx int
}

type PDF struct {
	b        []byte
	xref     map[int]xent
	Trailer  *Dict
	cache    map[int]any
	rawSpan  map[int][2]int // for type-1 objects: [start,end) of "N G obj ... endobj"
	objstm   map[int][][]byte
	Modified map[int]any
}

func intOf(o any) int {
	switch v := o.(type) {
	case Num:
		f, _ := strconv.ParseFloat(string(v), 64)
		return int(f)
	}
	return 0
}
func floatOf(o any) float64 {
	if v, ok := o.(Num); ok {
		f, _ := strconv.ParseFloat(string(v), 64)
		return f
	}
	return 0
}

func OpenPDF(b []byte) (*PDF, error) {
	d := &PDF{b: b, xref: map[int]xent{}, cache: map[int]any{}, rawSpan: map[int][2]int{}, objstm: map[int][][]byte{}, Modified: map[int]any{}}
	i := bytes.LastIndex(b, []byte("startxref"))
	if i < 0 {
		return nil, errors.New("not a PDF (no startxref)")
	}
	p := &parser{b: b, pos: i + 9}
	off, err := strconv.Atoi(p.token())
	if err != nil {
		return nil, errors.New("bad startxref")
	}
	seen := map[int]bool{}
	for off > 0 && !seen[off] {
		seen[off] = true
		tr, err := d.readXref(off)
		if err != nil {
			return nil, err
		}
		if d.Trailer == nil {
			d.Trailer = tr
		}
		if xs, ok := tr.Get("XRefStm").(Num); ok {
			if _, err := d.readXref(intOf(xs)); err != nil {
				return nil, err
			}
		}
		off = intOf(tr.Get("Prev"))
	}
	if d.Trailer == nil {
		return nil, errors.New("no trailer")
	}
	if d.Trailer.Get("Encrypt") != nil {
		return nil, errors.New("this PDF is encrypted/protected — please export it again without a password")
	}
	return d, nil
}

func (d *PDF) readXref(off int) (*Dict, error) {
	if off >= len(d.b) {
		return nil, errors.New("bad xref offset")
	}
	p := &parser{b: d.b, pos: off}
	p.skipWS()
	if bytes.HasPrefix(d.b[p.pos:], []byte("xref")) {
		p.pos += 4
		for {
			save := p.pos
			t := p.token()
			if t == "trailer" {
				o, err := p.parse()
				if err != nil {
					return nil, err
				}
				td, ok := o.(*Dict)
				if !ok {
					return nil, errors.New("bad trailer")
				}
				return td, nil
			}
			if !isIntTok(t) {
				p.pos = save
				return nil, errors.New("bad xref table")
			}
			start, _ := strconv.Atoi(t)
			cnt, _ := strconv.Atoi(p.token())
			for k := 0; k < cnt; k++ {
				o, _ := strconv.Atoi(p.token())
				p.token()
				f := p.token()
				n := start + k
				if _, ok := d.xref[n]; !ok {
					if f == "n" {
						d.xref[n] = xent{typ: 1, off: o}
					} else {
						d.xref[n] = xent{typ: 0}
					}
				}
			}
		}
	}
	// xref stream
	_, v, _, err := d.parseIndirectAt(off)
	if err != nil {
		return nil, err
	}
	s, ok := v.(*Stream)
	if !ok {
		return nil, errors.New("bad xref stream")
	}
	data, err := d.Decode(s)
	if err != nil {
		return nil, err
	}
	w := s.D.Get("W").(Arr)
	w0, w1, w2 := intOf(w[0]), intOf(w[1]), intOf(w[2])
	idx := Arr{Num("0"), s.D.Get("Size")}
	if a, ok := s.D.Get("Index").(Arr); ok {
		idx = a
	}
	pos := 0
	rd := func(n int) int {
		v := 0
		for i := 0; i < n; i++ {
			v = v<<8 | int(data[pos])
			pos++
		}
		return v
	}
	for k := 0; k+1 < len(idx); k += 2 {
		st, cnt := intOf(idx[k]), intOf(idx[k+1])
		for j := 0; j < cnt; j++ {
			if pos+w0+w1+w2 > len(data) {
				break
			}
			t := 1
			if w0 > 0 {
				t = rd(w0)
			}
			a, b := rd(w1), rd(w2)
			n := st + j
			if _, ok := d.xref[n]; ok {
				continue
			}
			switch t {
			case 1:
				d.xref[n] = xent{typ: 1, off: a}
			case 2:
				d.xref[n] = xent{typ: 2, stm: a, idx: b}
			default:
				d.xref[n] = xent{typ: 0}
			}
		}
	}
	return s.D, nil
}

// parse "N G obj <value> [stream..endstream] endobj" at offset
func (d *PDF) parseIndirectAt(off int) (int, any, int, error) {
	p := &parser{b: d.b, pos: off}
	nt := p.token()
	p.token()
	if p.token() != "obj" {
		return 0, nil, 0, fmt.Errorf("no object at offset %d", off)
	}
	n, _ := strconv.Atoi(nt)
	v, err := p.parse()
	if err != nil {
		return 0, nil, 0, err
	}
	p.skipWS()
	if dd, ok := v.(*Dict); ok && bytes.HasPrefix(d.b[p.pos:], []byte("stream")) {
		p.pos += 6
		if p.pos < len(d.b) && d.b[p.pos] == '\r' {
			p.pos++
		}
		if p.pos < len(d.b) && d.b[p.pos] == '\n' {
			p.pos++
		}
		ln := -1
		switch l := dd.Get("Length").(type) {
		case Num:
			ln = intOf(l)
		case Ref:
			if lv, err := d.Resolve(l); err == nil {
				ln = intOf(lv)
			}
		}
		start := p.pos
		if ln < 0 || start+ln > len(d.b) || !bytes.Contains(d.b[start+ln:min(len(d.b), start+ln+30)], []byte("endstream")) {
			e := bytes.Index(d.b[start:], []byte("endstream"))
			if e < 0 {
				return 0, nil, 0, errEOF
			}
			ln = e
			for ln > 0 && (d.b[start+ln-1] == '\n' || d.b[start+ln-1] == '\r') {
				ln--
			}
		}
		v = &Stream{D: dd, Data: d.b[start : start+ln]}
		p.pos = start + ln
	}
	e := bytes.Index(d.b[p.pos:], []byte("endobj"))
	end := len(d.b)
	if e >= 0 {
		end = p.pos + e + 6
	}
	return n, v, end, nil
}

func (d *PDF) Resolve(o any) (any, error) {
	r, ok := o.(Ref)
	if !ok {
		return o, nil
	}
	return d.Get(r.N)
}

func (d *PDF) Get(n int) (any, error) {
	if v, ok := d.Modified[n]; ok {
		return v, nil
	}
	if v, ok := d.cache[n]; ok {
		return v, nil
	}
	e, ok := d.xref[n]
	if !ok || e.typ == 0 {
		return Null{}, nil
	}
	if e.typ == 1 {
		d.cache[n] = Null{} // guard recursion
		_, v, end, err := d.parseIndirectAt(e.off)
		if err != nil {
			delete(d.cache, n)
			return nil, err
		}
		d.cache[n] = v
		d.rawSpan[n] = [2]int{e.off, end}
		return v, nil
	}
	objs, err := d.loadObjStm(e.stm)
	if err != nil {
		return nil, err
	}
	if e.idx >= len(objs) {
		return Null{}, nil
	}
	p := &parser{b: objs[e.idx]}
	v, err := p.parse()
	if err != nil {
		return nil, err
	}
	d.cache[n] = v
	return v, nil
}

func (d *PDF) loadObjStm(n int) ([][]byte, error) {
	if o, ok := d.objstm[n]; ok {
		return o, nil
	}
	v, err := d.Get(n)
	if err != nil {
		return nil, err
	}
	s, ok := v.(*Stream)
	if !ok {
		return nil, errors.New("bad object stream")
	}
	data, err := d.Decode(s)
	if err != nil {
		return nil, err
	}
	cnt, first := intOf(s.D.Get("N")), intOf(s.D.Get("First"))
	p := &parser{b: data}
	offs := make([]int, cnt)
	for i := 0; i < cnt; i++ {
		p.token()
		offs[i], _ = strconv.Atoi(p.token())
	}
	out := make([][]byte, cnt)
	for i := 0; i < cnt; i++ {
		s := first + offs[i]
		e := len(data)
		if i+1 < cnt {
			e = first + offs[i+1]
		}
		if s > len(data) || e > len(data) || s > e {
			continue
		}
		out[i] = data[s:e]
	}
	d.objstm[n] = out
	return out, nil
}

func (d *PDF) Decode(s *Stream) ([]byte, error) {
	data := s.Data
	filters := []any{}
	switch f := s.D.Get("Filter").(type) {
	case Name:
		filters = append(filters, f)
	case Arr:
		filters = f
	case Ref:
		r, _ := d.Resolve(f)
		if n, ok := r.(Name); ok {
			filters = append(filters, n)
		}
	}
	parms, _ := d.Resolve(s.D.Get("DecodeParms"))
	for i, f := range filters {
		if f != Name("FlateDecode") {
			return nil, fmt.Errorf("unsupported filter %v", f)
		}
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		out, err := io.ReadAll(zr)
		if err != nil && len(out) == 0 {
			return nil, err
		}
		data = out
		var pd *Dict
		switch pp := parms.(type) {
		case *Dict:
			pd = pp
		case Arr:
			if i < len(pp) {
				x, _ := d.Resolve(pp[i])
				pd, _ = x.(*Dict)
			}
		}
		if pd != nil && intOf(pd.Get("Predictor")) >= 10 {
			cols := intOf(pd.Get("Columns"))
			if cols == 0 {
				cols = 1
			}
			data = pngUnpredict(data, cols)
		}
	}
	return data, nil
}

func pngUnpredict(data []byte, cols int) []byte {
	row := cols + 1
	prev := make([]byte, cols)
	var out []byte
	for i := 0; i+row <= len(data); i += row {
		ft := data[i]
		cur := append([]byte{}, data[i+1:i+row]...)
		for j := 0; j < cols; j++ {
			var a, c byte
			if j > 0 {
				a = cur[j-1]
				c = prev[j-1]
			}
			b := prev[j]
			switch ft {
			case 1:
				cur[j] += a
			case 2:
				cur[j] += b
			case 3:
				cur[j] += byte((int(a) + int(b)) / 2)
			case 4:
				pa, pb, pc := abs(int(b)-int(c)), abs(int(a)-int(c)), abs(int(a)+int(b)-2*int(c))
				if pa <= pb && pa <= pc {
					cur[j] += a
				} else if pb <= pc {
					cur[j] += b
				} else {
					cur[j] += c
				}
			}
		}
		out = append(out, cur...)
		prev = cur
	}
	return out
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Pages returns page dicts with inherited Resources resolved.
type Page struct {
	D   *Dict
	Res *Dict
}

func (d *PDF) Pages() ([]Page, error) {
	root, err := d.Resolve(d.Trailer.Get("Root"))
	if err != nil {
		return nil, err
	}
	rd, ok := root.(*Dict)
	if !ok {
		return nil, errors.New("bad catalog")
	}
	var out []Page
	var walk func(o any, res *Dict, depth int) error
	walk = func(o any, res *Dict, depth int) error {
		if depth > 50 {
			return nil
		}
		v, err := d.Resolve(o)
		if err != nil {
			return err
		}
		nd, ok := v.(*Dict)
		if !ok {
			return nil
		}
		if r, _ := d.Resolve(nd.Get("Resources")); r != nil {
			if rdd, ok := r.(*Dict); ok {
				res = rdd
			}
		}
		if nd.Get("Type") == Name("Page") {
			out = append(out, Page{D: nd, Res: res})
			return nil
		}
		kids, _ := d.Resolve(nd.Get("Kids"))
		if ka, ok := kids.(Arr); ok {
			for _, k := range ka {
				if err := walk(k, res, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err = walk(rd.Get("Pages"), nil, 0)
	return out, err
}

// ---------- writer ----------

func writeObj(w *bytes.Buffer, o any) {
	switch v := o.(type) {
	case Name:
		w.WriteByte('/')
		w.WriteString(string(v))
	case Num:
		w.WriteString(string(v))
	case Bool:
		if v {
			w.WriteString("true")
		} else {
			w.WriteString("false")
		}
	case Null:
		w.WriteString("null")
	case Str:
		w.Write(v.Raw)
	case Ref:
		fmt.Fprintf(w, "%d %d R", v.N, v.G)
	case Arr:
		w.WriteByte('[')
		for i, x := range v {
			if i > 0 {
				w.WriteByte(' ')
			}
			writeObj(w, x)
		}
		w.WriteByte(']')
	case *Dict:
		w.WriteString("<<")
		for _, k := range v.Keys {
			w.WriteByte('/')
			w.WriteString(k)
			w.WriteByte(' ')
			writeObj(w, v.M[k])
		}
		w.WriteString(">>")
	case *Stream:
		writeObj(w, v.D)
		w.WriteString("\nstream\n")
		w.Write(v.Data)
		w.WriteString("\nendstream")
	}
}

var reObjHead = regexp.MustCompile(`^\s*\d+\s+\d+\s+obj`)

func (d *PDF) Save() ([]byte, error) {
	var nums []int
	maxN := 0
	for n := range d.xref {
		nums = append(nums, n)
		if n > maxN {
			maxN = n
		}
	}
	for n := range d.Modified {
		if _, ok := d.xref[n]; !ok {
			nums = append(nums, n)
		}
		if n > maxN {
			maxN = n
		}
	}
	sort.Ints(nums)
	var w bytes.Buffer
	w.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offs := map[int]int{}
	for _, n := range nums {
		if n == 0 {
			continue
		}
		e := d.xref[n]
		_, mod := d.Modified[n]
		if !mod && e.typ == 0 {
			continue
		}
		v, err := d.Get(n)
		if err != nil {
			return nil, fmt.Errorf("object %d: %v", n, err)
		}
		if s, ok := v.(*Stream); ok {
			t := s.D.Get("Type")
			if t == Name("ObjStm") || t == Name("XRef") {
				continue
			}
		}
		offs[n] = w.Len()
		if !mod && e.typ == 1 {
			sp := d.rawSpan[n]
			raw := d.b[sp[0]:sp[1]]
			// normalise header to "n 0 obj"
			if loc := reObjHead.FindIndex(raw); loc != nil {
				fmt.Fprintf(&w, "%d 0 obj", n)
				w.Write(raw[loc[1]:])
			} else {
				w.Write(raw)
			}
			w.WriteByte('\n')
			continue
		}
		fmt.Fprintf(&w, "%d 0 obj\n", n)
		writeObj(&w, v)
		w.WriteString("\nendobj\n")
	}
	xo := w.Len()
	fmt.Fprintf(&w, "xref\n0 %d\n", maxN+1)
	w.WriteString("0000000000 65535 f\r\n")
	for n := 1; n <= maxN; n++ {
		if o, ok := offs[n]; ok {
			fmt.Fprintf(&w, "%010d 00000 n\r\n", o)
		} else {
			w.WriteString("0000000000 00001 f\r\n")
		}
	}
	tr := NewDict()
	tr.Set("Size", Num(strconv.Itoa(maxN+1)))
	for _, k := range []string{"Root", "Info", "ID"} {
		if v := d.Trailer.Get(k); v != nil {
			tr.Set(k, v)
		}
	}
	w.WriteString("trailer\n")
	writeObj(&w, tr)
	fmt.Fprintf(&w, "\nstartxref\n%d\n%%%%EOF\n", xo)
	return w.Bytes(), nil
}

func flate(b []byte) []byte {
	var buf bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	zw.Write(b)
	zw.Close()
	return buf.Bytes()
}
