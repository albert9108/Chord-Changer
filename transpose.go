package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// House spelling: only Bb is written as a flat. Db->C#, Eb->D#, Ab->G#.
var NAMES = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "Bb", "B"}
var IDX = map[string]int{"C": 0, "B#": 0, "C#": 1, "Db": 1, "D": 2, "D#": 3, "Eb": 3, "E": 4, "Fb": 4, "F": 5, "E#": 5,
	"F#": 6, "Gb": 6, "G": 7, "G#": 8, "Ab": 8, "A": 9, "A#": 10, "Bb": 10, "B": 11, "Cb": 11}

var chordRun = regexp.MustCompile(`^[\s|:()/A-G#b0-9majdimsu+\-]*$`)
var hasNote = regexp.MustCompile(`[A-G]`)

// isChordText reports whether a text run contains only chord symbols.
func isChordText(s string) bool { return chordRun.MatchString(s) && hasNote.MatchString(s) }

// noteAt returns the note token starting at i (e.g. "C", "F#", "Bb") or "".
func noteAt(s string, i int) string {
	c := s[i]
	if c < 'A' || c > 'G' {
		return ""
	}
	if i+1 < len(s) {
		n := s[i+1]
		if n == '#' {
			return s[i : i+2]
		}
		if n == 'b' && !(i+2 < len(s) && s[i+2] >= 'a' && s[i+2] <= 'z') {
			return s[i : i+2]
		}
	}
	return s[i : i+1]
}

func transposeText(s string, n int) string {
	if !isChordText(s) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if t := noteAt(s, i); t != "" {
			b.WriteString(NAMES[((IDX[t]+n)%12+12)%12])
			i += len(t)
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// ---------- key detection ----------

var reKey = regexp.MustCompile(`[Kk][Ee][Yy]\s*[:：]\s*([A-G](?:#|b)?m?)`)
var reKeyName = regexp.MustCompile(`^\s*([A-G](?:#|b)?m?)\s*[Kk]ey`)

type KeyInfo struct {
	Key    string // e.g. "G", "Em"
	Source string
}

func keyFromLines(lines []string, fname string) (KeyInfo, error) {
	for _, l := range lines {
		if m := reKey.FindStringSubmatch(l); m != nil {
			return KeyInfo{normKey(m[1]), "from the \"Key:\" line"}, nil
		}
	}
	if m := reKeyName.FindStringSubmatch(filepath.Base(fname)); m != nil {
		return KeyInfo{normKey(m[1]), "from the file name"}, nil
	}
	for _, l := range lines {
		for _, f := range strings.Fields(strings.NewReplacer("|", " ", ":", " ", "(", " ", ")", " ").Replace(l)) {
			if isChordText(f) {
				if t := noteAt(f, 0); t != "" {
					k := t
					if strings.HasPrefix(f[len(t):], "m") && !strings.HasPrefix(f[len(t):], "maj") {
						k += "m"
					}
					return KeyInfo{normKey(k), "guessed from the first chord — please check"}, nil
				}
			}
		}
	}
	return KeyInfo{}, errors.New("could not find the key in this file")
}

func normKey(k string) string {
	minor := strings.HasSuffix(k, "m")
	r := strings.TrimSuffix(k, "m")
	if i, ok := IDX[r]; ok {
		r = NAMES[i]
	}
	if minor {
		return r + "m"
	}
	return r
}

func KeyRoot(k string) int { return IDX[strings.TrimSuffix(k, "m")] }

// ---------- docx ----------

var reWT = regexp.MustCompile(`(<w:t(?:\s[^>]*)?>)([^<]*)(</w:t>)`)
var rePara = regexp.MustCompile(`(?s)<w:p[ >].*?</w:p>`)
var reDocxPart = regexp.MustCompile(`^word/(document|header\d*|footer\d*)\.xml$`)

func docxParts(src []byte) (*zip.Reader, error) {
	return zip.NewReader(bytes.NewReader(src), int64(len(src)))
}

func DetectDocx(src []byte, fname string) (KeyInfo, error) {
	zr, err := docxParts(src)
	if err != nil {
		return KeyInfo{}, errors.New("not a valid .docx file")
	}
	var lines []string
	order := []string{}
	for _, f := range zr.File {
		if reDocxPart.MatchString(f.Name) {
			order = append(order, f.Name)
		}
	}
	sort.Slice(order, func(i, j int) bool { // headers first
		return strings.Contains(order[i], "header") && !strings.Contains(order[j], "header")
	})
	for _, name := range order {
		for _, f := range zr.File {
			if f.Name != name {
				continue
			}
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			for _, p := range rePara.FindAll(b, -1) {
				var sb strings.Builder
				for _, m := range reWT.FindAllSubmatch(p, -1) {
					sb.WriteString(html.UnescapeString(string(m[2])))
				}
				lines = append(lines, sb.String())
			}
		}
	}
	return keyFromLines(lines, fname)
}

func ConvertDocx(src []byte, n int) ([]byte, int, error) {
	zr, err := docxParts(src)
	if err != nil {
		return nil, 0, errors.New("not a valid .docx file")
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	count := 0
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, 0, err
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		if reDocxPart.MatchString(f.Name) {
			b = reWT.ReplaceAllFunc(b, func(m []byte) []byte {
				sm := reWT.FindSubmatch(m)
				t := string(sm[2])
				nt := transposeText(t, n)
				if nt == t {
					return m
				}
				count++
				return []byte(string(sm[1]) + nt + string(sm[3]))
			})
		}
		h := f.FileHeader
		w, err := zw.CreateHeader(&h)
		if err != nil {
			return nil, 0, err
		}
		w.Write(b)
	}
	zw.Close()
	return out.Bytes(), count, nil
}

// ---------- pdf ----------

// Arial widths (/1000) for glyphs a transposition can introduce.
var awBold = map[byte]int{' ': 278, '#': 556, 'b': 611, 'A': 722, 'B': 722, 'C': 722, 'D': 722, 'E': 667, 'F': 611, 'G': 778}
var awReg = map[byte]int{' ': 278, '#': 556, 'b': 556, 'A': 667, 'B': 667, 'C': 722, 'D': 722, 'E': 667, 'F': 611, 'G': 778}

var reBlk = regexp.MustCompile(`(?s)BT\s*/([^\s/\[\]()<>]+)\s+(-?[\d.]+)\s+Tf\s*1 0 0 1 (-?[\d.]+) (-?[\d.]+) Tm(.*?)\sET\b`)
var reTJ = regexp.MustCompile(`(?s)\[(.*?)\]\s*TJ`)
var reTm = regexp.MustCompile(`1 0 0 1 -?[\d.]+ `)

type fontInfo struct {
	ref    any
	d      *Dict
	simple bool
	subset bool
	first  int
	widths []int
	bold   bool
}

type run struct {
	st, en int // span of whole match in content
	font   string
	sz     float64
	x, y   float64
	s, nw  string
	nx     float64
	line   int
}

func unescapePDF(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				if s[i] >= '0' && s[i] <= '7' {
					j := i
					for j < len(s) && j < i+3 && s[j] >= '0' && s[j] <= '7' {
						j++
					}
					v, _ := strconv.ParseInt(s[i:j], 8, 32)
					b.WriteByte(byte(v))
					i = j - 1
				} else {
					b.WriteByte(s[i])
				}
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func tjText(body string) (string, bool) {
	m := reTJ.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	inner := m[1]
	var sb strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] == '(' {
			depth, j := 1, i+1
			for ; j < len(inner) && depth > 0; j++ {
				if inner[j] == '\\' {
					j++
					continue
				}
				if inner[j] == '(' {
					depth++
				} else if inner[j] == ')' {
					depth--
				}
			}
			sb.WriteString(unescapePDF(inner[i+1 : j-1]))
			i = j - 1
		} else if inner[i] == '<' {
			return "", false // hex strings: not a plain chord run
		}
	}
	return sb.String(), true
}

func escapePDF(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}

type pageWork struct {
	content []byte
	streams []Ref // content stream refs
	fonts   map[string]*fontInfo
	runs    []*run
}

func (d *PDF) loadPage(pg Page) (*pageWork, error) {
	pw := &pageWork{fonts: map[string]*fontInfo{}}
	var parts []any
	switch c := pg.D.Get("Contents").(type) {
	case Ref:
		v, _ := d.Resolve(c)
		if a, ok := v.(Arr); ok {
			parts = a
		} else {
			parts = []any{c}
		}
	case Arr:
		parts = c
	}
	for _, p := range parts {
		r, ok := p.(Ref)
		if !ok {
			continue
		}
		v, err := d.Resolve(r)
		if err != nil {
			return nil, err
		}
		s, ok := v.(*Stream)
		if !ok {
			continue
		}
		data, err := d.Decode(s)
		if err != nil {
			return nil, err
		}
		pw.streams = append(pw.streams, r)
		pw.content = append(pw.content, data...)
		pw.content = append(pw.content, '\n')
	}
	if pg.Res != nil {
		fv, _ := d.Resolve(pg.Res.Get("Font"))
		if fd, ok := fv.(*Dict); ok {
			for _, k := range fd.Keys {
				ref := fd.M[k]
				v, _ := d.Resolve(ref)
				f, ok := v.(*Dict)
				if !ok {
					continue
				}
				st := f.Get("Subtype")
				bf, _ := f.Get("BaseFont").(Name)
				fi := &fontInfo{ref: ref, d: f, simple: st == Name("TrueType") || st == Name("Type1") || st == Name("MMType1"),
					bold: strings.Contains(string(bf), "Bold"), subset: len(bf) > 7 && bf[6] == '+'}
				fi.first = intOf(f.Get("FirstChar"))
				wv, _ := d.Resolve(f.Get("Widths"))
				if wa, ok := wv.(Arr); ok {
					for _, x := range wa {
						xv, _ := d.Resolve(x)
						fi.widths = append(fi.widths, int(math.Round(floatOf(xv))))
					}
				}
				pw.fonts[k] = fi
			}
		}
	}
	c := string(pw.content)
	for _, m := range reBlk.FindAllStringSubmatchIndex(c, -1) {
		font := c[m[2]:m[3]]
		fi := pw.fonts[font]
		if fi == nil || !fi.simple {
			continue
		}
		s, ok := tjText(c[m[10]:m[11]])
		if !ok {
			continue
		}
		sz, _ := strconv.ParseFloat(c[m[4]:m[5]], 64)
		x, _ := strconv.ParseFloat(c[m[6]:m[7]], 64)
		y, _ := strconv.ParseFloat(c[m[8]:m[9]], 64)
		pw.runs = append(pw.runs, &run{st: m[0], en: m[1], font: font, sz: sz, x: x, y: y, s: s, nx: x})
	}
	// group runs into lines (superscripts sit a few points above the baseline)
	idx := make([]*run, len(pw.runs))
	copy(idx, pw.runs)
	sort.Slice(idx, func(i, j int) bool { return idx[i].y > idx[j].y })
	line, prev := -1, math.Inf(1)
	for _, r := range idx {
		if prev-r.y > 8 || line < 0 {
			line++
		}
		prev = r.y
		r.line = line
	}
	return pw, nil
}

func (fi *fontInfo) width(ch byte) int {
	i := int(ch) - fi.first
	if i >= 0 && i < len(fi.widths) && fi.widths[i] > 0 {
		return fi.widths[i]
	}
	t := awReg
	if fi.bold {
		t = awBold
	}
	if w, ok := t[ch]; ok {
		return w
	}
	return 556
}

func (pw *pageWork) wd(r *run, s string) float64 {
	fi := pw.fonts[r.font]
	t := 0
	for i := 0; i < len(s); i++ {
		t += fi.width(s[i])
	}
	return float64(t) * r.sz / 1000
}

func linesOf(pw *pageWork) []string {
	by := map[int][]*run{}
	maxL := -1
	for _, r := range pw.runs {
		by[r.line] = append(by[r.line], r)
		if r.line > maxL {
			maxL = r.line
		}
	}
	var out []string
	for l := 0; l <= maxL; l++ {
		rs := by[l]
		sort.Slice(rs, func(i, j int) bool { return rs[i].x < rs[j].x })
		var sb strings.Builder
		for _, r := range rs {
			sb.WriteString(r.s)
		}
		out = append(out, sb.String())
	}
	return out
}

func DetectPDF(src []byte, fname string) (KeyInfo, error) {
	d, err := OpenPDF(src)
	if err != nil {
		return KeyInfo{}, err
	}
	pages, err := d.Pages()
	if err != nil || len(pages) == 0 {
		return KeyInfo{}, errors.New("could not read pages in this PDF")
	}
	var lines []string
	for _, pg := range pages {
		pw, err := d.loadPage(pg)
		if err != nil {
			return KeyInfo{}, err
		}
		lines = append(lines, linesOf(pw)...)
	}
	return keyFromLines(lines, fname)
}

func ConvertPDF(src []byte, n int) ([]byte, int, []string, error) {
	d, err := OpenPDF(src)
	if err != nil {
		return nil, 0, nil, err
	}
	pages, err := d.Pages()
	if err != nil {
		return nil, 0, nil, err
	}
	count := 0
	warn := map[string]bool{}
	for _, pg := range pages {
		pw, err := d.loadPage(pg)
		if err != nil {
			return nil, 0, nil, err
		}
		for _, r := range pw.runs {
			r.nw = transposeText(r.s, n)
		}
		// push following text right when a chord got wider
		by := map[int][]*run{}
		for _, r := range pw.runs {
			by[r.line] = append(by[r.line], r)
		}
		for _, rs := range by {
			sort.Slice(rs, func(i, j int) bool { return rs[i].x < rs[j].x })
			shift := 0.0
			for i, r := range rs {
				r.nx = r.x + shift
				if i+1 >= len(rs) {
					continue
				}
				nxt := rs[i+1]
				oend := r.x + pw.wd(r, r.s)
				nend := r.nx + pw.wd(r, r.nw)
				if math.Abs(nxt.x-oend) < 1.5 {
					shift = nend - oend
				} else {
					gap := math.Max(nxt.x-oend, 0)
					shift = math.Max(0, nend+math.Min(gap, 5.5)-nxt.x)
				}
			}
		}
		c := string(pw.content)
		var sb strings.Builder
		pos := 0
		used := map[string]map[byte]bool{}
		changed := false
		for _, r := range pw.runs {
			if r.nw == r.s && math.Abs(r.nx-r.x) < 1e-6 {
				continue
			}
			orig := c[r.st:r.en]
			loc := reTm.FindStringIndex(orig)
			t := orig[:loc[0]] + fmt.Sprintf("1 0 0 1 %.2f ", r.nx) + orig[loc[1]:]
			tl := reTJ.FindStringIndex(t)
			t = t[:tl[0]] + "[(" + escapePDF(r.nw) + ")] TJ" + t[tl[1]:]
			sb.WriteString(c[pos:r.st])
			sb.WriteString(t)
			pos = r.en
			changed = true
			if r.nw != r.s {
				count++
				if used[r.font] == nil {
					used[r.font] = map[byte]bool{}
				}
				for i := 0; i < len(r.nw); i++ {
					used[r.font][r.nw[i]] = true
				}
			}
		}
		if !changed {
			continue
		}
		sb.WriteString(c[pos:])
		// write new content into the first stream, empty the others
		for i, ref := range pw.streams {
			v, _ := d.Resolve(ref)
			s := v.(*Stream)
			nd := s.D.Clone()
			nd.Del("DecodeParms")
			nd.Set("Filter", Name("FlateDecode"))
			var data []byte
			if i == 0 {
				data = flate([]byte(sb.String()))
			} else {
				data = flate(nil)
			}
			nd.Set("Length", Num(strconv.Itoa(len(data))))
			d.Modified[ref.N] = &Stream{D: nd, Data: data}
		}
		// fill zero widths for new glyphs (e.g. 'b', '#')
		for fname, chars := range used {
			fi := pw.fonts[fname]
			if len(fi.widths) == 0 {
				continue
			}
			ws := append([]int{}, fi.widths...)
			ch := false
			for c := range chars {
				i := int(c) - fi.first
				if i >= 0 && i < len(ws) && ws[i] == 0 {
					if fi.subset {
						warn[fmt.Sprintf("The chord font is embedded without the \"%c\" symbol, so it may not show correctly. If so, convert the Word (.docx) version instead.", c)] = true
					}
					ws[i] = fi.width(c)
					ch = true
				}
			}
			if !ch {
				continue
			}
			fi.widths = ws
			arr := Arr{}
			for _, w := range ws {
				arr = append(arr, Num(strconv.Itoa(w)))
			}
			if wr, ok := fi.d.Get("Widths").(Ref); ok {
				d.Modified[wr.N] = arr
			} else if fr, ok := fi.ref.(Ref); ok {
				nd := fi.d.Clone()
				nd.Set("Widths", arr)
				d.Modified[fr.N] = nd
				fi.d = nd
			}
		}
	}
	out, err := d.Save()
	if err != nil {
		return nil, 0, nil, err
	}
	var ws []string
	for w := range warn {
		ws = append(ws, w)
	}
	sort.Strings(ws)
	return out, count, ws, nil
}

// ---------- front door ----------

func Detect(path string, data []byte) (KeyInfo, error) {
	if strings.EqualFold(filepath.Ext(path), ".docx") {
		return DetectDocx(data, path)
	}
	return DetectPDF(data, path)
}

func Convert(path string, data []byte, n int) ([]byte, int, []string, error) {
	if strings.EqualFold(filepath.Ext(path), ".docx") {
		b, c, err := ConvertDocx(data, n)
		return b, c, nil, err
	}
	return ConvertPDF(data, n)
}

// OutputName swaps the key at the start of the file name, or adds it.
func OutputName(src, newKey string) string {
	base := filepath.Base(src)
	if loc := regexp.MustCompile(`^\s*[A-G](?:#|b)?m?\s*[Kk]ey`).FindStringIndex(base); loc != nil {
		return newKey + " Key" + base[loc[1]:]
	}
	return newKey + " Key - " + base
}
