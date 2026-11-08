//go:build !windows

package main

import (
	"fmt"
	"os"
)

// Test harness: chordkey IN OUT TARGETKEY
func main() {
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
