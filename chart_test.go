package main

import (
	"archive/zip"
	"bytes"
	"image/png"
	"io"
	"strings"
	"testing"
)

const sampleChart = `title: 海屿你
tempo: 75
key: G
intro: | G | D/F# | Em | C |
## Verse
[G]从不主动示弱
[D/F#]我们的过去 [Em7]分分合合太多
## Bridge
[D#maj7]爱 还会不会回来
(此段截图没有 chord，请补上)
===
## Chorus
[C]我不知为何 [D]疯狂对你执着
[D]求你别离开我　[G]
`

func docXML(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			x, _ := io.ReadAll(rc)
			rc.Close()
			return string(x)
		}
	}
	t.Fatal("no document.xml")
	return ""
}

func TestParseChart(t *testing.T) {
	c := ParseChart(sampleChart)
	if c.Title != "海屿你" || c.Tempo != "75" || c.Key != "G" {
		t.Fatalf("meta: %+v", c)
	}
	if len(c.Intro) != 1 || strings.Join(c.Intro[0], ",") != "G,D/F#,Em,C" {
		t.Fatalf("intro: %v", c.Intro)
	}
	var l ChartLine
	for _, x := range c.Lines {
		if x.Kind == "lyric" && strings.HasPrefix(x.Text, "我们的过去") {
			l = x
		}
	}
	if l.Text != "我们的过去 分分合合太多" || len(l.Chords) != 2 {
		t.Fatalf("lyric: %+v", l)
	}
	// 5 CJK chars + a half-width space = 5.5em
	if l.Chords[0].X != 0 || l.Chords[1].X != 5.5*lyricPt {
		t.Fatalf("positions: %+v", l.Chords)
	}
}

func TestBuildChartDocxRoundTrip(t *testing.T) {
	c := ParseChart(sampleChart)
	b, warns, err := BuildChartDocx(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	x := docXML(t, b)
	for _, want := range []string{"海屿你", "Key: ", "| D#maj", `w:num="2"`, `w:type="column"`, "SimHei"} {
		if !strings.Contains(x, want) {
			t.Errorf("document.xml missing %q", want)
		}
	}

	// the key changer must recognise and transpose what the chart maker writes
	k, err := DetectDocx(b, "chart.docx")
	if err != nil || k.Key != "G" {
		t.Fatalf("detect: %v %v", k, err)
	}
	out, n, err := ConvertDocx(b, 2) // G -> A
	if err != nil || n == 0 {
		t.Fatalf("convert: %d %v", n, err)
	}
	y := docXML(t, out)
	for _, want := range []string{"| E/G#", "| F#", "| Fmaj", ">A<"} {
		if !strings.Contains(y, want) {
			t.Errorf("transposed chart missing %q", want)
		}
	}
	if !strings.Contains(y, "海屿你") || strings.Contains(y, "| D/F#") {
		t.Error("lyrics changed or chords not transposed")
	}
}

func TestChartFileName(t *testing.T) {
	if got := ChartFileName(Chart{Title: "泪海", Key: "D"}); got != "D Key - 泪海.docx" {
		t.Fatal(got)
	}
	if got := ChartFileName(Chart{Title: "a/b"}); got != "a-b.docx" {
		t.Fatal(got)
	}
}

func TestDibToPNG(t *testing.T) {
	// 2x1 bottom-up 24-bit DIB: red, blue
	d := make([]byte, 40+8)
	d[0] = 40
	d[4], d[8] = 2, 1
	d[12], d[14] = 1, 24
	copy(d[40:], []byte{0, 0, 255, 255, 0, 0})
	b, err := dibToPNG(d)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := img.At(0, 0).RGBA(); r>>8 != 255 {
		t.Fatal("first pixel should be red")
	}
	if _, _, bl, _ := img.At(1, 0).RGBA(); bl>>8 != 255 {
		t.Fatal("second pixel should be blue")
	}
}
