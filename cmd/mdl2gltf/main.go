// Command mdl2gltf converts an MDL model and its MDX vertex data to a glTF file and its binary buffer, which
// is written beside it with the extension .bin, and prints the converter's report line. A missing MDX file
// is treated as empty. Textures are not linked, so the materials carry only their names.
//
// usage: go run ./cmd/mdl2gltf in.mdl in.mdx out.gltf
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"re/kotor"
)

func main() {
	if len(os.Args) != 4 {
		kotor.Fatal("usage: mdl2gltf in.mdl in.mdx out.gltf")
	}

	model := kotor.ReadFile(os.Args[1])
	vertices, _ := os.ReadFile(os.Args[2])
	binPath := strings.TrimSuffix(os.Args[3], filepath.Ext(os.Args[3])) + ".bin"

	result := kotor.ConvertMdl(model, vertices, filepath.Base(os.Args[1]), filepath.Base(binPath), nil, kotor.MdlOptions{})
	kotor.WriteFile(os.Args[3], result.Gltf)
	kotor.WriteFile(binPath, result.Bin)
	fmt.Println(kotor.ReportMdl(os.Args[1], result))
}
