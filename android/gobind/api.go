//go:build ignore

// Android bridge. The workflow copies this file next to a copy of the engine
// (chart.go, transpose.go, pdf.go, ai.go, dib.go as package "engine") and runs
// gomobile bind, which turns it into chordkey.aar. Java sees the class
// chordkey.Chordkey with static methods, plus Images, Chart and Converted.
// Only gomobile-friendly types (string, []byte, int, error, *struct) here.

package chordkey

import (
	"errors"
	"fmt"
	"strings"

	"chordkeymobile/engine"
)

// Version is set at build time.
var Version = "dev"

// ---------------------------------------------------------------- make chart

// Images holds the chord screenshots for one AI reading.
type Images struct{ list []engine.AIImage }

func NewImages() *Images { return &Images{} }

func (m *Images) Add(mediaType string, data []byte) error {
	if len(data) > engine.AIMaxImage {
		return errors.New("picture is larger than 5 MB")
	}
	m.list = append(m.list, engine.AIImage{MediaType: mediaType, Data: append([]byte(nil), data...)})
	return nil
}

func (m *Images) Count() int { return len(m.list) }
func (m *Images) Clear()     { m.list = nil }

// ReadWithAI sends the screenshots and lyrics to Claude and returns chart text.
func ReadWithAI(apiKey, lyrics string, m *Images) (string, error) {
	if m == nil {
		return "", errors.New("add at least one chord screenshot")
	}
	return engine.ReadChartWithAI(apiKey, "", lyrics, m.list)
}

// Chart is a built Word chart.
type Chart struct {
	Data     []byte
	FileName string
	Warnings string // one per line
}

func BuildChart(text string) (*Chart, error) {
	c := engine.ParseChart(text)
	lyrics := 0
	for _, l := range c.Lines {
		if l.Kind == "lyric" {
			lyrics++
		}
	}
	if lyrics == 0 {
		return nil, errors.New("there are no lyric lines yet — paste the lyrics (and Read with AI) first")
	}
	b, warns, err := engine.BuildChartDocx(c)
	if err != nil {
		return nil, err
	}
	return &Chart{Data: b, FileName: engine.ChartFileName(c), Warnings: strings.Join(warns, "\n")}, nil
}

// ---------------------------------------------------------------- change key

// KeyNames lists the 12 keys in the house spelling, space separated.
func KeyNames() string { return strings.Join(engine.NAMES, " ") }

// DetectKey returns the chart's key, e.g. "G". name must end in .pdf or .docx.
func DetectKey(name string, data []byte) (string, error) {
	k, err := engine.Detect(name, data)
	if err != nil {
		return "", err
	}
	return k.Key, nil
}

// Converted is a transposed chart.
type Converted struct {
	Data     []byte
	FileName string
	Changed  int
	Warnings string // one per line
}

func Convert(name string, data []byte, fromKey, toKey string) (*Converted, error) {
	to, ok := engine.IDX[toKey]
	if !ok {
		return nil, fmt.Errorf("unknown key %q", toKey)
	}
	from := engine.KeyRoot(fromKey)
	if to == from {
		return nil, errors.New("the chart is already in this key — pick a different key")
	}
	b, n, warns, err := engine.Convert(name, data, to-from)
	if err != nil {
		return nil, err
	}
	return &Converted{Data: b, FileName: engine.OutputName(name, toKey), Changed: n,
		Warnings: strings.Join(warns, "\n")}, nil
}
