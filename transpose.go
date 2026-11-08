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
	// The whole song is consulted either way: as the answer when nothing
	// states the key, and as a cross-check when something does.
	guess, margin, haveGuess := keyFromChords(lines)

	// Only contradict a stated key when the chords say so clearly. A relative
	// major/minor pair shares every diatonic chord, so a thin margin there is
	// not evidence of anything.
	note := func(src, k string) string {
		if haveGuess && guess.Key != k && margin >= 10 {
			return src + " (the chords look more like " + guess.Key + ")"
		}
		return src
	}

	for _, l := range lines {
		if m := reKey.FindStringSubmatch(l); m != nil {
			k := normKey(m[1])
			return KeyInfo{k, note(`the "Key:" line`, k)}, nil
		}
	}
	if m := reKeyName.FindStringSubmatch(filepath.Base(fname)); m != nil {
		k := normKey(m[1])
		return KeyInfo{k, note("the file name", k)}, nil
	}
	if haveGuess {
		return guess, nil
	}
	return KeyInfo{}, errors.New("could not find the key in this file")
}

// ---------- key from the song's chords ----------

// chord quality, as far as key-fitting cares
const (
	qMaj = 0
	qMin = 1
	qDim = 2
)

type chordTok struct{ root, qual int }

// Scale degrees and the triad quality built on each, as semitone offsets
// from the tonic.
var (
	degMaj  = [7]int{0, 2, 4, 5, 7, 9, 11}
	qualMaj = [7]int{qMaj, qMin, qMin, qMaj, qMaj, qMin, qDim}
	degMin  = [7]int{0, 2, 3, 5, 7, 8, 10}
	qualMin = [7]int{qMin, qDim, qMaj, qMin, qMin, qMaj, qMaj}
)

// parseChord reads a chord symbol, ignoring any slash bass.
func parseChord(s string) (chordTok, bool) {
	t := noteAt(s, 0)
	if t == "" {
		return chordTok{}, false
	}
	r, ok := IDX[t]
	if !ok {
		return chordTok{}, false
	}
	rest := s[len(t):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	q := qMaj
	switch {
	case strings.HasPrefix(rest, "dim"):
		q = qDim
	case strings.HasPrefix(rest, "m") && !strings.HasPrefix(rest, "maj"):
		q = qMin
	}
	return chordTok{r, q}, true
}

var chordSep = strings.NewReplacer("|", " ", ":", " ", "(", " ", ")", " ")

// chordsIn collects the song's chords in reading order. Lines made up
// entirely of chords are preferred, since that is how these charts are laid
// out and it keeps lyrics like "Dad" out of the tally; if too few turn up,
// every chord-shaped token is taken instead.
func chordsIn(lines []string) []chordTok {
	var strict, loose []chordTok
	for _, l := range lines {
		fields := strings.Fields(chordSep.Replace(l))
		if len(fields) == 0 {
			continue
		}
		var line []chordTok
		for _, f := range fields {
			if !isChordText(f) {
				continue
			}
			if c, ok := parseChord(f); ok {
				line = append(line, c)
			}
		}
		loose = append(loose, line...)
		// A chord line is mostly chords. Section labels ("Intro:") and
		// performance marks ("(4n)") sit among them without making the line
		// anything else, so a couple of strays are tolerated; a lyric line
		// never clears the bar.
		// A lone chord on its own line is usually the song's last one, which
		// is the most telling of all, so the bar is the ratio, not a count.
		if len(line) > 0 && len(line)*10 >= len(fields)*6 {
			strict = append(strict, line...)
		}
	}
	if len(strict) >= 3 {
		return strict
	}
	return loose
}

// scoreKey rates how well the chords fit one key. Diatonic chords score,
// chromatic ones cost, and the opening and closing chords count for more
// because songs tend to begin and end on the tonic.
func scoreKey(toks []chordTok, root int, minor bool) int {
	deg, qual := degMaj, qualMaj
	tonicQual := qMaj
	if minor {
		deg, qual = degMin, qualMin
		tonicQual = qMin
	}
	total := 0
	for i, c := range toks {
		w := 1
		switch i {
		case 0:
			w = 2
		case len(toks) - 1:
			w = 4
		}
		rel := ((c.root-root)%12 + 12) % 12
		hit := -1
		for d := 0; d < 7; d++ {
			if deg[d] == rel {
				hit = d
				break
			}
		}
		switch {
		case hit < 0:
			total -= 2 * w
		case qual[hit] == c.qual:
			total += 3 * w
		case minor && rel == 7 && c.qual == qMaj:
			total += 3 * w // harmonic-minor V, very common
		default:
			total += w // right root, unexpected quality
		}
	}
	last, first := toks[len(toks)-1], toks[0]
	if last.root == root && last.qual == tonicQual {
		total += 8
	}
	if first.root == root && first.qual == tonicQual {
		total += 4
	}
	// Charts are often laid out in columns, so reading order - and with it
	// the "last chord" - is not always the order the song is played in.
	// How much the tonic is used does not depend on that order.
	for _, c := range toks {
		if c.root == root && c.qual == tonicQual {
			total += 2
		}
	}
	return total
}

// keyFromChords picks the key the song's chords fit best. A relative
// major/minor pair shares every diatonic chord, so the tonic bonuses in
// scoreKey are usually what separates them; when they do not, the result
// says to check it.
func keyFromChords(lines []string) (KeyInfo, int, bool) {
	toks := chordsIn(lines)
	if len(toks) < 3 {
		return KeyInfo{}, 0, false
	}
	best, second := math.MinInt, math.MinInt
	bestRoot, bestMinor := 0, false
	for r := 0; r < 12; r++ {
		for _, m := range []bool{false, true} {
			switch sv := scoreKey(toks, r, m); {
			case sv > best:
				second = best
				best, bestRoot, bestMinor = sv, r, m
			case sv > second:
				second = sv
			}
		}
	}
	k := NAMES[bestRoot]
	if bestMinor {
		k += "m"
	}
	src := "the chords in the song"
	if best-second < 6 {
		src += " — please check"
	}
	return KeyInfo{k, src}, best - second, true
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

// Helvetica / Arial advance widths (/1000) for printable ASCII, from the
// standard Helvetica AFM metrics, which Arial shares. They size glyphs a
// transposition introduces into a subset that lacks them, and whole runs
// re-set in the standard fallback font - where a run like "||: F#m" needs
// the bar and colon measured too, not just the note letters.
var awBold = asciiWidths(
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278, // space ! " # $ % & ' ( ) * + , - . /
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, // 0-9
	333, 333, 584, 584, 584, 611, 975, // : ; < = > ? @
	722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, // A-M
	722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, // N-Z
	333, 278, 333, 584, 556, 333, // [ \ ] ^ _ `
	556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, // a-m
	611, 611, 611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, // n-z
	389, 280, 389, 584, // { | } ~
)
var awReg = asciiWidths(
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556,
	278, 278, 584, 584, 584, 556, 1015,
	667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833,
	722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611,
	278, 278, 278, 469, 556, 333,
	556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833,
	556, 556, 556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500,
	334, 260, 334, 584,
)

func asciiWidths(w ...int) map[byte]int {
	if len(w) != 95 {
		panic("asciiWidths: need 95 widths for ' '..'~'")
	}
	m := make(map[byte]int, len(w))
	for i, v := range w {
		m[byte(' '+i)] = v
	}
	return m
}

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
	uni    map[byte]string // /ToUnicode: character code -> real text
}

type run struct {
	st, en int // span of whole match in content
	font   string
	sz     float64
	x, y   float64
	s, nw  string
	nx     float64
	line   int

	// Set only for Quartz-shaped blocks, which are written back by splicing
	// spans rather than by rebuilding a fixed "1 0 0 1 x y Tm ... TJ" shape.
	enc      map[rune]byte // real character -> this subset's code
	xSt, xEn int           // span of the Tm x operand, absolute in content
	fSt, fEn int           // span of the Tf font name, absolute in content
	lits     []strLit      // spans of the block's string operands, absolute
	fb       bool          // write in the standard fallback font instead
	fbBold   bool
	blank    bool // emptied: its text moved into the first run of its word

	// Page x = ca*x + ce. Quartz places some runs by the Tm and others by the
	// cm around them, so text-space x values from different runs are not
	// comparable; only page x is. Zero ca means an ordinary run (identity).
	ca, ce float64
}

func (r *run) scale() float64 {
	if r.ca == 0 {
		return 1
	}
	return r.ca
}

// px is the run's position on the page, in points.
func (r *run) px() float64 { return r.scale()*r.x + r.ce }

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
				fi.uni = toUnicode(d, f)
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
	if len(pw.runs) == 0 {
		pw.quartzRuns()
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
	if r.fb {
		// the standard fallback font: Helvetica shares Arial's metrics
		tbl := awReg
		if fi != nil && fi.bold {
			tbl = awBold
		}
		t := 0
		for i := 0; i < len(s); i++ {
			if w, ok := tbl[s[i]]; ok {
				t += w
			} else {
				t += 556
			}
		}
		return float64(t) * r.sz / 1000
	}
	if r.enc != nil {
		// a subset's codes, not the text, is what the width table is keyed by
		if b, miss := encodeRun(s, r.enc); len(miss) == 0 {
			s = b
		}
	}
	t := 0
	for i := 0; i < len(s); i++ {
		t += fi.width(s[i])
	}
	return float64(t) * r.sz / 1000
}

// ---------- /ToUnicode ----------
//
// Quartz (macOS) re-encodes each embedded subset so its character codes run
// from 33 upwards in the order the glyphs were first used, which has nothing
// to do with ASCII. The real text is only recoverable through the font's
// /ToUnicode CMap.

var reBfChar = regexp.MustCompile(`(?s)beginbfchar(.*?)endbfchar`)
var reBfRange = regexp.MustCompile(`(?s)beginbfrange(.*?)endbfrange`)
var reCMapHex = regexp.MustCompile(`<([0-9A-Fa-f]+)>`)

// utf16BEString decodes the UTF-16BE hex a CMap uses for its destinations.
func utf16BEString(h string) string {
	var sb strings.Builder
	for i := 0; i+4 <= len(h); i += 4 {
		v, err := strconv.ParseUint(h[i:i+4], 16, 32)
		if err != nil {
			break
		}
		sb.WriteRune(rune(v))
	}
	return sb.String()
}

// toUnicode reads a simple font's /ToUnicode CMap. Only single-byte codes are
// kept, which is what a simple font uses.
func toUnicode(d *PDF, f *Dict) map[byte]string {
	v, err := d.Resolve(f.Get("ToUnicode"))
	if err != nil {
		return nil
	}
	st, ok := v.(*Stream)
	if !ok {
		return nil
	}
	b, err := d.Decode(st)
	if err != nil {
		return nil
	}
	cm := string(b)
	m := map[byte]string{}
	for _, blk := range reBfChar.FindAllStringSubmatch(cm, -1) {
		h := reCMapHex.FindAllStringSubmatch(blk[1], -1)
		for i := 0; i+1 < len(h); i += 2 {
			c, err := strconv.ParseUint(h[i][1], 16, 32)
			if err != nil || c > 0xff {
				continue
			}
			m[byte(c)] = utf16BEString(h[i+1][1])
		}
	}
	for _, blk := range reBfRange.FindAllStringSubmatch(cm, -1) {
		h := reCMapHex.FindAllStringSubmatch(blk[1], -1)
		for i := 0; i+2 < len(h); i += 3 {
			lo, e1 := strconv.ParseUint(h[i][1], 16, 32)
			hi, e2 := strconv.ParseUint(h[i+1][1], 16, 32)
			dst := h[i+2][1]
			if e1 != nil || e2 != nil || lo > 0xff || hi > 0xff || hi < lo {
				continue
			}
			base := []rune(utf16BEString(dst))
			if len(base) == 0 {
				continue
			}
			for c := lo; c <= hi; c++ {
				r := append([]rune(nil), base...)
				r[len(r)-1] += rune(c - lo)
				m[byte(c)] = string(r)
			}
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// ---------- general text extraction (detection only) ----------
//
// reBlk only matches the one shape Word emits. Quartz writes the text matrix
// before the font, folds the point size into that matrix and wraps each run
// in its own cm, so nothing matches and no runs come out. This reads both
// shapes, decodes through /ToUnicode, and is used only to read a chart - the
// rewriting path still works from pw.runs and is untouched.

var reBT = regexp.MustCompile(`(?s)BT\b(.*?)\bET\b`)
var reNum6 = `(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s+`
var reTmAny = regexp.MustCompile(reNum6 + `Tm`)
var reTfAny = regexp.MustCompile(`/([^\s/\[\]()<>]+)\s+(-?[\d.]+)\s+Tf`)

type placedText struct {
	x, y float64
	s    string
}

// strLit is one string operand of a content stream and where it sits.
type strLit struct {
	st, en int
	raw    string
}

// blockStrings finds every string operand in a BT..ET body, keeping the span
// so it can be written back in place.
func blockStrings(body string, base int) []strLit {
	var out []strLit
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth, j := 1, i+1
			for ; j < len(body) && depth > 0; j++ {
				if body[j] == '\\' {
					j++
					continue
				}
				if body[j] == '(' {
					depth++
				} else if body[j] == ')' {
					depth--
				}
			}
			if j-1 > i {
				out = append(out, strLit{base + i, base + j, unescapePDF(body[i+1 : j-1])})
			}
			i = j - 1
		case '<':
			if i+1 < len(body) && body[i+1] == '<' { // a dictionary, not a string
				return out
			}
			j := strings.IndexByte(body[i:], '>')
			if j < 0 {
				return out
			}
			h := body[i+1 : i+j]
			var sb strings.Builder
			for k := 0; k+1 < len(h); k += 2 {
				v, err := strconv.ParseUint(h[k:k+2], 16, 16)
				if err != nil {
					break
				}
				sb.WriteByte(byte(v))
			}
			out = append(out, strLit{base + i, base + i + j + 1, sb.String()})
			i += j
		}
	}
	return out
}

// mat is a PDF transformation matrix [a b c d e f].
type mat struct{ a, b, c, d, e, f float64 }

var identity = mat{1, 0, 0, 1, 0, 0}

// then returns m followed by n, the way "cm" composes with the CTM.
func (m mat) then(n mat) mat {
	return mat{
		m.a*n.a + m.b*n.c, m.a*n.b + m.b*n.d,
		m.c*n.a + m.d*n.c, m.c*n.b + m.d*n.d,
		m.e*n.a + m.f*n.c + n.e, m.e*n.b + m.f*n.d + n.f,
	}
}

func isPDFSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}

func isPDFDelim(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

// textCTMs walks a content stream tracking q/Q/cm and returns the transform
// in force at each BT, keyed by the BT's offset. Quartz nests its states and
// places some runs entirely through the cm, so guessing from the nearest cm
// is not enough. Strings are skipped so their bytes are never read as
// operators.
func textCTMs(c string) map[int]mat {
	out := map[int]mat{}
	ctm := identity
	var stack []mat
	var nums []float64
	for i := 0; i < len(c); {
		ch := c[i]
		switch {
		case ch == '(':
			depth := 1
			for i++; i < len(c) && depth > 0; i++ {
				switch c[i] {
				case '\\':
					i++
				case '(':
					depth++
				case ')':
					depth--
				}
			}
			nums = nums[:0]
			continue
		case ch == '<' && i+1 < len(c) && c[i+1] == '<', ch == '>' && i+1 < len(c) && c[i+1] == '>':
			i += 2
			continue
		case ch == '<':
			k := strings.IndexByte(c[i:], '>')
			if k < 0 {
				return out
			}
			i += k + 1
			nums = nums[:0]
			continue
		case ch == '%':
			for i < len(c) && c[i] != '\n' && c[i] != '\r' {
				i++
			}
			continue
		case ch == '/':
			i++
			for i < len(c) && !isPDFSpace(c[i]) && !isPDFDelim(c[i]) {
				i++
			}
			nums = nums[:0]
			continue
		case isPDFSpace(ch) || isPDFDelim(ch):
			i++
			continue
		}
		st := i
		for i < len(c) && !isPDFSpace(c[i]) && !isPDFDelim(c[i]) {
			i++
		}
		tok := c[st:i]
		if v, err := strconv.ParseFloat(tok, 64); err == nil {
			nums = append(nums, v)
			continue
		}
		switch tok {
		case "q":
			stack = append(stack, ctm)
		case "Q":
			if len(stack) > 0 {
				ctm = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case "cm":
			if k := len(nums) - 6; k >= 0 {
				ctm = mat{nums[k], nums[k+1], nums[k+2], nums[k+3], nums[k+4], nums[k+5]}.then(ctm)
			}
		case "BT":
			out[st] = ctm
		}
		nums = nums[:0]
	}
	return out
}

// decodeWith turns a run's raw bytes into real text through /ToUnicode.
func decodeWith(raw string, uni map[byte]string) string {
	if uni == nil {
		return raw
	}
	var sb strings.Builder
	for i := 0; i < len(raw); i++ {
		if u, ok := uni[raw[i]]; ok {
			sb.WriteString(u)
		} else {
			sb.WriteByte(raw[i])
		}
	}
	return sb.String()
}

// reverseUni maps real characters back to this subset's codes. A subset only
// carries the glyphs the document already used, so this is also the list of
// what can be written at all.
func reverseUni(uni map[byte]string) map[rune]byte {
	codes := make([]int, 0, len(uni))
	for c := range uni {
		codes = append(codes, int(c))
	}
	sort.Ints(codes)
	enc := map[rune]byte{}
	for _, c := range codes {
		r := []rune(uni[byte(c)])
		if len(r) != 1 {
			continue
		}
		if _, dup := enc[r[0]]; !dup {
			enc[r[0]] = byte(c)
		}
	}
	return enc
}

// encodeRun writes text in a subset's own codes, reporting any character the
// font simply does not carry.
func encodeRun(s string, enc map[rune]byte) (string, []rune) {
	var sb strings.Builder
	var miss []rune
	for _, r := range s {
		c, ok := enc[r]
		if !ok {
			miss = append(miss, r)
			continue
		}
		sb.WriteByte(c)
	}
	return sb.String(), miss
}

// blockText pulls the text out of one BT..ET block. tjText only accepts the
// "[...] TJ" array form because that is all the rewriting path needs; reading
// a chart also has to cope with a bare "(...) Tj" and with hex strings, which
// is how Quartz writes most of its runs.
func blockText(body string) string {
	var sb strings.Builder
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth, j := 1, i+1
			for ; j < len(body) && depth > 0; j++ {
				if body[j] == '\\' {
					j++
					continue
				}
				if body[j] == '(' {
					depth++
				} else if body[j] == ')' {
					depth--
				}
			}
			if end := j - 1; end > i {
				sb.WriteString(unescapePDF(body[i+1 : end]))
			}
			i = j - 1
		case '<':
			if i+1 < len(body) && body[i+1] == '<' { // a dictionary, not a string
				return sb.String()
			}
			j := strings.IndexByte(body[i:], '>')
			if j < 0 {
				return sb.String()
			}
			h := body[i+1 : i+j]
			for k := 0; k+1 < len(h); k += 2 {
				v, err := strconv.ParseUint(h[k:k+2], 16, 16)
				if err != nil {
					break
				}
				sb.WriteByte(byte(v))
			}
			i += j
		}
	}
	return sb.String()
}

func num(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }

// textLines reads the page's text in reading order, whatever produced it.
func textLines(pw *pageWork) []string {
	c := string(pw.content)

	ctms := textCTMs(c)

	var segs []placedText
	for _, m := range reBT.FindAllStringSubmatchIndex(c, -1) {
		body := c[m[2]:m[3]]
		mt := reTmAny.FindStringSubmatch(body)
		mf := reTfAny.FindStringSubmatch(body)
		if mt == nil || mf == nil {
			continue
		}
		raw := blockText(body)
		if raw == "" {
			continue
		}
		txt := raw
		if fi := pw.fonts[mf[1]]; fi != nil {
			txt = decodeWith(raw, fi.uni)
		}
		ctm, ok := ctms[m[0]]
		if !ok {
			ctm = identity
		}
		tx, ty := num(mt[5]), num(mt[6])
		segs = append(segs, placedText{ctm.a*tx + ctm.c*ty + ctm.e, ctm.b*tx + ctm.d*ty + ctm.f, txt})
	}
	if len(segs) == 0 {
		return nil
	}
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].y > segs[j].y })
	var out []string
	var cur []placedText
	prev := math.Inf(1)
	flush := func() {
		if len(cur) == 0 {
			return
		}
		sort.SliceStable(cur, func(i, j int) bool { return cur[i].x < cur[j].x })
		var sb strings.Builder
		for _, s := range cur {
			sb.WriteString(s.s)
		}
		out = append(out, sb.String())
		cur = nil
	}
	for _, s := range segs {
		if prev-s.y > 8 {
			flush()
		}
		prev = s.y
		cur = append(cur, s)
	}
	flush()
	return out
}

// quartzRuns builds rewritable runs from Quartz-shaped content, where the
// text matrix comes before the font, the point size lives in that matrix and
// each run sits in its own cm. It is only used when the Word-shaped pattern
// found nothing, so that path is never disturbed.
//
// Positions and widths stay in the block's own text space (the Tm e value and
// the matrix scale), which is the space the width table is in. Only the y used
// to group lines goes through the cm, so that threshold stays in points.
func (pw *pageWork) quartzRuns() {
	c := string(pw.content)
	ctms := textCTMs(c)
	for _, m := range reBT.FindAllStringSubmatchIndex(c, -1) {
		body := c[m[2]:m[3]]
		mt := reTmAny.FindStringSubmatchIndex(body)
		mfi := reTfAny.FindStringSubmatchIndex(body)
		if mt == nil || mfi == nil {
			continue
		}
		fname := body[mfi[2]:mfi[3]]
		fi := pw.fonts[fname]
		if fi == nil || !fi.simple || fi.uni == nil {
			continue
		}
		lits := blockStrings(body, m[2])
		if len(lits) == 0 {
			continue
		}
		var raw strings.Builder
		for _, l := range lits {
			raw.WriteString(l.raw)
		}
		txt := decodeWith(raw.String(), fi.uni)
		if txt == "" {
			continue
		}
		// Tm is "a b c d e f": d scales the text, e and f place it.
		scale := num(body[mt[8]:mt[9]])
		if scale == 0 {
			scale = 1
		}
		x, ty := num(body[mt[10]:mt[11]]), num(body[mt[12]:mt[13]])
		ctm, ok := ctms[m[0]]
		if !ok {
			ctm = identity
		}
		pw.runs = append(pw.runs, &run{
			st: m[0], en: m[1], font: fname,
			sz: scale * num(body[mfi[4]:mfi[5]]), x: x, y: ctm.b*x + ctm.d*ty + ctm.f,
			ca: ctm.a, ce: ctm.c*ty + ctm.e,
			s: txt, nx: x,
			enc: reverseUni(fi.uni),
			xSt: m[2] + mt[10], xEn: m[2] + mt[11],
			fSt: m[2] + mfi[2], fEn: m[2] + mfi[3],
			lits: lits,
		})
	}
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
	var lines, general []string
	for _, pg := range pages {
		pw, err := d.loadPage(pg)
		if err != nil {
			return KeyInfo{}, err
		}
		lines = append(lines, linesOf(pw)...)
		general = append(general, textLines(pw)...)
	}
	// Word-shaped content reads straight off pw.runs; anything else (Quartz,
	// for one) only comes out through the general extractor.
	if !hasReadableText(lines) {
		lines = general
	}
	ki, err := keyFromLines(lines, fname)
	if err != nil && !hasReadableText(lines) {
		return KeyInfo{}, errors.New("no readable text in this PDF — it is probably a scan")
	}
	return ki, err
}

// hasReadableText reports whether the extracted text looks like letters at
// all, as opposed to the glyph codes a CID-keyed font leaves behind.
func hasReadableText(lines []string) bool {
	n := 0
	for _, l := range lines {
		for i := 0; i < len(l); i++ {
			if c := l[i]; (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				if n++; n >= 20 {
					return true
				}
			}
		}
	}
	return false
}

// spliceRun rewrites one Quartz block in place: the new codes go into its
// first string operand, the rest are emptied, and the Tm x is updated if the
// run moved. The block's matrix and font are left exactly as they were.
// fbFontName is the resource name used for the standard font a run falls
// back to when its own subset cannot draw the new chord.
func fbFontName(bold bool) string {
	if bold {
		return "CKCFallbackB"
	}
	return "CKCFallbackR"
}

// addFallbackFont puts a standard PDF font in the page's resources. The 14
// standard fonts need no embedding - every viewer has them - and
// Helvetica-Bold matches the Arial-Bold these charts use.
func (d *PDF) addFallbackFont(pg Page, bold bool) bool {
	name := fbFontName(bold)
	base := "Helvetica"
	if bold {
		base = "Helvetica-Bold"
	}
	fv, _ := d.Resolve(pg.Res.Get("Font"))
	fd, ok := fv.(*Dict)
	if !ok {
		return false
	}
	if _, done := fd.M[name]; done {
		return true
	}
	nf := NewDict()
	nf.Set("Type", Name("Font"))
	nf.Set("Subtype", Name("Type1"))
	nf.Set("BaseFont", Name(base))
	nf.Set("Encoding", Name("WinAnsiEncoding"))

	n := 0
	for k := range d.xref {
		if k > n {
			n = k
		}
	}
	for k := range d.Modified {
		if k > n {
			n = k
		}
	}
	n++
	d.Modified[n] = nf

	ndf := fd.Clone()
	ndf.Set(name, Ref{N: n})
	if fr, isRef := pg.Res.Get("Font").(Ref); isRef {
		d.Modified[fr.N] = ndf
		return true
	}
	if rr, isRef := pg.D.Get("Resources").(Ref); isRef {
		nres := pg.Res.Clone()
		nres.Set("Font", ndf)
		d.Modified[rr.N] = nres
		return true
	}
	return false
}

// transposeWord transposes one whole word. A "Key: G" heading is not chord
// text, but the note in it still has to move with the song.
func transposeWord(s string, n int) string {
	if isChordText(s) {
		return transposeText(s, n)
	}
	if m := reKey.FindStringSubmatchIndex(s); m != nil {
		return s[:m[2]] + transposeText(s[m[2]:m[3]], n) + s[m[3]:]
	}
	return s
}

// quartzTranspose decides the new text for Quartz runs. Quartz breaks text at
// arbitrary points - "Chorus" arrives as "C" then "horus" - so a block is only
// a chord when the whole word around it is one, or the lone "C" gets
// transposed into an "F". Touching blocks are joined, the word is transposed
// as a unit, and the result goes into the first block with the rest emptied.
func (pw *pageWork) quartzTranspose(n int) {
	by := map[int][]*run{}
	for _, r := range pw.runs {
		by[r.line] = append(by[r.line], r)
	}
	for _, rs := range by {
		sort.Slice(rs, func(i, j int) bool { return rs[i].px() < rs[j].px() })
		for i := 0; i < len(rs); {
			var sb strings.Builder
			sb.WriteString(rs[i].s)
			j := i + 1
			for j < len(rs) {
				prev, cur := rs[j-1], rs[j]
				if strings.TrimSpace(prev.s) == "" || strings.TrimSpace(cur.s) == "" {
					break
				}
				k := prev.scale()
				sw := pw.wd(prev, " ") * k
				if sw <= 0 {
					sw = prev.sz * 0.25 * k
				}
				if cur.px()-(prev.px()+pw.wd(prev, prev.s)*k) > 0.35*sw {
					break
				}
				// Only blocks set the same way are parts of one word: a
				// superscript "7" has its own font, size and baseline and is
				// an extension, not part of the root. A raised sharp or flat
				// is the exception - it is part of the chord, and leaving it
				// out transposes the wrong note ("D/F#" read as "D/F").
				if cur.font != prev.font || math.Abs(cur.sz-prev.sz) > 0.01 ||
					math.Abs(cur.y-prev.y) > 0.5 {
					if t := strings.TrimSpace(cur.s); (t != "#" && t != "b") || !isChordText(sb.String()) {
						break
					}
				}
				sb.WriteString(cur.s)
				j++
			}
			word := sb.String()
			if nw := transposeWord(word, n); nw != word {
				// The whole word is written into its first block. That block
				// now stands for the word, so it carries the old text too -
				// otherwise the width comparison that spaces the line would
				// weigh a whole chord against one fragment of it.
				rs[i].s, rs[i].nw = word, nw
				for _, r := range rs[i+1 : j] {
					r.nw, r.blank = "", true
				}
			} else {
				for _, r := range rs[i:j] {
					r.nw = r.s
				}
			}
			i = j
		}
	}
}

func spliceRun(c string, r *run, nb string) string {
	type ed struct {
		st, en int
		s      string
	}
	var eds []ed
	if math.Abs(r.nx-r.x) > 1e-6 {
		eds = append(eds, ed{r.xSt, r.xEn, strconv.FormatFloat(r.nx, 'f', 2, 64)})
	}
	if r.fb {
		eds = append(eds, ed{r.fSt, r.fEn, fbFontName(r.fbBold)})
	}
	for i, l := range r.lits {
		s := ""
		if i == 0 {
			s = "(" + escapePDF(nb) + ")"
		}
		eds = append(eds, ed{l.st, l.en, s})
	}
	sort.Slice(eds, func(i, j int) bool { return eds[i].st < eds[j].st })
	var sb strings.Builder
	pos := r.st
	for _, e := range eds {
		if e.st < pos {
			continue
		}
		sb.WriteString(c[pos:e.st])
		sb.WriteString(e.s)
		pos = e.en
	}
	sb.WriteString(c[pos:r.en])
	return sb.String()
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
	// Rewriting works from pw.runs. When a PDF yields none of those but does
	// contain text, converting it would quietly produce an identical file, so
	// say what is wrong instead.
	runs := 0
	var readable []string
	for _, pg := range pages {
		pw, err := d.loadPage(pg)
		if err != nil {
			return nil, 0, nil, err
		}
		runs += len(pw.runs)
		readable = append(readable, textLines(pw)...)
	}
	if runs == 0 {
		if !hasReadableText(readable) {
			return nil, 0, nil, errors.New("no text found to convert in this PDF — it is probably a scan")
		}
		return nil, 0, nil, errors.New("this PDF's text is laid out in a way this app cannot rewrite. Convert the .docx version instead")
	}

	// Characters the transposed chords need that an embedded subset cannot
	// draw. A subset carries only the glyphs the document already used, and
	// one missing letter is enough to spoil the chart, so this is fatal.
	cannot := map[rune]bool{}
	count := 0
	warn := map[string]bool{}
	for _, pg := range pages {
		pw, err := d.loadPage(pg)
		if err != nil {
			return nil, 0, nil, err
		}
		quartz := len(pw.runs) > 0 && pw.runs[0].enc != nil
		if quartz {
			pw.quartzTranspose(n)
		} else {
			for _, r := range pw.runs {
				r.nw = transposeText(r.s, n)
			}
		}
		for _, r := range pw.runs {
			if r.enc == nil || r.nw == r.s {
				continue
			}
			// A subset carries only the glyphs the document already used, so a
			// new key often needs a letter that is not in the file. Those runs
			// are written in a standard font instead.
			if _, miss := encodeRun(r.nw, r.enc); len(miss) > 0 {
				r.fb = true
				if fi := pw.fonts[r.font]; fi != nil {
					r.fbBold = fi.bold
				}
			}
		}
		// push following text right when a chord got wider
		by := map[int][]*run{}
		for _, r := range pw.runs {
			if r.blank {
				continue // no longer drawn; its word is measured at its first block
			}
			by[r.line] = append(by[r.line], r)
		}
		for _, rs := range by {
			// Measured in page points: runs placed by different transforms
			// share no text space. For an ordinary run the scale is 1 and the
			// offset 0, so this is exactly the arithmetic it always did.
			sort.Slice(rs, func(i, j int) bool { return rs[i].px() < rs[j].px() })
			shift := 0.0
			for i, r := range rs {
				k := r.scale()
				r.nx = r.x + shift/k
				if i+1 >= len(rs) {
					continue
				}
				nxt := rs[i+1]
				oend := r.px() + pw.wd(r, r.s)*k
				nend := r.px() + shift + pw.wd(r, r.nw)*k
				if math.Abs(nxt.px()-oend) < 1.5 {
					shift = nend - oend
				} else {
					gap := math.Max(nxt.px()-oend, 0)
					shift = math.Max(0, nend+math.Min(gap, 5.5)-nxt.px())
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
			var t string
			if r.enc != nil {
				nb := r.nw
				if !r.fb {
					var miss []rune
					nb, miss = encodeRun(r.nw, r.enc)
					if len(miss) > 0 {
						for _, ch := range miss {
							cannot[ch] = true
						}
						continue
					}
				} else if !d.addFallbackFont(pg, r.fbBold) {
					for _, ch := range r.nw {
						cannot[ch] = true
					}
					continue
				}
				t = spliceRun(c, r, nb)
			} else {
				orig := c[r.st:r.en]
				loc := reTm.FindStringIndex(orig)
				t = orig[:loc[0]] + fmt.Sprintf("1 0 0 1 %.2f ", r.nx) + orig[loc[1]:]
				tl := reTJ.FindStringIndex(t)
				t = t[:tl[0]] + "[(" + escapePDF(r.nw) + ")] TJ" + t[tl[1]:]
			}
			sb.WriteString(c[pos:r.st])
			sb.WriteString(t)
			pos = r.en
			changed = true
			if r.nw != r.s {
				if r.nw != "" {
					count++
				}
				if r.enc != nil {
					continue // a subset's own codes already have widths
				}
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
	if len(cannot) > 0 {
		var chars []string
		for r := range cannot {
			chars = append(chars, strconv.Quote(string(r)))
		}
		sort.Strings(chars)
		return nil, 0, nil, fmt.Errorf(
			"this key needs %s, which the chart's embedded font does not contain "+
				"(it carries only the characters the document already used). "+
				"Convert the .docx version instead", strings.Join(chars, ", "))
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
