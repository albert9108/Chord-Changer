//go:build !windows

package main

import (
	"fmt"
	"os"
	"strings"
)

// Test harness:
//
//	chordkey IN OUT TARGETKEY                    transpose a chart
//	chordkey chart IN.txt OUT.docx               build a chart from chart text
//	chordkey ai OUT.txt [lyrics.txt] img...      read screenshots with Claude (needs ANTHROPIC_API_KEY)
func main() {
	if len(os.Args) > 1 && os.Args[1] == "chart" {
		src, err := os.ReadFile(os.Args[2])
		if err != nil {
			panic(err)
		}
		b, warns, err := BuildChartDocx(ParseChart(string(src)))
		if err != nil {
			panic(err)
		}
		os.WriteFile(os.Args[3], b, 0644)
		fmt.Println("saved", os.Args[3], "warnings:", warns)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "ai" {
		lyrics := ""
		var imgs []aiImage
		for _, p := range os.Args[3:] {
			if strings.HasSuffix(strings.ToLower(p), ".txt") {
				b, _ := os.ReadFile(p)
				lyrics = string(b)
				continue
			}
			im, err := loadAIImage(p)
			if err != nil {
				panic(err)
			}
			imgs = append(imgs, im)
		}
		s := loadSettings()
		text, err := ReadChartWithAI(s.APIKey, s.Model, lyrics, imgs)
		if err != nil {
			fmt.Println("ai error:", err)
			os.Exit(1)
		}
		os.WriteFile(os.Args[2], []byte(text), 0644)
		fmt.Println(text)
		return
	}

	in, out, target := os.Args[1], os.Args[2], os.Args[3]
	data, err := os.ReadFile(in)
	if err != nil {
		panic(err)
	}
	k, err := Detect(in, data)
	if err != nil {
		fmt.Println("detect error:", err)
		os.Exit(1)
	}
	fmt.Printf("Detected: %s (%s)\n", k.Key, k.Source)
	n := IDX[target] - KeyRoot(k.Key)
	b, c, w, err := Convert(in, data, n)
	if err != nil {
		fmt.Println("convert error:", err)
		os.Exit(1)
	}
	os.WriteFile(out, b, 0644)
	fmt.Println("changed runs:", c, "warnings:", w, "suggested name:", OutputName(in, target))
}
