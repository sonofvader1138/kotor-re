// Command unxiso finds the Xbox disc image (.iso) in the dump folder and expands it into dump/xbox with the
// extract-xiso tool. It refuses to run when dump/xbox already exists, so a finished extraction is never
// overwritten.
//
// usage: go run ./cmd/unxiso
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"re/kotor"
)

const extractXiso = "extract-xiso"

func main() {
	dumpDir := filepath.Join(kotor.Root, "dump")
	out := filepath.Join(dumpDir, "xbox")

	images := kotor.ListFiles(dumpDir, ".iso", false)
	if len(images) != 1 {
		kotor.Fatal(fmt.Sprintf("expected one .iso file in %s, found %d: %s", dumpDir, len(images), strings.Join(images, ", ")))
	}
	image := filepath.Join(dumpDir, images[0])

	if !kotor.Has(extractXiso) {
		kotor.Fatal(extractXiso + " is not on the search path")
	}
	if _, err := os.Stat(out); err == nil {
		kotor.Fatal(out + " already exists; remove it to extract again")
	}

	// extract-xiso can report a read error and still exit with status 0, so any output on standard error
	// also counts as a failure. A failed run leaves a partial folder, which is removed. The folder did not
	// exist before this run, so nothing else is lost.
	result := kotor.Exec(extractXiso, []string{"-q", "-x", "-d", out, image}, nil)
	if result.Status != 0 || strings.TrimSpace(result.Stderr) != "" {
		message := fmt.Sprintf("%s failed with status %d: %s", extractXiso, result.Status, strings.TrimSpace(result.Stderr))
		if err := os.RemoveAll(out); err != nil {
			message += "\nthe partial folder could not be removed: " + err.Error()
		}
		kotor.Fatal(message)
	}

	count := 0
	err := filepath.WalkDir(out, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			count++
		}
		return err
	})
	if err != nil {
		kotor.Fatal(err.Error())
	}

	fmt.Printf("%s expanded into %s\n", images[0], out)
	fmt.Printf("%d files written\n", count)
}
