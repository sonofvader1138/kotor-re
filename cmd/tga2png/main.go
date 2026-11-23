// Command tga2png converts a TGA image to an 8-bit PNG with the same channels and prints the parser's
// report line.
//
// usage: go run ./cmd/tga2png in.tga out.png
package main

import (
	"fmt"
	"os"

	"re/kotor"
)

func main() {
	if len(os.Args) != 3 {
		kotor.Fatal("usage: tga2png in.tga out.png")
	}

	file := kotor.ReadFile(os.Args[1])

	header := kotor.ParseTga(file)
	image := kotor.DecodeTga(file, header)
	fmt.Println(kotor.ReportTga(os.Args[1], header))

	if image != nil {
		kotor.WriteFile(os.Args[2], kotor.EncodePng(image.Width, image.Height, image.Pix, image.Channels))
	}
}
