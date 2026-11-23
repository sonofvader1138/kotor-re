// Command mdl2obj converts an MDL model and its MDX vertex data to a Wavefront OBJ file and prints the
// converter's report line. A missing MDX file is treated as empty.
//
// usage: go run ./cmd/mdl2obj in.mdl in.mdx out.obj
package main

import (
	"fmt"
	"os"

	"re/kotor"
)

func main() {
	if len(os.Args) != 4 {
		kotor.Fatal("usage: mdl2obj in.mdl in.mdx out.obj")
	}

	model, err := os.ReadFile(os.Args[1])
	if err != nil {
		kotor.Fatal(err.Error())
	}
	vertices, _ := os.ReadFile(os.Args[2])

	result := kotor.ConvertMdl(model, vertices, os.Args[1], "")
	if err := os.WriteFile(os.Args[3], []byte(result.Obj), 0o644); err != nil {
		kotor.Fatal(err.Error())
	}
	fmt.Println(kotor.ReportMdl(os.Args[1], result))
}
