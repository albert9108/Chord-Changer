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
)

const (
	lyricPt     = 18.0
	chordPt     = 11.0
	colWidthPt  = 250.0
	chordBlue   = "0070C0"
	chordRed    = "C00000"
	noteGrey    = "808080"
	pageMarginT = 720 // 1.27 cm in twips
)

type ChartLine struct {
	Kind   string // "lyric", "section", "note", "blank", "colbreak"
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
		return lyricPt
	case r < 0x2E80:
		return lyricPt * 0.5
	}
	return lyricPt
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
						var bars []string
						for _, b := range strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|") {
							if b = strings.TrimSpace(b); b != "" {
								bars = append(bars, b)
							}
						}
						if len(bars) > 0 {
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
		case strings.HasPrefix(s, "##"):
			c.Lines = append(c.Lines, ChartLine{Kind: "section", Text: strings.TrimSpace(strings.TrimLeft(s, "#"))})
		case (strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")) ||
			(strings.HasPrefix(s, "（") && strings.HasSuffix(s, "）")):
			c.Lines = append(c.Lines, ChartLine{Kind: "note", Text: s})
		default:
			c.Lines = append(c.Lines, parseLyric(s))
		}
	}
	return c
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
		pos := m.X - 5 // '|' sits just left of the syllable
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

	for i, bars := range c.Intro {
		var stops []float64
		for k := range bars {
			stops = append(stops, 56.7+float64(k)*102)
		}
		label := ""
		if i == 0 {
			label = "Intro:"
		}
		runs := wrun(rPr("Arial", "", 12, true, "", ""), label)
		for _, b := range bars {
			runs += tabRun() + chordRuns("| "+b)
		}
		body.WriteString(para(tabs(stops, "left")+spacing(2), runs))
	}

	// end of the one-column heading section
	body.WriteString(`<w:p><w:pPr>` + spacing(0) + sectPr(1, false) + `</w:pPr></w:p>`)

	for _, l := range c.Lines {
		switch l.Kind {
		case "blank":
			body.WriteString(para(spacing(0), ""))
		case "colbreak":
			body.WriteString(para(spacing(0), `<w:r><w:br w:type="column"/></w:r>`))
		case "section":
			bdr := `<w:bdr w:val="single" w:sz="6" w:space="0" w:color="000000"/>`
			body.WriteString(para(`<w:spacing w:before="200" w:after="40" w:line="240" w:lineRule="auto"/>`,
				wrun(rPr("Arial", "", 12, true, "", bdr), l.Text)))
		case "note":
			body.WriteString(para(spacing(2), wrun(rPr("Arial", "SimHei", 9, false, noteGrey, ""), l.Text)))
		case "lyric":
			if len(l.Chords) > 0 {
				body.WriteString(chordLine(l.Chords))
			}
			body.WriteString(para(spacing(1), wrun(rPr("SimHei", "SimHei", lyricPt, false, "", ""), l.Text)))
			if w := textWidth(l.Text); w > colWidthPt {
				warns = append(warns, "Line may be too long for one column: "+l.Text)
			}
		}
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
