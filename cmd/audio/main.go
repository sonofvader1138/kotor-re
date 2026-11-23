// Command audio unwraps KotOR stream audio files and prints one report line per file. When the first
// argument is a directory, each passing file's payload is written there as <name>.mp3 or <name>.wav.
//
// usage: go run ./cmd/audio [outdir] file.wav...
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"re/kotor"
)

var wavPattern = regexp.MustCompile(`(?i)\.wav$`)

func main() {
	args := os.Args[1:]
	out := ""
	if len(args) > 0 {
		if info, err := os.Stat(args[0]); err == nil && info.IsDir() {
			out, args = args[0], args[1:]
		}
	}

	for _, file := range args {
		name := wavPattern.ReplaceAllString(filepath.Base(file), "")
		result := kotor.UnwrapAudio(kotor.ReadFile(file))
		fmt.Printf("%-24s %s %-40s %s\n", name, result.Kind, result.Info, kotor.Verdict(result.Err))

		if out != "" && result.Err == "" {
			kotor.WriteFile(filepath.Join(out, name+"."+result.Kind), result.Payload)
		}
	}
}
