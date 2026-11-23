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

	file := kotor.ReadFile(os.Args[1])

	texture := kotor.ParseTpc(file)
	image := kotor.DecodeTpc(file, texture)
	fmt.Println(kotor.ReportTpc(os.Args[1], texture))

	if image != nil {
		kotor.WriteFile(os.Args[2], kotor.EncodePng(image.Width, image.Height, image.Rgba, 4))
	}
}
