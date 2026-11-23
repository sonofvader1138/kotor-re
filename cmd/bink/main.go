// Command bink parses Bink video files and prints one report line per file, ending in PASS or FAIL.
//
// usage: go run ./cmd/bink file.bik...
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"re/kotor"
)

func main() {
	for _, file := range os.Args[1:] {
		data, err := os.ReadFile(file)
		if err != nil {
			kotor.Fatal(err.Error())
		}
		fmt.Println(kotor.ReportBink(filepath.Base(file), kotor.ParseBink(data)))
	}
}
