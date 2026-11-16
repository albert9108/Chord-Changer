package main

// Chart maker: turns chart text such as
//
//	title: 我选择喜乐
//	tempo: 145
//	key: G
//	intro: | C | D | G | Em7 |
//	## Verse
//	[C]我来到[D]神面前
//
// into a Word chart in the house layout (two columns, blue bold Arial chords
// over SimHei lyrics, boxed section names). [X] marks where a bar starts;
// [Em7 D] is two chords in one bar.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	baseLyricPt = 18.0
	baseChordPt = 11.0
	colWidthPt  = 250.0
	pageHeight  = 841.9 // A4, points
	minScale    = 0.6
	chordBlue   = "0070C0"
	chordRed    = "C00000"
	noteGrey    = "808080"
	pageMarginT = 720 // 1.27 cm in twips
)

// Current font sizes. BuildChartDocx shrinks them so the chart fits one page.
var (
	lyricPt = baseLyricPt
	chordPt = baseChordPt
	scale   = 1.0
)

type ChartLine struct {
	Kind   string   // "lyric", "section", "note", "blank", "colbreak", "bars"
	Bars   []string // for "bars": an instrumental row such as | C | F | C | G |
	Text   string
	Chords []ChartMark
}

type ChartMark struct {
	X     float64 // points from the line start
	Chord string
}

type Chart struct {
	Title, Tempo, Key string
	Intro             [][]string // rows of bars
	Lines             []ChartLine
}

var reMeta = regexp.MustCompile(`(?i)^(title|tempo|key|intro)\s*[:：]\s*(.*)$`)
var reMark = regexp.MustCompile(`\[[^\]]+\]`)

func charWidth(r rune) float64 {
	switch {
	case r == '　':
		return baseLyricPt
	case r < 0x2E80:
		return baseLyricPt * 0.5
	}
	return baseLyricPt
}

func textWidth(s string) float64 {
	w := 0.0
	for _, r := range s {
		w += charWidth(r)
	}
	return w
}

func ParseChart(src string) Chart {
	var c Chart
	body := false
	for _, raw := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		s := strings.TrimSpace(raw)
		if !body {
			if m := reMeta.FindStringSubmatch(s); m != nil {
				v := strings.TrimSpace(m[2])
				switch strings.ToLower(m[1]) {
				case "title":
					c.Title = v
				case "tempo":
					c.Tempo = v
				case "key":
					c.Key = v
				case "intro":
					for _, row := range strings.Split(v, "//") {
						if bars := parseBars(row); len(bars) > 0 {
							c.Intro = append(c.Intro, bars)
						}
					}
				}
				continue
			}
			if s == "" {
				continue
			}
			body = true
		}
		switch {
		case s == "":
			c.Lines = append(c.Lines, ChartLine{Kind: "blank"})
		case s == "===":
			c.Lines = append(c.Lines, ChartLine{Kind: "colbreak"})
		case strings.HasPrefix(s, "|") && !reMark.MatchString(s):
			c.Lines = append(c.Lines, ChartLine{Kind: "bars", Bars: parseBars(s)})
		case strings.HasPrefix(s, "##"):
			c.Lines = append(c.Lines, ChartLine{Kind: "section", Text: strings.TrimSpace(strings.TrimLeft(s, "#"))})
		case (strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")) ||
			(strings.HasPrefix(s, "（") && strings.HasSuffix(s, "）")):
			c.Lines = append(c.Lines, ChartLine{Kind: "note", Text: s})
		default:
			c.Lines = append(c.Lines, splitLong(parseLyric(s))...)
		}
	}
	return c
}

// splitLong breaks a lyric line wider than a column into several lines,
// preferring a space near the middle, and keeps each chord on its syllable.
func splitLong(l ChartLine) []ChartLine {
	if textWidth(l.Text) <= colWidthPt {
		return []ChartLine{l}
	}
	type pos struct {
		byteAt int
		x      float64
		space  bool
	}
	var ps []pos
	x := 0.0
	for i, r := range l.Text {
		ps = append(ps, pos{i, x, r == ' ' || r == '　'})
		x += charWidth(r)
	}
	total := x
	best, bestScore := -1, 1e18
	for k, p := range ps {
		if k == 0 || p.x > colWidthPt {
			continue
		}
		score := p.x - total/2
		if score < 0 {
			score = -score
		}
		if !p.space {
			score += 10000 // break inside a phrase only if there is no space
		}
		if score < bestScore {
			best, bestScore = k, score
		}
	}
	if best < 0 {
		return []ChartLine{l}
	}
	cut := ps[best]
	leftText := strings.TrimRight(l.Text[:cut.byteAt], " 　")
	rest := l.Text[cut.byteAt:]
	shift := cut.x
	if cut.space {
		_, w := utf8.DecodeRuneInString(rest)
		rest = rest[w:]
		shift += charWidth([]rune(l.Text[cut.byteAt:])[0])
	}
	left := ChartLine{Kind: "lyric", Text: leftText}
	right := ChartLine{Kind: "lyric", Text: rest}
	for _, m := range l.Chords {
		if m.X < cut.x || (!cut.space && m.X < shift) {
			left.Chords = append(left.Chords, m)
		} else {
			m.X -= shift
			if m.X < 0 {
				m.X = 0
			}
			right.Chords = append(right.Chords, m)
		}
	}
	return append([]ChartLine{left}, splitLong(right)...)
}

func parseBars(row string) []string {
	var bars []string
	for _, b := range strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|") {
		if b = strings.TrimSpace(b); b != "" {
			bars = append(bars, b)
		}
	}
	return bars
}

func parseLyric(s string) ChartLine {
	l := ChartLine{Kind: "lyric"}
	var lyric strings.Builder
	x, last := 0.0, 0
	for _, loc := range reMark.FindAllStringIndex(s, -1) {
		seg := s[last:loc[0]]
		lyric.WriteString(seg)
		x += textWidth(seg)
		l.Chords = append(l.Chords, ChartMark{X: x, Chord: strings.TrimSpace(s[loc[0]+1 : loc[1]-1])})
		last = loc[1]
	}
	lyric.WriteString(s[last:])
	l.Text = lyric.String()
	return l
}

// ---------------------------------------------------------------- docx

func xesc(s string) string { return html.EscapeString(s) }

func rPr(font, eastAsia string, pt float64, bold bool, color string, extra string) string {
	if eastAsia == "" {
		eastAsia = font
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<w:rPr><w:rFonts w:ascii="%s" w:hAnsi="%s" w:eastAsia="%s" w:cs="%s"/>`,
		font, font, eastAsia, font)
	if bold {
		b.WriteString(`<w:b/><w:bCs/>`)
	}
	if color != "" {
		fmt.Fprintf(&b, `<w:color w:val="%s"/>`, color)
	}
	hp := int(pt*2 + 0.5)
	fmt.Fprintf(&b, `<w:sz w:val="%d"/><w:szCs w:val="%d"/>%s</w:rPr>`, hp, hp, extra)
	return b.String()
}

func wrun(props, text string) string {
	return `<w:r>` + props + `<w:t xml:space="preserve">` + xesc(text) + `</w:t></w:r>`
}

func tabRun() string { return `<w:r><w:tab/></w:r>` }

func para(ppr, runs string) string { return `<w:p><w:pPr>` + ppr + `</w:pPr>` + runs + `</w:p>` }

func spacing(after float64) string {
	return fmt.Sprintf(`<w:spacing w:before="0" w:after="%d" w:line="240" w:lineRule="auto"/>`, int(after*20))
}

func tabs(posPt []float64, align string) string {
	if len(posPt) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<w:tabs>`)
	for _, p := range posPt {
		fmt.Fprintf(&b, `<w:tab w:val="%s" w:pos="%d"/>`, align, int(p*20+0.5))
	}
	b.WriteString(`</w:tabs>`)
	return b.String()
}

var reChordSplit = regexp.MustCompile(`\(4n\)|[0-9]+`)

// chordRuns writes one chord as runs: digits superscript, (4n) red superscript.
// '#' stays in the same run as its letter so the key changer can transpose it.
func chordRuns(text string) string {
	var b strings.Builder
	norm := rPr("Arial", "", chordPt, true, chordBlue, "")
	sup := rPr("Arial", "", chordPt, true, chordBlue, `<w:vertAlign w:val="superscript"/>`)
	red := rPr("Arial", "", chordPt-2, true, chordRed, `<w:vertAlign w:val="superscript"/>`)
	last := 0
	for _, loc := range reChordSplit.FindAllStringIndex(text, -1) {
		if loc[0] > last {
			b.WriteString(wrun(norm, text[last:loc[0]]))
		}
		tok := text[loc[0]:loc[1]]
		if tok == "(4n)" {
			b.WriteString(wrun(red, tok))
		} else {
			b.WriteString(wrun(sup, tok))
		}
		last = loc[1]
	}
	if last < len(text) {
		b.WriteString(wrun(norm, text[last:]))
	}
	return b.String()
}

func chordLine(marks []ChartMark) string {
	var stops []float64
	var runs strings.Builder
	lastEnd := -99.0
	for _, m := range marks {
		pos := m.X*scale - 5*scale // '|' sits just left of the syllable
		if pos < lastEnd+4 {
			pos = lastEnd + 4
		}
		if pos < 0 {
			pos = 0
		}
		if pos >= 1 { // Word ignores a tab stop at 0 and would shift every chord
			stops = append(stops, pos)
			runs.WriteString(tabRun())
		}
		label := "| " + m.Chord
		runs.WriteString(chordRuns(label + " "))
		lastEnd = pos + float64(len(label))*chordPt*0.6
	}
	return para(tabs(stops, "left")+spacing(0), runs.String())
}

func sectPr(cols int, continuous bool) string {
	t := ""
	if continuous {
		t = `<w:type w:val="continuous"/>`
	}
	return fmt.Sprintf(`<w:sectPr>%s<w:pgSz w:w="11906" w:h="16838"/>`+
		`<w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="709" w:footer="709" w:gutter="0"/>`+
		`<w:cols w:num="%d" w:space="425"/></w:sectPr>`,
		t, pageMarginT, pageMarginT, pageMarginT, pageMarginT, cols)
}

// BuildChartDocx renders the chart. Warnings list lines too wide for a column.
func BuildChartDocx(c Chart) ([]byte, []string, error) {
	var body strings.Builder
	var warns []string

	title := rPr("Microsoft YaHei UI", "", 14, true, "", `<w:u w:val="single"/>`)
	body.WriteString(para(`<w:jc w:val="center"/>`+spacing(4), wrun(title, c.Title)))

	// tempo left, key right; "Key: " and the key in separate runs so the key
	// changer recognises the key on its own.
	head := wrun(rPr("Segoe UI Symbol", "", 9, false, "", ""), "♩ = "+c.Tempo) +
		tabRun() + wrun(rPr("Arial", "", 9, true, "", ""), "Key: ")
	if c.Key != "" {
		head += wrun(rPr("Arial", "", 9, true, "", ""), c.Key)
	}
	body.WriteString(para(tabs([]float64{523}, "right")+spacing(4), head))

	// The intro row is always there, empty bars when the source has none.
	intro := c.Intro
	if len(intro) == 0 {
		intro = [][]string{{"", "", "", ""}}
	}
	for i, bars := range intro {
		label := ""
		if i == 0 {
			label = "Intro:"
		}
		body.WriteString(barRow(label, bars, 56.7, 102, baseChordPt))
	}

	// end of the one-column heading section
	body.WriteString(`<w:p><w:pPr>` + spacing(0) + sectPr(1, false) +
		`<w:rPr><w:sz w:val="4"/></w:rPr></w:pPr></w:p>`)

	header := 14*1.3 + 4 + 9*1.2 + 4 + float64(len(intro))*(baseChordPt*1.25+2) + 4
	split := layoutColumns(c.Lines, pageHeight-2*36-header-24)
	defer func() { lyricPt, chordPt, scale = baseLyricPt, baseChordPt, 1 }()

	for i, l := range c.Lines {
		var ps []string
		switch l.Kind {
		case "blank":
			ps = append(ps, `<w:p><w:pPr>`+spacing(0)+
				fmt.Sprintf(`<w:rPr><w:sz w:val="%d"/></w:rPr>`, int(12*scale))+`</w:pPr></w:p>`)
		case "colbreak":
			continue
		case "section":
			bdr := `<w:bdr w:val="single" w:sz="6" w:space="0" w:color="000000"/>`
			ps = append(ps, para(fmt.Sprintf(`<w:spacing w:before="%d" w:after="40" w:line="240" w:lineRule="auto"/>`, int(200*scale)),
				wrun(rPr("Arial", "", 12*scale, true, "", bdr), l.Text)))
		case "note":
			ps = append(ps, para(spacing(2), wrun(rPr("Arial", "SimHei", 9, false, noteGrey, ""), l.Text)))
		case "bars":
			ps = append(ps, barRow("", l.Bars, 0, 60*scale, chordPt))
		case "lyric":
			if len(l.Chords) > 0 {
				ps = append(ps, chordLine(l.Chords))
			}
			ps = append(ps, para(spacing(1), wrun(rPr("SimHei", "SimHei", lyricPt, false, "", ""), l.Text)))
			if w := textWidth(l.Text) * scale; w > colWidthPt {
				warns = append(warns, "Line may be too long for one column: "+l.Text)
			}
		}
		if i == split && len(ps) > 0 {
			ps[0] = strings.Replace(ps[0], "</w:pPr>", `</w:pPr><w:r><w:br w:type="column"/></w:r>`, 1)
		}
		body.WriteString(strings.Join(ps, ""))
	}
	if scale < minScale+0.001 {
		warns = append(warns, "The song is very long — it may not fit on one page even at the smallest size.")
	}

	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + body.String() + sectPr(2, true) + `</w:body></w:document>`

	files := []struct{ name, data string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`},
		{"word/document.xml", doc},
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, nil, err
		}
		if _, err := w.Write([]byte(f.data)); err != nil {
			return nil, nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, nil, err
	}
	return out.Bytes(), warns, nil
}

// barRow writes a row of chord bars: label, then | C | F | … at tab stops.
func barRow(label string, bars []string, first, step, pt float64) string {
	var stops []float64
	runs := ""
	if label != "" {
		runs = wrun(rPr("Arial", "", 12, true, "", ""), label)
	}
	for k, b := range bars {
		pos := first + float64(k)*step
		if pos >= 1 {
			stops = append(stops, pos)
			runs += tabRun()
		}
		saved := chordPt
		chordPt = pt
		runs += chordRuns("| " + b)
		chordPt = saved
	}
	end := first + float64(len(bars))*step
	stops = append(stops, end)
	runs += tabRun() + wrun(rPr("Arial", "", pt, true, chordBlue, ""), "|")
	return para(tabs(stops, "left")+spacing(2), runs)
}

// lineHeight estimates a line's height in points at the current scale.
func lineHeight(l ChartLine) float64 {
	switch l.Kind {
	case "blank":
		return 12 * scale * 1.2
	case "section":
		return 10*scale + 12*scale*1.25 + 2
	case "note":
		return 9*1.25 + 2
	case "bars":
		return chordPt*1.25 + 2
	case "lyric":
		h := lyricPt*1.25 + 1
		if len(l.Chords) > 0 {
			h += chordPt * 1.25
		}
		return h
	}
	return 0
}

func setScale(s float64) {
	scale = s
	lyricPt = baseLyricPt * s
	chordPt = baseChordPt * s
}

// layoutColumns picks where the second column starts so both columns are about
// the same height, and shrinks the fonts until the taller column fits avail.
// A "===" line in the text forces the split. Returns the index of the first
// line of the right column (-1 for none).
func layoutColumns(lines []ChartLine, avail float64) int {
	forced := -1
	for i, l := range lines {
		if l.Kind == "colbreak" {
			forced = i + 1
			for forced < len(lines) && lines[forced].Kind == "blank" {
				forced++
			}
			break
		}
	}
	best := -1
	s := 1.0
	for iter := 0; iter < 6; iter++ {
		setScale(s)
		h := make([]float64, len(lines))
		total := 0.0
		for i, l := range lines {
			h[i] = lineHeight(l)
			total += h[i]
		}
		bestCost, tall := 1e18, total
		left := 0.0
		for i := 0; i < len(lines); i++ {
			if i > 0 {
				left += h[i-1]
			}
			k := lines[i].Kind
			if k == "blank" || k == "colbreak" {
				continue
			}
			if forced >= 0 && i != forced {
				continue
			}
			right := total - left
			cost := left
			if right > cost {
				cost = right
			}
			if k != "section" {
				cost += 10000 // start the column at a section whenever there is one
			}
			if cost < bestCost {
				bestCost, best = cost, i
				tall = left
				if right > tall {
					tall = right
				}
			}
		}
		if tall <= avail || s <= minScale {
			break
		}
		s *= avail / tall * 0.99
		if s < minScale {
			s = minScale
		}
	}
	return best
}

// ChartFileName gives "G Key - Title.docx", like the charts already on file.
func ChartFileName(c Chart) string {
	t := strings.TrimSpace(c.Title)
	if t == "" {
		t = "New chart"
	}
	t = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return '-'
		}
		return r
	}, t)
	if k := strings.TrimSpace(c.Key); k != "" {
		return k + " Key - " + t + ".docx"
	}
	return t + ".docx"
}
