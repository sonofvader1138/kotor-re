// Command tpc2png converts a TPC texture, or an Xbox TXB texture when the input name ends in .txb, to an
// 8-bit RGBA PNG and prints the parser's report line.
//
// usage: go run ./cmd/tpc2png in.tpc|in.txb out.png
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"re/kotor"
)

func main() {
	if len(os.Args) != 3 {
		kotor.Fatal("usage: tpc2png in.tpc|in.txb out.png")
	}

	file := kotor.ReadFile(os.Args[1])

	texture := kotor.ParseTpc(file, strings.EqualFold(filepath.Ext(os.Args[1]), ".txb"))
	image := kotor.DecodeTpc(file, texture)
	fmt.Println(kotor.ReportTpc(os.Args[1], texture))

	if image != nil {
		kotor.WriteFile(os.Args[2], kotor.EncodePng(image.Width, image.Height, image.Rgba, 4))
	}
}
