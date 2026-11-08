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
