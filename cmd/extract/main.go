// Command extract writes <resref>.<ext> for the resources of one BIF archive that match the given resrefs,
// or for all of its resources when none are given.
//
// usage: go run ./cmd/extract data/models.bif outdir [resref...]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"re/kotor"
)

func main() {
	if len(os.Args) < 3 {
		kotor.Fatal("usage: extract data/models.bif outdir [resref...]")
	}

	bif, out := os.Args[1], os.Args[2]
	wanted := map[string]bool{}
	for _, resref := range os.Args[3:] {
		wanted[strings.ToLower(resref)] = true
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		kotor.Fatal(err.Error())
	}

	count := 0
	for _, resource := range kotor.BifResources(bif, kotor.ReadKey()) {
		if len(wanted) > 0 && !wanted[resource.Resref] {
			continue
		}

		if err := os.WriteFile(filepath.Join(out, resource.Resref+"."+resource.Ext), resource.Data, 0o644); err != nil {
			kotor.Fatal(err.Error())
		}
		count++
	}

	fmt.Printf("%d resources written to %s\n", count, out)
}
