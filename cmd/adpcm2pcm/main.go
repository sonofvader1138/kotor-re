// Command adpcm2pcm converts an IMA ADPCM WAV file to 16-bit PCM WAV, or copies another WAV file
// unchanged, and prints the decoder's report line.
//
// usage: go run ./cmd/adpcm2pcm in.wav out.wav
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"re/kotor"
)

func main() {
	if len(os.Args) != 3 {
		kotor.Fatal("usage: adpcm2pcm in.wav out.wav")
	}

	file := kotor.ReadFile(os.Args[1])

	result := kotor.DecodeAdpcm(file)
	if result.Out != nil {
		kotor.WriteFile(os.Args[2], result.Out)
	}

	fmt.Printf("%s %s %s\n", filepath.Base(os.Args[1]), result.Info, kotor.Verdict(result.Err))
}
