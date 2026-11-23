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

	model := kotor.ReadFile(os.Args[1])
	vertices, _ := os.ReadFile(os.Args[2])

	result := kotor.ConvertMdl(model, vertices, os.Args[1], "")
	kotor.WriteFile(os.Args[3], []byte(result.Obj))
	fmt.Println(kotor.ReportMdl(os.Args[1], result))
}
