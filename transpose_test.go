package main

import "testing"

func TestKeyFromLines(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		fname string
		want  string
	}{
		{"key line wins", []string{"Key: G", "C F G C"}, "song.pdf", "G"},
		{"file name when no key line", []string{"C F G C"}, "D Key - Song.pdf", "D"},

		{"major from chords", []string{"C  F  G  C", "Am F C G", "C F G C"}, "song.pdf", "C"},
		{"G major from chords", []string{"G C D Em", "G C D G"}, "song.pdf", "G"},
		{"natural minor", []string{"Am Dm G C", "Am Dm Em Am"}, "song.pdf", "Am"},
		{"harmonic minor V", []string{"Am Dm E Am", "Am F E Am"}, "song.pdf", "Am"},

		// Em and G share every diatonic chord; starting and ending on Em
		// is what separates them.
		{"relative minor split by tonic", []string{"Em C G D", "Em C G D", "Em"}, "song.pdf", "Em"},
		{"relative major split by tonic", []string{"G C D Em", "G C D Em", "G"}, "song.pdf", "G"},

		{"flat key normalised", []string{"Bb Eb F Bb", "Gm Eb Bb F", "Bb"}, "song.pdf", "Bb"},
		{"sharp key", []string{"F# B C# F#", "D#m B F# C#", "F#"}, "song.pdf", "F#"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := keyFromLines(c.lines, c.fname)
			if err != nil {
				t.Fatalf("keyFromLines: %v", err)
			}
			if got.Key != c.want {
				t.Errorf("key = %q, want %q (source %q)", got.Key, c.want, got.Source)
			}
		})
	}
}

// A stated key still wins, but a clear disagreement is reported.
func TestKeyLineCrossCheck(t *testing.T) {
	lines := []string{"Key: C", "Am Dm E Am", "Am Dm E Am", "Am"}
	got, err := keyFromLines(lines, "song.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "C" {
		t.Fatalf("key = %q, want C (the stated key must win)", got.Key)
	}
	if want := `the "Key:" line (the chords look more like Am)`; got.Source != want {
		t.Errorf("source = %q, want %q", got.Source, want)
	}
}

// Lyrics that happen to be spellable as chords must not drown out the chords.
func TestLyricsIgnored(t *testing.T) {
	lines := []string{
		"G        C        D        G",
		"Dad bade a a dead cab",
		"G        C        D        G",
		"G",
	}
	got, err := keyFromLines(lines, "song.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "G" {
		t.Errorf("key = %q, want G (source %q)", got.Key, got.Source)
	}
}

func TestTooFewChords(t *testing.T) {
	if _, err := keyFromLines([]string{"just some words here"}, "song.pdf"); err == nil {
		t.Error("expected an error when there is nothing to go on")
	}
}

func TestHasReadableText(t *testing.T) {
	if hasReadableText([]string{"\x03\x01\x04", "\x02\x07"}) {
		t.Error("glyph codes must not count as readable text")
	}
	if !hasReadableText([]string{"Amazing grace how sweet the sound"}) {
		t.Error("plain text must count as readable")
	}
}

// Chord lines as they come out of a real two-column chart, including the
// section labels and performance marks that sit among the chords, and lyric
// lines that must not count.
func TestKeyFromRealChartLayout(t *testing.T) {
	cases := []struct {
		name  string
		want  string
		lines []string
	}{
		{"G chart", "G", []string{
			"我选择喜乐 I Will Be Joyful ",
			"  q  = 145 ",
			"Intro:  ||: C  | D | G | Em7 :|| ",
			"||: Am7  | D | G | G (4n) :|| ",
			"Verse Chorus 1 ",
			"| C | D | C | D ",
			"我来到神面前 我选择喜乐 喜乐 ",
			"| G | Em7 | G | Em7 ",
			"最喜乐神面前 喜乐 喜乐    ",
			"| Am7 | D | G | G (4n) | Am7 | D | G | G ",
			"弹琴 跳舞 大声欢呼 靠主 常常喜乐 ",
			"| G ",
		}},
		{"C chart", "C", []string{
			"定睛在耶稣身上 Jesus, Set My Eyes on You ",
			"  q  = 160 ",
			"Intro:  ||: C  | C | F | F  :|| ",
			"||: Am7  | Am7 | F | F  :|| ",
			"Verse Chorus ",
			"| C | G | C | C ",
			"看似有风浪逼近 定睛在耶稣身上 ",
			"| Am7 | Em | G | G ",
			"在船上失去平静 祂使狂风平息啦 ",
			"| F | C/E | Am7 | Am7 ",
			"记得呼求耶稣的名 定睛在耶稣身上 ",
			"| Dm7 | G | F | F/G G ",
			"| C ",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// no "Key:" line and no key in the file name: the chords alone
			got, err := keyFromLines(c.lines, "untitled.pdf")
			if err != nil {
				t.Fatalf("keyFromLines: %v", err)
			}
			if got.Key != c.want {
				t.Errorf("key = %q, want %q (source %q)", got.Key, c.want, got.Source)
			}
			// and the stated key must still win, without a spurious note
			withKey := append([]string{"Key: " + c.want}, c.lines...)
			got, err = keyFromLines(withKey, "untitled.pdf")
			if err != nil {
				t.Fatal(err)
			}
			if got.Key != c.want || got.Source != `the "Key:" line` {
				t.Errorf("stated key: got %q / %q, want %q / the \"Key:\" line", got.Key, got.Source, c.want)
			}
		})
	}
}

// Performance marks and section labels sit among chords without disqualifying
// the line; a lyric line never qualifies.
func TestChordLineFilter(t *testing.T) {
	if got := len(chordsIn([]string{"| Am7 | D | G | G (4n) | Am7 | D "})); got != 6 {
		t.Errorf("chords on a line with a (4n) mark = %d, want 6", got)
	}
	if got := len(chordsIn([]string{"Intro:  ||: C  | D | G | Em7 :|| "})); got != 4 {
		t.Errorf("chords on a labelled intro line = %d, want 4", got)
	}
	// "Dad" is spellable as a chord, but the line around it is not a chord
	// line, so alongside real chord lines it contributes nothing. (With
	// nothing else to go on, chordsIn deliberately falls back to every
	// chord-shaped token rather than giving up.)
	mixed := []string{"| G | C | D | G ", "Dad bade a a dead cab", "| G | C | D | G "}
	if got := len(chordsIn(mixed)); got != 8 {
		t.Errorf("chords alongside a lyric line = %d, want 8", got)
	}
}

// Quartz writes most runs as a bare "(..) Tj"; tjText only accepts "[..] TJ".
func TestBlockText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`63 0 0 63 256 792 Tm /TT6 1 Tf (!) Tj`, "!"},
		{`/TT2 1 Tf [ ('&) 1 (\() -3 (\)) ] TJ`, "'&()"},
		{`/TT1 1 Tf <48656C6C6F> Tj`, "Hello"},
		{`/TT1 1 Tf (a) Tj (b) Tj`, "ab"},
		{`/Span <</ActualText(x)>> BDC`, ""},
	} {
		if got := blockText(c.in); got != c.want {
			t.Errorf("blockText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
