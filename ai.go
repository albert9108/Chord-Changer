package main

// Reads chord screenshots with the Claude API and returns chart text in the
// format ParseChart understands. Standard library only.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var aiEndpoint = "https://api.anthropic.com/v1/messages"

const (
	aiDefaultModel = "claude-sonnet-5-5"
	aiMaxImage     = 5 << 20 // API limit per image
	// Sonnet 5.5 thinks by default and thinking counts toward max_tokens,
	// so leave plenty of room for the chart after the thinking.
	aiMaxTokens = 32000
)

const aiSystem = `You turn worship-song chord sources into a chord chart in one exact text format.

INPUT: one or more screenshots of chords (from any website: chords above lyrics, guitar TAB with chord boxes, jianpu/numbered notation with chord names, ChordPro-like text), plus the lyrics the user typed (may be empty, may be Traditional Chinese, may already contain some [chord] marks).

OUTPUT FORMAT (output only this, no explanation, no code fences):
title: <song title>
tempo: <bpm number if shown, else empty>
key: <key of the chord shapes used, e.g. G>
intro: | G | D/F# | Em | C |
## Verse
[G]从不主动示弱
[D/F#]我们的过去 [Em]分分合合太多
## Chorus
...

RULES
1. All Chinese must be Simplified Chinese. Convert any Traditional characters.
2. [X] goes immediately before the syllable where chord X starts. [Em7 D] = two chords inside one bar.
3. ONE CHORD PER BAR. If the source shows bar lines (TAB, jianpu, sheet music), mark each bar once at its first syllable, using the bar's chord box. Never repeat a chord that just continues. If the source has no bar lines and repeats the same chord several times in a row, mark only the first.
4. A bar that starts on a rest/held note with no syllable: put the mark at the end of the previous line after a full-width space, e.g. 求你别离开我　[G]
5. Chord spelling: sharps only, except Bb. Db->C#, Eb->D#, Ab->G#, Gb->F#. Keep slash chords (D/F#), 7, maj7, sus4, m, dim etc. as written.
6. If the source says Capo N, keep the chord shapes as written and use the shape key for "key:".
7. Section names in English on their own line after "## ": Intro is the intro: line; then Verse, Pre-Chorus, Chorus, Bridge, Ending (Verse 2, Chorus 2 when they differ). Write each section once; do not repeat sections that repeat identically. If the source has a rap/interlude part before the bridge, merge it into Bridge.
8. Keep lyric lines short: at most 13 Chinese characters per line (Latin letters count half). Split long phrases at natural pauses. Use one space between phrases on the same line.
9. Put "===" on its own line before the section that should start the second column, choosing it so the two columns are roughly equal in length.
10. Use the lyrics the user typed as the lyric source when given (convert to Simplified); otherwise read them from the screenshots.
11. If something is missing (e.g. a section has no chords in any screenshot), still write the lyrics and add a line in round brackets such as (此段截图没有 chord，请补上).
12. If a section repeats with different chords (for example the last chorus moves up a key), write it again as its own section, e.g. "## Chorus 2 (Key D)", with the chords exactly as shown.`

type aiImage struct {
	MediaType string
	Data      []byte
}

func imageMediaType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}
	return ""
}

func loadAIImage(path string) (aiImage, error) {
	mt := imageMediaType(path)
	if mt == "" {
		return aiImage{}, errors.New("not a PNG, JPG, WEBP or GIF image")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return aiImage{}, err
	}
	if len(b) > aiMaxImage {
		return aiImage{}, fmt.Errorf("%s is larger than 5 MB — crop or resize it", filepath.Base(path))
	}
	return aiImage{mt, b}, nil
}

// ReadChartWithAI sends the screenshots and lyrics to Claude and returns chart text.
func ReadChartWithAI(apiKey, model, lyrics string, imgs []aiImage) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", errors.New("enter your Claude API key first")
	}
	if len(imgs) == 0 {
		return "", errors.New("add at least one chord screenshot")
	}
	if model == "" {
		model = aiDefaultModel
	}
	var content []map[string]any
	for _, im := range imgs {
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": im.MediaType,
				"data":       base64.StdEncoding.EncodeToString(im.Data),
			},
		})
	}
	prompt := "Make the chart from these chord screenshots."
	if strings.TrimSpace(lyrics) != "" {
		prompt += "\n\nLyrics / current chart text from the user:\n" + lyrics
	}
	content = append(content, map[string]any{"type": "text", "text": prompt})

	reqBody, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": aiMaxTokens,
		"system":     aiSystem,
		"messages":   []map[string]any{{"role": "user", "content": content}},
	})
	req, err := http.NewRequest("POST", aiEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", strings.TrimSpace(apiKey))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := (&http.Client{Timeout: 6 * time.Minute}).Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach the Claude API: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("unexpected reply from the Claude API (HTTP %d)", resp.StatusCode)
	}
	if out.Error != nil {
		if resp.StatusCode == 401 {
			return "", errors.New("the API key was rejected — check it in console.anthropic.com")
		}
		return "", fmt.Errorf("Claude API error: %s", out.Error.Message)
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	text = strings.TrimPrefix(text, "```text")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	if text == "" {
		switch out.StopReason {
		case "max_tokens":
			return "", errors.New("the AI ran out of room before writing the chart — try fewer screenshots at a time")
		case "refusal":
			return "", errors.New("the AI declined this request — try again, or crop the screenshots to just the chords and lyrics")
		}
		return "", fmt.Errorf("the AI returned no chart (stop reason: %q) — try again", out.StopReason)
	}
	return text, nil
}

// ---------------------------------------------------------------- settings

type appSettings struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model,omitempty"`
}

func settingsPath() string {
	dir, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Chord Key Changer", "settings.json")
}

func loadSettings() appSettings {
	var s appSettings
	if p := settingsPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			json.Unmarshal(b, &s)
		}
	}
	if s.APIKey == "" {
		s.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	return s
}

func saveSettings(s appSettings) error {
	p := settingsPath()
	if p == "" {
		return errors.New("no settings folder")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, b, 0600)
}
