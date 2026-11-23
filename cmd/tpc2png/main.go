// Command tpc2png converts a TPC texture to an 8-bit RGBA PNG and prints the parser's report line.
//
// usage: go run ./cmd/tpc2png in.tpc out.png
package main

import (
	"fmt"
	"os"

	"re/kotor"
)

func main() {
	if len(os.Args) != 3 {
		kotor.Fatal("usage: tpc2png in.tpc out.png")
	}

	file, err := os.ReadFile(os.Args[1])
	if err != nil {
		kotor.Fatal(err.Error())
	}

	texture := kotor.ParseTpc(file)
	image := kotor.DecodeTpc(file, texture)
	fmt.Println(kotor.ReportTpc(os.Args[1], texture))

	if image != nil {
		if err := os.WriteFile(os.Args[2], kotor.EncodePng(image.Width, image.Height, image.Rgba, 4), 0o644); err != nil {
			kotor.Fatal(err.Error())
		}
	}
}
