// Command mdl2gltf converts an MDL model and its MDX vertex data to a glTF file and its binary buffer, which
// is written beside it with the extension .bin, and prints the converter's report line. A missing MDX file
// is treated as empty. Textures are not linked, so the materials carry only their names. The flag -xbox
// reads the Xbox form of the model.
//
// usage: go run ./cmd/mdl2gltf [-xbox] in.mdl in.mdx out.gltf
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"re/kotor"
)

func main() {
	xbox := flag.Bool("xbox", false, "read the Xbox form of the model")
	flag.Parse()
	if flag.NArg() != 3 {
		kotor.Fatal("usage: mdl2gltf [-xbox] in.mdl in.mdx out.gltf")
	}

	model := kotor.ReadFile(flag.Arg(0))
	vertices, _ := os.ReadFile(flag.Arg(1))
	binPath := strings.TrimSuffix(flag.Arg(2), filepath.Ext(flag.Arg(2))) + ".bin"

	result := kotor.ConvertMdl(model, vertices, filepath.Base(flag.Arg(0)), filepath.Base(binPath), nil, kotor.MdlOptions{Xbox: *xbox})
	kotor.WriteFile(flag.Arg(2), result.Gltf)
	kotor.WriteFile(binPath, result.Bin)
	fmt.Println(kotor.ReportMdl(flag.Arg(0), result))
}
