// Command unpack writes <resref>.<ext> for the resources of one archive that match the given resrefs, or
// for all of its resources when none are given. The archive is a BIF archive listed in chitin.key, given
// relative to the game folder, or a RIM, ERF or MOD file, given by any path.
//
// usage: go run ./cmd/unpack data/models.bif outdir [resref...]
//
//	go run ./cmd/unpack swkotor/modules/danm13_s.rim outdir [resref...]
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
		kotor.Fatal("usage: unpack data/models.bif|file.rim|file.erf|file.mod outdir [resref...]")
	}

	archive, out := os.Args[1], os.Args[2]
	wanted := map[string]bool{}
	for _, resref := range os.Args[3:] {
		wanted[strings.ToLower(resref)] = true
	}
	kotor.MakeDir(out)

	var resources []kotor.Resource
	switch strings.ToLower(filepath.Ext(archive)) {
	case ".bif":
		resources = kotor.BifResources(archive, kotor.ReadKey())

	case ".rim":
		rim, problem := kotor.ReadRim(kotor.ReadFile(archive))
		if problem != "" {
			kotor.Fatal(archive + ": " + problem)
		}
		for _, entry := range rim.Entries {
			resources = append(resources, kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data})
		}

	default:
		for _, entry := range kotor.ReadErf(archive) {
			resources = append(resources, kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data()})
		}
	}

	count := 0
	for _, resource := range resources {
		if len(wanted) > 0 && !wanted[resource.Resref] {
			continue
		}

		kotor.WriteFile(filepath.Join(out, resource.Resref+"."+resource.Ext), resource.Data)
		count++
	}

	fmt.Printf("%d resources written to %s\n", count, out)
}
