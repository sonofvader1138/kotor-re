// Command verify checks the converted assets in ~/re/extracted with independent tools and writes one report
// per check to ~/re/temp/reports. Run the extract command first. A stage whose tool is missing is reported
// as UNVERIFIED. All stages run at the same time, and their checks share jobSlots job slots, so that at
// most that many external programs run at once. Each report lists its lines in job order.
//
// models: each model and walkmesh glTF file is checked in four ways. assimp must load it with at least one
// mesh or animation, with the face count that its index accessors give, and with its counts of animations,
// animated nodes and bones (see assimpGltf). The Khronos glTF validator, when installed, must report no
// errors. jq must read the mesh header fields and the light, emitter, reference, dangly, AABB and static
// controller values of the node extras, which must equal the source model (see checkModelExtras and
// checkNodeExtras), or the walkmesh values of the node extras, which must build again into the source's header
// values, edge table and perimeter table (see checkWalkmeshExtras). The binary buffer, located through the
// accessors that jq reads, must hold the source's vertex positions, face indices and face fields (see
// checkModelBin and checkWalkmeshBin). Each model glTF file is also checked in two more ways: its animations
// must equal the controllers of the source's animation trees (see checkAnimations), and its skins the bone
// maps, bone indices and bone weights of the source (see checkSkins).
// textures: ImageMagick's decoding of each PNG must equal the decoded TPC texture, and pngcheck must accept it.
// tga: ImageMagick's decoding of each PNG must equal its decoding of the source TGA, and pngcheck must
// accept the PNG. Each TXI must equal the source bytes. Each room visibility list, read with jq, must build
// again into its source text in canonical form (see checkVisJson).
// music, sounds, waves: ffprobe must read each file without error. The samples are not compared.
// movies: ffprobe must report the Bink file's size, frame count, sample rate, channel count and audio
// duration (within 0.1 s; ffmpeg's Bink audio decoder emits about 0.03 s more than the packets declare),
// and every output frame must have a PSNR of at least 40 dB against the decoded Bink frame.
// gui: jq must parse each JSON file.
// patch: the resources of patch.erf, checked as addResourceChecks describes.
// lips: Go's encoding/csv reader must read each CSV file, and its keyframes, packed again as f32 and u8, must
// equal the source LIP keyframe bytes.
// modules: jq must parse each JSON file. No independent tool reads NCS, so each script listing is
// assembled again from its text and must equal the source script byte for byte. The assembler shares only
// the opcode, type and operand tables with the disassembler.
// rims: each model is checked as for models. Every other resource is checked as addResourceChecks describes: WAV files with ffprobe and ffmpeg, 2DA CSV files with encoding/csv, SSF and LTR JSON files rebuilt from
// jq's output, other JSON files with jq, NSS files with iconv, and script listings by assembling them.
// bifs: the resources of the BIF archives in kotor.ResourceBifs, checked as addResourceChecks describes.
// tlk: the string table JSON, read with jq, must build again into dialog.tlk byte for byte.
// saves: the GFF files and archive resources of each saved game, checked as addResourceChecks describes;
// the screenshot PNG against ImageMagick and pngcheck; and the decoded global variables, read with jq,
// built again into the value fields of GLOBALVARS.res.
//
// usage: go run ./cmd/verify [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui] [patch] [lips] [modules] [rims] [bifs] [tlk] [saves]   (default: all)
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"re/kotor"
)

// Two jobs per CPU keep every CPU busy while some jobs wait on their programs.
const jobsPerCpu = 2

var jobSlots = make(chan struct{}, runtime.NumCPU()*jobsPerCpu)

// A job returns its report lines, usually one.
type job func() []string

func line(text string) []string {
	return []string{text}
}

// pool runs jobs in the shared slots and returns their lines in job order.
func pool(jobs []job) []string {
	results := make([][]string, len(jobs))
	var group sync.WaitGroup
	for i, run := range jobs {
		group.Go(func() {
			jobSlots <- struct{}{}
			results[i] = run()
			<-jobSlots
		})
	}
	group.Wait()
	return slices.Concat(results...)
}

var logMutex sync.Mutex

func log(message string) {
	logMutex.Lock()
	fmt.Println(message)
	logMutex.Unlock()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type decodedImage struct {
	err  string
	size string
	rgba []byte
}

// magickDecode decodes an image with ImageMagick into 8-bit RGBA, top row first. The size goes to stderr.
func magickDecode(format string, data []byte) decodedImage {
	result := kotor.Exec("convert", []string{
		format + ":-", "-auto-orient", "-format", "%w %h", "-write", "info:fd:2", "-depth", "8", "rgba:-",
	}, data)
	if result.Status != 0 {
		return decodedImage{err: kotor.FirstLine(result.Stderr)}
	}

	lines := strings.Split(strings.TrimSpace(result.Stderr), "\n")
	return decodedImage{size: lines[len(lines)-1], rgba: result.Stdout}
}

// pngCheck returns a report line that ends in PASS when the PNG decodes to the expected size and pixels.
func pngCheck(name, file, width, height string, rgba []byte) []string {
	if !exists(file) {
		return line(name + " FAIL: output missing")
	}

	decoded := magickDecode("png", kotor.ReadFile(file))
	if decoded.err != "" {
		return line(name + " FAIL: " + decoded.err)
	}
	if decoded.size != width+" "+height {
		return line(fmt.Sprintf("%s FAIL: size %s, expected %s %s", name, decoded.size, width, height))
	}
	if !bytes.Equal(decoded.rgba, rgba) {
		return line(name + " " + decoded.size + " FAIL: pixels differ")
	}
	return line(name + " " + decoded.size + " PASS")
}

// pngcheckDir runs pngcheck on every PNG in dir, pngcheckBatch files per run.
const pngcheckBatch = 200

var pngcheckOk = regexp.MustCompile(`^OK: (.*?) \(`)

func pngcheckDir(dir string) []string {
	names := kotor.ListFiles(dir, ".png", false)
	var jobs []job
	for chunk := range slices.Chunk(names, pngcheckBatch) {
		jobs = append(jobs, func() []string {
			paths := make([]string, len(chunk))
			for i, file := range chunk {
				paths[i] = filepath.Join(dir, file)
			}
			result := kotor.Exec("pngcheck", paths, nil)

			ok := map[string]bool{}
			for text := range strings.SplitSeq(string(result.Stdout), "\n") {
				if match := pngcheckOk.FindStringSubmatch(text); match != nil {
					ok[filepath.Base(match[1])] = true
				}
			}

			lines := make([]string, len(chunk))
			for i, file := range chunk {
				lines[i] = file + " FAIL: pngcheck"
				if ok[file] {
					lines[i] = file + " PASS"
				}
			}
			return lines
		})
	}
	return pool(jobs)
}

var (
	magick   = kotor.Has("convert")
	pngcheck = kotor.Has("pngcheck")
	ffprobe  = kotor.Has("ffprobe")
)

// imageChecks runs the pixel and structure checks of one directory at the same time.
func imageChecks(summary *kotor.Summary, name, dir string, jobs []job) {
	var pixelLines, structureLines []string
	var group sync.WaitGroup
	if magick {
		group.Go(func() { pixelLines = pool(jobs) })
	}
	if pngcheck {
		group.Go(func() { structureLines = pngcheckDir(dir) })
	}
	group.Wait()

	if magick {
		summary.Tally("imagemagick_"+name, pixelLines)
	} else {
		summary.Note("ImageMagick missing: " + name + " UNVERIFIED")
	}

	if pngcheck {
		summary.Tally("pngcheck_"+name, structureLines)
	} else {
		summary.Note("pngcheck missing: " + name + " PNG structure unchecked")
	}
}

var (
	meshesPattern     = regexp.MustCompile(`(?m)^Meshes:\s+(\d+)`)
	facesPattern      = regexp.MustCompile(`(?m)^Faces:\s+(\d+)`)
	animationsPattern = regexp.MustCompile(`(?m)^Animations:\s+(\d+)`)
	channelsPattern   = regexp.MustCompile(`(?m)^Animation Channels:\s+(\d+)`)
	bonesPattern      = regexp.MustCompile(`(?m)^Bones:\s+(\d+)`)

	assimpErrorPattern = regexp.MustCompile(`(?m)^ERROR: (.*)$`)
)

// gltfLocation is a jq function that prints the byte offset of an accessor in the binary buffer and its
// element count. Its input is the accessor index, and $d is the document.
const gltfLocation = `def location($d): $d.accessors[.] as $a
	| "\(($d.bufferViews[$a.bufferView].byteOffset // 0) + ($a.byteOffset // 0)) \($a.count)";`

// assimpGltf loads one glTF file with assimp and returns its report line. The import is raw (-r), because
// assimp's default post-processing merges meshes with identical geometry, and some models repeat a mesh at
// the same place. The expected face count is the sum of the index counts of all primitives divided by 3, as
// jq reads it.
func assimpGltf(path, label string) []string {
	counted := kotor.Exec("jq", []string{"-r", `. as $d | [.meshes[]?.primitives[].indices | $d.accessors[.].count] | add // 0`, path}, nil)
	indices, err := strconv.Atoi(strings.TrimSpace(string(counted.Stdout)))
	if counted.Status != 0 || err != nil {
		return line(label + " FAIL: jq: " + kotor.FirstLine(counted.Stderr))
	}
	faces := indices / 3

	result := kotor.Exec("assimp", []string{"info", path, "-r"}, nil)
	meshes := meshesPattern.FindSubmatch(result.Stdout)
	loaded := facesPattern.FindSubmatch(result.Stdout)
	if result.Status != 0 || meshes == nil || loaded == nil {
		message := kotor.FirstLine(result.Stderr)
		if found := assimpErrorPattern.FindSubmatch(result.Stdout); found != nil {
			message = string(found[1])
		}
		return line(label + " FAIL: assimp: " + message)
	}

	// assimp merges the channels of one node in one animation, so its channel count is the number of
	// animated nodes summed over the animations.
	var animations, nodes, bones int
	counted = kotor.Exec("jq", []string{"-r", `(.animations // [] | length), ([.animations[]? | [.channels[].target.node] | unique | length] | add // 0), ([.skins[]?.joints | length] | add // 0)`, path}, nil)
	if _, err := fmt.Sscan(string(counted.Stdout), &animations, &nodes, &bones); counted.Status != 0 || err != nil {
		return line(label + " FAIL: jq: " + kotor.FirstLine(counted.Stderr))
	}

	var errs []string
	if count, _ := strconv.Atoi(string(meshes[1])); count == 0 && animations == 0 {
		errs = append(errs, "no meshes and no animations")
	}
	if count, _ := strconv.Atoi(string(loaded[1])); count != faces {
		errs = append(errs, fmt.Sprintf("assimp loaded %s faces, accessors give %d", loaded[1], faces))
	}
	for _, check := range []struct {
		pattern *regexp.Regexp
		what    string
		want    int
	}{{animationsPattern, "animations", animations}, {channelsPattern, "animation channels", nodes}, {bonesPattern, "bones", bones}} {
		found := check.pattern.FindSubmatch(result.Stdout)
		if found == nil {
			errs = append(errs, "assimp printed no "+check.what)
			continue
		}
		if count, _ := strconv.Atoi(string(found[1])); count != check.want {
			errs = append(errs, fmt.Sprintf("assimp loaded %d %s, glTF has %d", count, check.what, check.want))
		}
	}
	return line(fmt.Sprintf("%s meshes=%s faces=%s animations=%d bones=%d %s", label, meshes[1], loaded[1], animations,
		bones, kotor.Verdict(errs...)))
}

// validateGltf runs the Khronos glTF validator on one glTF file, which also loads its buffer and images, and
// returns an error message when the report counts errors.
func validateGltf(path string) string {
	result := kotor.Exec("gltf_validator", []string{"-o", path}, nil)
	var report struct {
		Issues struct {
			NumErrors int `json:"numErrors"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(result.Stdout, &report); err != nil {
		return "gltf_validator: " + kotor.FirstLine(result.Stderr)
	}
	if report.Issues.NumErrors > 0 {
		return fmt.Sprintf("gltf_validator reports %d errors", report.Issues.NumErrors)
	}
	return ""
}

// addGltfChecks adds the checks that every glTF file gets: the assimp import and the validator.
func addGltfChecks(jobs map[string][]job, label, path string) {
	jobs["assimp"] = append(jobs["assimp"], func() []string { return assimpGltf(path, label) })
	jobs["validator"] = append(jobs["validator"], func() []string { return line(label + " " + kotor.Verdict(validateGltf(path))) })
}

// Models: extracted/data/<archive>/*.gltf for each archive in kotor.ModelBifs, models and walkmeshes. Each
// written glTF file is checked against its source resource.
func models(summary *kotor.Summary) {
	log("models...")
	data := filepath.Join(kotor.Extracted, "data")
	key := kotor.ReadKey()
	modelJobs := map[string][]job{}
	walkmeshJobs := map[string][]job{}
	for _, name := range kotor.ModelBifs {
		resources := kotor.BifResources("data/"+name+".bif", key)
		vertexData := map[string][]byte{}
		for _, resource := range resources {
			if resource.Ext == "mdx" {
				vertexData[resource.Resref] = resource.Data
			}
		}
		for _, resource := range resources {
			if resource.Ext == "mdl" {
				file := filepath.Join(name, resource.Resref)
				addModelChecks(modelJobs, file, filepath.Join(data, file), resource.Data, vertexData[resource.Resref])
				continue
			}
			if resource.Ext != "wok" && resource.Ext != "dwk" && resource.Ext != "pwk" {
				continue
			}

			file := filepath.Join(name, resource.Resref+"."+resource.Ext)
			path := filepath.Join(data, file)
			if !exists(path + ".gltf") {
				continue
			}
			addGltfChecks(walkmeshJobs, file+".gltf", path+".gltf")
			walkmeshJobs["jq"] = append(walkmeshJobs["jq"], func() []string {
				return line(file + ".gltf " + kotor.Verdict(checkWalkmeshExtras(path+".gltf", resource.Data)))
			})
			walkmeshJobs["gltf"] = append(walkmeshJobs["gltf"], func() []string {
				return line(file + ".bin " + kotor.Verdict(checkWalkmeshBin(path, resource.Data)))
			})
		}
	}

	runResourceChecks(summary, "models", modelJobs)
	log("walkmeshes...")
	runResourceChecks(summary, "walkmeshes", walkmeshJobs)
}

// walkmeshProgram prints the numbers of the mesh node extras of a walkmesh glTF file, one per line, in source
// order: the type, the two relative and the two absolute use points, the position, then for each perimeter
// its edges as 3 * face + edge and transition (-1 for null), followed by the perimeter's end in the edge
// list. Node 1 is the mesh node, below the root node that turns Z-up into Y-up.
const walkmeshProgram = `.nodes[1].extras | .type, (.relative_use + .absolute_use | .[][]), .position[],
	(foreach .perimeters[] as $perimeter (0; . + ($perimeter | length);
		($perimeter[] | (3 * .face + .edge), (.transition // -1)), "end \(.)"))`

// checkWalkmeshExtras builds the header bytes 0x08 to 0x47, the edge table and the perimeter table of a
// walkmesh from the glTF extras that jq reads, and compares them with the source. The header offsets of the
// two tables are read from the source.
func checkWalkmeshExtras(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", walkmeshProgram, path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}
	fields := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")

	const headerFloats = 15
	if len(fields) < 1+headerFloats {
		return fmt.Sprintf("jq printed %d values", len(fields))
	}
	var header, edges, perimeters []byte
	typeValue, err := strconv.ParseUint(fields[0], 10, 32)
	if err != nil {
		return "bad type " + fields[0]
	}
	header = binary.LittleEndian.AppendUint32(header, uint32(typeValue))
	for _, field := range fields[1 : 1+headerFloats] {
		value, err := strconv.ParseFloat(field, 32)
		if err != nil {
			return "bad number " + field
		}
		header = binary.LittleEndian.AppendUint32(header, math.Float32bits(float32(value)))
	}

	rest := fields[1+headerFloats:]
	for i := 0; i < len(rest); i++ {
		if end, ok := strings.CutPrefix(rest[i], "end "); ok {
			value, _ := strconv.ParseUint(end, 10, 32)
			perimeters = binary.LittleEndian.AppendUint32(perimeters, uint32(value))
			continue
		}
		if i+1 >= len(rest) {
			return "edge without transition"
		}
		edge, edgeErr := strconv.ParseInt(rest[i], 10, 32)
		transition, transitionErr := strconv.ParseInt(rest[i+1], 10, 32)
		if edgeErr != nil || transitionErr != nil {
			return "bad edge " + rest[i] + " " + rest[i+1]
		}
		edges = binary.LittleEndian.AppendUint32(edges, uint32(int32(edge)))
		edges = binary.LittleEndian.AppendUint32(edges, uint32(int32(transition)))
		i++
	}

	const (
		headerStart     = 0x08
		edgeCount       = 0x78
		edgeOffset      = 0x7C
		perimeterCount  = 0x80
		perimeterOffset = 0x84
		edgeSize        = 8
		perimeterSize   = 4
	)
	table := func(countOffset, dataOffset, size int) []byte {
		count := int(binary.LittleEndian.Uint32(source[countOffset:]))
		start := int(binary.LittleEndian.Uint32(source[dataOffset:]))
		if count == 0 {
			return nil
		}
		return kotor.Slice(source, start, start+count*size)
	}

	switch {
	case !bytes.Equal(header, source[headerStart:headerStart+len(header)]):
		return "header differs from the source"
	case !bytes.Equal(edges, table(edgeCount, edgeOffset, edgeSize)):
		return fmt.Sprintf("built %d edge bytes differ from the source", len(edges))
	case !bytes.Equal(perimeters, table(perimeterCount, perimeterOffset, perimeterSize)):
		return fmt.Sprintf("built %d perimeter bytes differ from the source", len(perimeters))
	}
	return ""
}

// walkmeshBinProgram prints the URI of the binary buffer, the location of the positions and, for each
// primitive, its material and the location of its indices.
const walkmeshBinProgram = gltfLocation + ` . as $d | .buffers[0].uri,
	(.meshes[0].primitives[0].attributes.POSITION | location($d)),
	(.meshes[0].primitives[] | "\(.material) \(.indices | location($d))")`

// checkWalkmeshBin compares the binary buffer of a walkmesh glTF file with the source's vertex, face and
// material tables. path is the converted file without the extensions .gltf and .bin. The positions must
// equal the vertex table byte for byte. The index accessors of the primitives, in order, must cover the
// buffer's face indices without gaps, and those must equal the face table. The material of each primitive,
// repeated once per face, must build again into the material table.
func checkWalkmeshBin(path string, source []byte) string {
	const (
		vertexCount    = 0x48
		vertexOffset   = 0x4C
		faceCount      = 0x50
		faceOffset     = 0x54
		materialOffset = 0x58
		vertexSize     = 12
		faceSize       = 12
		indexSize      = 4
	)
	result := kotor.Exec("jq", []string{"-r", walkmeshBinProgram, path + ".gltf"}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}
	fields := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
	if len(fields) < 3 || fields[0] != filepath.Base(path)+".bin" {
		return "unexpected buffer or no primitives"
	}
	bin, err := os.ReadFile(path + ".bin")
	if err != nil {
		return err.Error()
	}

	u32 := func(offset int) int {
		return readUint(source, offset, 4)
	}
	table := func(offsetField, count, size int) []byte {
		start := u32(offsetField)
		return kotor.Slice(source, start, start+count*size)
	}

	var start, count int
	if _, err := fmt.Sscan(fields[1], &start, &count); err != nil {
		return "bad position location " + fields[1]
	}
	if count != u32(vertexCount) || !bytes.Equal(kotor.Slice(bin, start, start+vertexSize*count), table(vertexOffset, count, vertexSize)) {
		return "positions differ from the vertex table"
	}

	var indices, materials []byte
	for _, field := range fields[2:] {
		var material, offset, length int
		if _, err := fmt.Sscan(field, &material, &offset, &length); err != nil {
			return "bad primitive " + field
		}
		indices = append(indices, kotor.Slice(bin, offset, offset+indexSize*length)...)
		for range length / 3 {
			materials = binary.LittleEndian.AppendUint32(materials, uint32(material))
		}
	}

	faces := u32(faceCount)
	switch {
	case !bytes.Equal(indices, table(faceOffset, faces, faceSize)):
		return "face indices differ from the face table"
	case !bytes.Equal(materials, table(materialOffset, faces, indexSize)):
		return "primitive materials differ from the material table"
	}
	return ""
}

// readUint reads a little-endian unsigned integer of size bytes at offset, or returns 0 when it does not lie
// inside data.
func readUint(data []byte, offset, size int) int {
	if offset < 0 || offset+size > len(data) {
		return 0
	}

	value := 0
	for i := size - 1; i >= 0; i-- {
		value = value<<8 | int(data[offset+i])
	}
	return value
}

// modelMesh is one mesh node of an MDL file: its name, its node flags and the offset of its mesh header in the
// model data, which starts 12 bytes into the file.
type modelMesh struct {
	name   string
	flags  int
	header int
}

// modelMeshes walks the node tree of an MDL file depth first, children in stored order, and returns its mesh
// nodes and the model data. Reads outside the data yield 0, and each node is visited once.
func modelMeshes(mdl []byte) ([]modelMesh, []byte) {
	const (
		fileHeader = 12
		rootNode   = 0x28
		nameTable  = 0xb8
		nameCount  = 0xc0
		childArray = 0x2c
		childCount = 0x30
		nameIndex  = 4
		meshFlag   = 0x20
		nodeSize   = 80
	)
	data := kotor.Slice(mdl, fileHeader, len(mdl))
	u32 := func(offset int) int {
		return readUint(data, offset, 4)
	}
	u16 := func(offset int) int {
		return readUint(data, offset, 2)
	}

	var meshes []modelMesh
	seen := map[int]bool{}
	var walk func(offset int)
	walk = func(offset int) {
		if seen[offset] {
			return
		}
		seen[offset] = true

		flags := u16(offset)
		if flags&meshFlag != 0 {
			name := ""
			if index := u16(offset + nameIndex); index < u32(nameCount) {
				start := u32(u32(nameTable) + 4*index)
				name = kotor.CString(kotor.Slice(data, start, len(data)))
			}
			meshes = append(meshes, modelMesh{name: name, flags: flags, header: offset + nodeSize})
		}

		for i := range u32(offset + childCount) {
			walk(u32(u32(offset+childArray) + 4*i))
		}
	}
	walk(u32(rootNode))
	return meshes, data
}

// modelProgram prints the mesh header fields of the node extras of a model glTF file one per line, mesh by
// mesh in node order. A mesh without texture scrolling prints 0 in place of the scroll values, and a mesh with
// it prints 1 followed by them.
const modelProgram = `.nodes[] | .extras.mesh // empty | .name, .kind, .render, .shadow, .lightmapped, .texture, .lightmap, .diffuse[],
	.ambient[], .transparency_hint,
	(if .uv_scroll then 1, .uv_scroll.direction[], .uv_scroll.jitter, .uv_scroll.jitter_speed else 0 end),
	.bounds_min[], .bounds_max[], .radius, .average_point[], .surface_area, .vertex_count, .face_count,
	(.vertex_components | join(" ")), "end"`

// checkModelExtras compares the mesh header fields of a model glTF file, as jq prints them, with the fields
// of the source MDL. Floats are compared bit for bit after rounding to f32, and names up to their first NUL byte.
// The mesh kind is compared with the one that the node flags give.
func checkModelExtras(path string, mdl []byte) string {
	fields, problem := jqLines(modelProgram, path)
	if problem != "" {
		return problem
	}

	meshes, data := modelMeshes(mdl)
	field := 0
	next := func() string {
		if field >= len(fields) {
			field++
			return ""
		}
		field++
		return fields[field-1]
	}
	u32 := func(offset int) uint32 {
		return uint32(readUint(data, offset, 4))
	}
	components := map[string]uint32{"position": 0x01, "uv1": 0x02, "uv2": 0x04, "normal": 0x20, "tangent_space": 0x80}
	kinds := []struct {
		flag int
		name string
	}{{0x40, "skin"}, {0x100, "dangly"}, {0x200, "aabb"}, {0x800, "lightsaber"}}

	for _, mesh := range meshes {
		header := mesh.header
		var problems []string
		expect := func(what, got, want string) {
			if got != want {
				problems = append(problems, fmt.Sprintf("%s %q, source %q", what, got, want))
			}
		}
		boolean := func(what string, offset int) {
			expect(what, next(), strconv.FormatBool(data[min(header+offset, len(data)-1)] != 0))
		}
		number := func(what string, offset int) {
			expect(what, next(), strconv.FormatUint(uint64(u32(header+offset)), 10))
		}
		floats := func(what string, offset, count int) {
			for i := range count {
				text := next()
				value, err := strconv.ParseFloat(text, 32)
				if err != nil || math.Float32bits(float32(value)) != u32(header+offset+4*i) {
					problems = append(problems, fmt.Sprintf("%s[%d] %q", what, i, text))
				}
			}
		}
		name := func(what string, offset int) {
			expect(what, next(), kotor.CString(kotor.Slice(data, header+offset, header+offset+32)))
		}

		kind := "trimesh"
		for _, candidate := range kinds {
			if mesh.flags&candidate.flag != 0 {
				kind = candidate.name
				break
			}
		}

		expect("name", next(), mesh.name)
		expect("kind", next(), kind)
		boolean("render", 0x139)
		boolean("shadow", 0x137)
		boolean("lightmapped", 0x134)
		name("texture", 0x58)
		name("lightmap", 0x78)
		floats("diffuse", 0x3c, 3)
		floats("ambient", 0x48, 3)
		number("transparency hint", 0x54)
		number("scroll flag", 0xe8)
		if u32(header+0xe8) != 0 {
			floats("scroll", 0xec, 4)
		}
		floats("bounds", 0x14, 6)
		floats("radius", 0x2c, 1)
		floats("average point", 0x30, 3)
		floats("surface area", 0x13c, 1)
		expect("vertex count", next(), strconv.Itoa(int(u32(header+0x130)&0xffff)))
		number("face count", 0x0c)

		flags := uint32(0)
		for component := range strings.FieldsSeq(next()) {
			flags |= components[component]
		}
		if flags != u32(header+0x100) {
			problems = append(problems, fmt.Sprintf("vertex flags %#x, source %#x", flags, u32(header+0x100)))
		}
		expect("end of mesh", next(), "end")

		if len(problems) > 0 {
			return mesh.name + ": " + strings.Join(problems, "; ")
		}
	}

	if field != len(fields) {
		return fmt.Sprintf("jq printed %d lines for %d meshes, %d expected", len(fields), len(meshes), field)
	}
	return ""
}

// modelBinProgram prints the URI of the binary buffer and, for each node with a mesh, its name and the
// locations of its positions, its indices, its face materials and its adjacent faces. A skinned mesh lies on
// the node that extras.skin names.
const modelBinProgram = gltfLocation + ` . as $d | .buffers[0].uri, (.nodes[] | select(.extras.mesh) |
	(.mesh // (if .extras.skin then $d.nodes[.extras.skin.node].mesh else null end)) as $m | select($m != null) |
	.name, ($d.meshes[$m].primitives[0] as $p
		| $p.attributes.POSITION, $p.indices, .extras.face_materials, .extras.face_adjacent | location($d)))`

// checkModelBin compares the binary buffer of a model glTF file with the source MDL. path is the converted
// file without the extensions .gltf and .bin. Every mesh whose render byte is set and that has faces must
// have a node with a mesh, in source order. Its positions must equal the MDL vertex array byte for byte, its
// u32 indices the u16 vertex indices of the faces, its face materials the u32 at 0x10 of each face and its
// adjacent faces the u16[3] at 0x14, byte for byte.
func checkModelBin(path string, mdl []byte) string {
	const (
		faceSize      = 32
		materialField = 0x10
		adjacentField = 0x14
		cornersField  = 0x1a
		renderOffset  = 0x139
		facesOffset   = 0x08
		countOffset   = 0x0c
		vertexCount   = 0x130
		vertexOffset  = 0x148
		vertexSize    = 12
		corners       = 3
		fieldsPerMesh = 5
		materialSize  = 4
		adjacentSize  = 6
		indexSize     = 4
	)
	result := kotor.Exec("jq", []string{"-r", modelBinProgram, path + ".gltf"}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}
	fields := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
	if fields[0] != filepath.Base(path)+".bin" {
		return "unexpected buffer " + fields[0]
	}
	bin, err := os.ReadFile(path + ".bin")
	if err != nil {
		return err.Error()
	}

	meshes, data := modelMeshes(mdl)
	u32 := func(offset int) int {
		return readUint(data, offset, 4)
	}

	field := 1
	for _, mesh := range meshes {
		faceCount := u32(mesh.header + countOffset)
		if data[min(mesh.header+renderOffset, len(data)-1)] == 0 || faceCount == 0 {
			continue
		}
		if field+fieldsPerMesh > len(fields) {
			return "no glTF mesh for " + mesh.name
		}
		if fields[field] != mesh.name {
			return fmt.Sprintf("glTF mesh %s, source %s", fields[field], mesh.name)
		}

		var starts, counts [4]int
		for k := range starts {
			if _, err := fmt.Sscan(fields[field+1+k], &starts[k], &counts[k]); err != nil {
				return "bad location " + fields[field+1+k]
			}
		}
		field += fieldsPerMesh

		vertices := u32(mesh.header+vertexCount) & 0xffff
		vertexStart := u32(mesh.header + vertexOffset)
		if counts[0] != vertices || !bytes.Equal(kotor.Slice(bin, starts[0], starts[0]+vertexSize*vertices),
			kotor.Slice(data, vertexStart, vertexStart+vertexSize*vertices)) {
			return mesh.name + ": positions differ from the source"
		}

		var indices, materials, adjacent []byte
		faces := u32(mesh.header + facesOffset)
		for i := range faceCount {
			face := faces + faceSize*i
			for corner := range corners {
				indices = binary.LittleEndian.AppendUint32(indices, uint32(readUint(data, face+cornersField+2*corner, 2)))
			}
			materials = append(materials, kotor.Slice(data, face+materialField, face+materialField+materialSize)...)
			adjacent = append(adjacent, kotor.Slice(data, face+adjacentField, face+adjacentField+adjacentSize)...)
		}

		switch {
		case counts[1] != corners*faceCount || !bytes.Equal(kotor.Slice(bin, starts[1], starts[1]+len(indices)), indices):
			return mesh.name + ": indices differ from the source"
		case counts[2] != faceCount || !bytes.Equal(kotor.Slice(bin, starts[2], starts[2]+len(materials)), materials):
			return mesh.name + ": face materials differ from the source"
		case counts[3] != faceCount || !bytes.Equal(kotor.Slice(bin, starts[3], starts[3]+len(adjacent)), adjacent):
			return mesh.name + ": adjacent faces differ from the source"
		}
	}

	if field != len(fields) {
		return fmt.Sprintf("%d glTF meshes, %d expected", (len(fields)-1)/fieldsPerMesh, (field-1)/fieldsPerMesh)
	}
	return ""
}

// addModelChecks adds the checks of one converted model to jobs, when the model was written: the glTF checks
// of addGltfChecks, the mesh header extras and the binary buffer. path is the converted file without
// extensions, and label is the same path relative to the report folder.
func addModelChecks(jobs map[string][]job, label, path string, mdl, mdx []byte) {
	if !exists(path + ".gltf") {
		return
	}
	addGltfChecks(jobs, label+".gltf", path+".gltf")
	jobs["jq"] = append(jobs["jq"], func() []string {
		return line(label + ".gltf " + kotor.Verdict(checkModelExtras(path+".gltf", mdl), checkNodeExtras(path, mdl)))
	})
	jobs["animation"] = append(jobs["animation"], func() []string {
		return line(label + ".gltf " + kotor.Verdict(checkAnimations(path, mdl)))
	})
	jobs["skin"] = append(jobs["skin"], func() []string {
		return line(label + ".gltf " + kotor.Verdict(checkSkins(path, mdl, mdx)))
	})
	jobs["gltf"] = append(jobs["gltf"], func() []string {
		return line(label + ".bin " + kotor.Verdict(checkModelBin(path, mdl)))
	})
}

// Textures: extracted/TexturePacks/<pack>/*.png against the TPC textures in TexturePacks/<pack>.erf.
// Each job decodes its TPC texture when it starts, so only one texture per job slot is held in memory.
func textures(summary *kotor.Summary) {
	for _, pack := range kotor.TexturePacks {
		log("textures " + pack + "...")
		dir := filepath.Join(kotor.Extracted, "TexturePacks", pack)
		entries := kotor.ReadErf(filepath.Join(kotor.Game, "TexturePacks", pack+".erf"))

		var jobs []job
		for _, entry := range entries {
			if entry.Ext != "tpc" {
				continue
			}

			jobs = append(jobs, func() []string {
				data := entry.Data()
				image := kotor.DecodeTpc(data, kotor.ParseTpc(data))
				name := entry.Resref + ".png"
				if image == nil {
					return line(name + " FAIL: source texture does not decode")
				}

				return pngCheck(name, filepath.Join(dir, name), strconv.Itoa(image.Width), strconv.Itoa(image.Height), image.Rgba)
			})
		}
		imageChecks(summary, pack, dir, jobs)
	}
}

// TGA images: extracted/data/<group>/*.png and *.txi against the BIF archives they came from
func tgaImages(summary *kotor.Summary) {
	key := kotor.ReadKey()
	for _, group := range kotor.TgaGroups {
		log("tga " + group.Name + "...")
		dir := filepath.Join(kotor.Extracted, "data", group.Name)

		var txiJobs, visJobs, jobs []job
		seen := map[string]bool{}
		for _, bif := range key.Bifs {
			if !group.Pattern.MatchString(bif) {
				continue
			}

			for _, resource := range kotor.BifResources(bif, key) {
				if resource.Ext == "vis" {
					name := resource.Resref + ".vis.json"
					visJobs = append(visJobs, func() []string {
						return line(name + " " + kotor.Verdict(checkVisJson(filepath.Join(dir, name), resource.Data)))
					})
					continue
				}
				if resource.Ext != "tga" && resource.Ext != "txi" {
					continue
				}

				name := resource.Resref + ".txi"
				if resource.Ext == "tga" {
					name = resource.Resref + ".png"
				}
				file := filepath.Join(dir, name)

				// extract keeps the first archive's copy of a name and reports the others.
				if seen[name] {
					continue
				}
				seen[name] = true

				if resource.Ext == "txi" {
					txiJobs = append(txiJobs, func() []string {
						if !exists(file) {
							return line(name + " FAIL: output missing")
						}
						if !bytes.Equal(kotor.ReadFile(file), resource.Data) {
							return line(name + " FAIL: differs from source")
						}
						return line(name + " PASS")
					})
					continue
				}

				jobs = append(jobs, func() []string {
					source := magickDecode("tga", resource.Data)
					if source.err != "" {
						return line(name + " FAIL: source TGA: " + source.err)
					}

					width, height, _ := strings.Cut(source.size, " ")
					return pngCheck(name, file, width, height, source.rgba)
				})
			}
		}

		summary.Tally("txi_"+group.Name, pool(txiJobs))
		if len(visJobs) > 0 {
			if kotor.Has("jq") {
				summary.Tally("jq_vis_"+group.Name, pool(visJobs))
			} else {
				summary.Note("jq missing: vis_" + group.Name + " UNVERIFIED")
			}
		}
		imageChecks(summary, group.Name, dir, jobs)
	}
}

// Stream audio: extracted/streammusic, extracted/streamsounds and extracted/streamwaves
func streamAudio(summary *kotor.Summary, stage, folder string) {
	if !ffprobe {
		summary.Note("ffprobe missing: " + stage + " UNVERIFIED")
		return
	}

	log(stage + "...")
	dir := filepath.Join(kotor.Extracted, folder)
	audioFiles := append(kotor.ListFiles(dir, ".mp3", true), kotor.ListFiles(dir, ".wav", true)...)
	slices.Sort(audioFiles)

	jobs := make([]job, len(audioFiles))
	for i, file := range audioFiles {
		jobs[i] = func() []string {
			result := kotor.Exec("ffprobe", []string{"-v", "error", filepath.Join(dir, file)}, nil)
			if result.Status == 0 && strings.TrimSpace(result.Stderr) == "" {
				return line(file + " PASS")
			}
			return line(file + " FAIL: " + kotor.FirstLine(result.Stderr))
		}
	}
	summary.Tally("ffprobe_"+stage, pool(jobs))
}

// Movies: extracted/movies/*.mp4 against movies/*.bik. The probe and the PSNR comparison of one movie run
// at the same time.
const minPsnr = 40

// ffmpeg's Bink audio decoder emits about 0.03 s more than the packets declare.
const audioDurationToleranceS = 0.1

// Bink audio is 16-bit.
const binkBytesPerSample = 2

// probeStream holds the ffprobe fields that the movie check reads. A missing field stays nil.
type probeStream struct {
	CodecType    string  `json:"codec_type"`
	CodecName    string  `json:"codec_name"`
	Width        *int    `json:"width"`
	Height       *int    `json:"height"`
	NbReadFrames *string `json:"nb_read_frames"`
	SampleRate   *string `json:"sample_rate"`
	Channels     *int    `json:"channels"`
	Duration     *string `json:"duration"`
}

// optional formats a missing ffprobe field as "?".
func optional[T any](value *T) string {
	if value == nil {
		return "?"
	}
	return fmt.Sprint(*value)
}

func optionalNumber(value *string) float64 {
	if value == nil {
		return math.NaN()
	}
	number, err := strconv.ParseFloat(*value, 64)
	if err != nil {
		return math.NaN()
	}
	return number
}

var (
	psnrPattern = regexp.MustCompile(`PSNR [^\n\r]*average:(\S+) min:(\S+)`)
)

func movies(summary *kotor.Summary) {
	if !ffprobe || !kotor.Has("ffmpeg") {
		summary.Note("ffmpeg or ffprobe missing: movies UNVERIFIED")
		return
	}

	log("movies...")
	src := filepath.Join(kotor.Game, "movies")
	dir := filepath.Join(kotor.Extracted, "movies")
	var jobs []job
	for _, file := range kotor.ListFiles(src, ".bik", false) {
		jobs = append(jobs, func() []string {
			name := strings.TrimSuffix(file, filepath.Ext(file))
			inFile := filepath.Join(src, file)
			outFile := filepath.Join(dir, name+".mp4")
			movie := kotor.ParseBink(kotor.ReadFile(inFile))
			if len(movie.Errors) > 0 {
				return line(name + ".mp4 FAIL: source does not parse")
			}
			if !exists(outFile) {
				return line(name + ".mp4 FAIL: output missing")
			}

			header := movie.Header
			var probe, compare kotor.ExecResult
			var group sync.WaitGroup
			group.Go(func() {
				probe = kotor.Exec("ffprobe", []string{
					"-v", "error", "-count_frames", "-of", "json", "-show_entries",
					"stream=codec_type,codec_name,width,height,nb_read_frames,sample_rate,channels,duration",
					outFile,
				}, nil)
			})
			group.Go(func() {
				compare = kotor.Exec("ffmpeg", []string{
					"-hide_banner", "-nostats", "-i", outFile, "-i", inFile,
					"-lavfi", "[0:v][1:v]psnr", "-f", "null", "-",
				}, nil)
			})
			group.Wait()

			var parsed struct {
				Streams []probeStream `json:"streams"`
			}
			if probe.Status == 0 && json.Unmarshal(probe.Stdout, &parsed) != nil {
				return line(name + ".mp4 FAIL: ffprobe output is not JSON")
			}
			var video, sound *probeStream
			for i := range parsed.Streams {
				stream := &parsed.Streams[i]
				if stream.CodecType == "video" && video == nil {
					video = stream
				}
				if stream.CodecType == "audio" && sound == nil {
					sound = stream
				}
			}

			var errs []string
			if video == nil || video.CodecName != "h264" {
				errs = append(errs, "no H.264 video stream")
			} else {
				if video.Width == nil || video.Height == nil || *video.Width != header.Width || *video.Height != header.Height {
					errs = append(errs, fmt.Sprintf("size %sx%s, Bink %dx%d",
						optional(video.Width), optional(video.Height), header.Width, header.Height))
				}
				if optionalNumber(video.NbReadFrames) != float64(header.Frames) {
					errs = append(errs, fmt.Sprintf("%s frames, Bink %d", optional(video.NbReadFrames), header.Frames))
				}
			}

			audioInfo := "no audio"
			if header.AudioTracks == 0 && sound != nil {
				errs = append(errs, "unexpected audio stream")
			}
			if header.AudioTracks > 0 {
				track := header.Tracks[0]
				channels := 1
				if track.Flags&kotor.BinkTrackStereo != 0 {
					channels = 2
				}
				durationS := float64(movie.DecodedBytes[0]) / float64(binkBytesPerSample*channels*track.SampleRate)
				if sound == nil || sound.CodecName != "aac" {
					errs = append(errs, "no AAC audio stream")
				} else {
					if optionalNumber(sound.SampleRate) != float64(track.SampleRate) {
						errs = append(errs, fmt.Sprintf("%s Hz, Bink %d Hz", optional(sound.SampleRate), track.SampleRate))
					}
					if sound.Channels == nil || *sound.Channels != channels {
						errs = append(errs, fmt.Sprintf("%s channels, Bink %d", optional(sound.Channels), channels))
					}
					if math.Abs(optionalNumber(sound.Duration)-durationS) > audioDurationToleranceS {
						errs = append(errs, fmt.Sprintf("audio %s s, Bink %.3f s", optional(sound.Duration), durationS))
					}
					audioInfo = fmt.Sprintf("%dHz %dch %.3fs", track.SampleRate, channels, durationS)
				}
			}

			average, minimum := "?", "?"
			measured := "not measured"
			psnrMin := math.NaN()
			if psnr := psnrPattern.FindStringSubmatch(compare.Stderr); psnr != nil {
				average, minimum, measured = psnr[1], psnr[2], psnr[2]
				if value, err := strconv.ParseFloat(psnr[2], 64); err == nil {
					psnrMin = value
				}
			}
			if !(psnrMin >= minPsnr) {
				errs = append(errs, "minimum PSNR "+measured)
			}

			return line(fmt.Sprintf("%s.mp4 %dx%d %d frames %s PSNR avg %s min %s %s",
				name, header.Width, header.Height, header.Frames, audioInfo, average, minimum, kotor.Verdict(errs...)))
		})
	}
	summary.Tally("ffprobe_movies", pool(jobs))
}

// GUI: extracted/data/gui/*.json. The GUI layouts of patch.erf are checked by the patch stage.
func gui(summary *kotor.Summary) {
	if !kotor.Has("jq") {
		summary.Note("jq missing: gui UNVERIFIED")
		return
	}

	log("gui...")
	var jobs []job
	dir := "data/gui"
	for _, file := range kotor.ListFiles(filepath.Join(kotor.Extracted, dir), ".json", false) {
		jobs = append(jobs, func() []string {
			result := kotor.Exec("jq", []string{"empty", filepath.Join(kotor.Extracted, dir, file)}, nil)
			if result.Status == 0 {
				return line(dir + "/" + file + " PASS")
			}
			return line(dir + "/" + file + " FAIL: jq: " + kotor.FirstLine(result.Stderr))
		})
	}
	summary.Tally("jq_gui", pool(jobs))
}

// Lips: extracted/lips/<module>/*.csv. Go's encoding/csv reader must read each file, and its keyframes, packed
// again as f32 and u8, must equal the keyframe bytes of the source LIP resource. This reader shares no
// code with the converter's writer, but it is not an external tool.
func lips(summary *kotor.Summary) {
	log("lips...")
	src := filepath.Join(kotor.Game, "lips")
	var report []string
	for _, archive := range kotor.ListFiles(src, ".mod", false) {
		module := strings.TrimSuffix(archive, filepath.Ext(archive))
		for _, entry := range kotor.ReadErf(filepath.Join(src, archive)) {
			label := filepath.Join("lips", module, entry.Resref+".csv")
			report = append(report, label+" "+kotor.Verdict(checkLipCsv(filepath.Join(kotor.Extracted, label), entry.Data())))
		}
	}
	summary.Tally("csv_lips", report)
}

// checkLipCsv reads a converted LIP file and returns an error message when its keyframes differ from the
// source resource, or an empty string when they match.
func checkLipCsv(path string, source []byte) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}

	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return err.Error()
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != kotor.LipCsvHeader {
		return "bad header"
	}

	packed := make([]byte, 0, len(source))
	for _, row := range rows[1:] {
		time, timeErr := strconv.ParseFloat(row[0], 32)
		shape, shapeErr := strconv.ParseUint(row[1], 10, 8)
		if timeErr != nil || shapeErr != nil {
			return "bad value in " + strings.Join(row, ",")
		}

		packed = binary.LittleEndian.AppendUint32(packed, math.Float32bits(float32(time)))
		packed = append(packed, byte(shape))
	}

	if !bytes.Equal(packed, source[kotor.LipHeaderSize:]) {
		return "keyframes differ from the source"
	}
	return ""
}

// Modules: extracted/modules/<archive>/*.json must parse with jq, and every <resref>.ncs.txt listing must
// assemble to the bytes of the script in the source archive.
func modules(summary *kotor.Summary) {
	log("modules...")
	src := filepath.Join(kotor.Game, "modules")
	var jsonJobs []job
	var report []string
	for _, archive := range kotor.ListFiles(src, ".rim", false) {
		name := strings.TrimSuffix(archive, filepath.Ext(archive))
		rim, problem := kotor.ReadRim(kotor.ReadFile(filepath.Join(src, archive)))
		if problem != "" {
			report = append(report, archive+" FAIL: "+problem)
			continue
		}

		for _, entry := range rim.Entries {
			if entry.Ext == "ncs" {
				label := filepath.Join("modules", name, entry.Resref+".ncs.txt")
				report = append(report, label+" "+kotor.Verdict(checkNcsListing(filepath.Join(kotor.Extracted, label), entry.Data)))
				continue
			}

			label := filepath.Join("modules", name, entry.Resref+"."+entry.Ext+".json")
			jsonJobs = append(jsonJobs, func() []string {
				result := kotor.Exec("jq", []string{"empty", filepath.Join(kotor.Extracted, label)}, nil)
				if result.Status == 0 {
					return line(label + " PASS")
				}
				return line(label + " FAIL: jq: " + kotor.FirstLine(result.Stderr))
			})
		}
	}
	summary.Tally("ncs_modules", report)

	if !kotor.Has("jq") {
		summary.Note("jq missing: module JSON UNVERIFIED")
		return
	}
	summary.Tally("jq_modules", pool(jsonJobs))
}

// checkTools names the tool that each kind of resource check runs, in report order. A check without a
// tool runs in Go and builds the source resource again from the converted file.
var checkTools = []struct{ kind, program string }{
	{"assimp", "assimp"}, {"validator", "gltf_validator"}, {"gltf", "jq"}, {"ffprobe", "ffprobe"}, {"csv", ""},
	{"ncs", ""}, {"jq", "jq"}, {"iconv", "iconv"}, {"copy", ""}, {"imagemagick", "convert"},
	{"pngcheck", "pngcheck"}, {"globals", "jq"}, {"animation", "jq"}, {"skin", "jq"},
}

// addResourceChecks adds the check of one converted resource to jobs, keyed by the kind of check. path is
// the converted file without the extensions that writeResource adds, and label is the same path relative
// to the folder of the report.
//
//   - A WAV file must be read by ffprobe as 16-bit PCM, and its samples must match the source (see
//     compareSamples).
//   - A 2DA CSV file, read with Go's encoding/csv reader, must build again into the source table.
//   - A script listing must assemble to the source script.
//   - The string numbers of an SSF JSON file, read with jq, must build again into the source sound set.
//   - A letter table JSON file, read with jq, must build again into the source letter table.
//   - A room layout JSON file, read with jq, must build again into the source layout (see checkLytJson).
//   - A script source, converted back to Windows-1252 by iconv, must equal the source bytes.
//   - A model glTF file must load with assimp and pass the glTF validator, its node extras must hold the mesh
//     header fields of the source model, and its binary buffer must hold the source's positions, face
//     indices and face fields (see addModelChecks).
//   - A TXI file must equal the source bytes.
//   - A texture PNG, decoded by ImageMagick, must equal the decoded TPC texture, and pngcheck must accept it.
//     Its TXI file, written when the TPC trailer holds text, must equal the trailer without trailing NUL
//     bytes.
//   - A GUI layout JSON file, which has no resource extension in its name, must parse with jq.
//   - Every other JSON file must parse with jq.
func addResourceChecks(jobs map[string][]job, label, path, ext string, source []byte) {
	add := func(kind, file string, check func(path string) string) {
		jobs[kind] = append(jobs[kind], func() []string {
			return line(label + file + " " + kotor.Verdict(check(path+file)))
		})
	}

	switch ext {
	case "mdl":
		addModelChecks(jobs, label, path, source, nil)

	case "mdx":

	case "wav":
		add("ffprobe", ".wav", func(path string) string {
			result := kotor.Exec("ffprobe", []string{"-v", "error", "-show_entries", "stream=codec_name",
				"-of", "csv=p=0", path}, nil)
			codec := strings.TrimSpace(string(result.Stdout))
			if result.Status != 0 || strings.TrimSpace(result.Stderr) != "" {
				return kotor.FirstLine(result.Stderr)
			}
			if codec != pcmCodec {
				return "codec " + codec
			}

			converted, err := os.ReadFile(path)
			if err != nil {
				return err.Error()
			}
			want, problem := ffmpegSamples(source)
			got, otherProblem := ffmpegSamples(converted)
			if problem != "" || otherProblem != "" {
				return problem + otherProblem
			}
			return compareSamples(want, got, isImaAdpcm(source))
		})

	case "2da":
		add("csv", ".2da.csv", func(path string) string { return checkTwoDaCsv(path, source) })

	case "ncs":
		add("ncs", ".ncs.txt", func(path string) string { return checkNcsListing(path, source) })

	case "ssf":
		add("jq", ".ssf.json", func(path string) string { return checkSsfJson(path, source) })

	case "ltr":
		add("jq", ".ltr.json", func(path string) string { return checkLtrJson(path, source) })

	case "lyt":
		add("jq", ".lyt.json", func(path string) string { return checkLytJson(path, source) })

	case "nss":
		add("iconv", ".nss", func(path string) string {
			result := kotor.Exec("iconv", []string{"-f", "UTF-8", "-t", "WINDOWS-1252", path}, nil)
			if result.Status != 0 {
				return "iconv: " + kotor.FirstLine(result.Stderr)
			}
			if !bytes.Equal(result.Stdout, source) {
				return fmt.Sprintf("%d bytes differ from the %d source bytes", len(result.Stdout), len(source))
			}
			return ""
		})

	case "txi":
		add("copy", ".txi", func(path string) string {
			data, err := os.ReadFile(path)
			if err != nil {
				return err.Error()
			}
			if !bytes.Equal(data, source) {
				return "differs from the source"
			}
			return ""
		})

	case "tpc":
		texture := kotor.ParseTpc(source)
		add("imagemagick", ".png", func(path string) string {
			image := kotor.DecodeTpc(source, texture)
			if image == nil {
				return "source texture does not decode"
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err.Error()
			}

			decoded := magickDecode("png", data)
			if decoded.err != "" {
				return decoded.err
			}
			if want := fmt.Sprintf("%d %d", image.Width, image.Height); decoded.size != want {
				return "size " + decoded.size + ", expected " + want
			}
			if !bytes.Equal(decoded.rgba, image.Rgba) {
				return "pixels differ"
			}
			return ""
		})
		add("pngcheck", ".png", func(path string) string {
			result := kotor.Exec("pngcheck", []string{path}, nil)
			if result.Status != 0 || !strings.HasPrefix(string(result.Stdout), "OK: ") {
				return "pngcheck: " + kotor.FirstLine(string(result.Stdout))
			}
			return ""
		})

		txi := string(texture.Txi)
		if strings.TrimSpace(strings.ReplaceAll(txi, "\x00", "")) != "" {
			add("copy", ".txi", func(path string) string {
				data, err := os.ReadFile(path)
				if err != nil {
					return err.Error()
				}
				if string(data) != strings.TrimRight(txi, "\x00") {
					return "differs from the TXI trailer"
				}
				return ""
			})
		}

	case "gui":
		add("jq", ".json", func(path string) string {
			result := kotor.Exec("jq", []string{"empty", path}, nil)
			if result.Status != 0 {
				return "jq: " + kotor.FirstLine(result.Stderr)
			}
			return ""
		})

	default:
		add("jq", "."+ext+".json", func(path string) string {
			result := kotor.Exec("jq", []string{"empty", path}, nil)
			if result.Status != 0 {
				return "jq: " + kotor.FirstLine(result.Stderr)
			}
			return ""
		})
	}
}

// ffmpegSamples decodes a WAV file with ffmpeg into raw 16-bit little-endian samples.
func ffmpegSamples(wav []byte) ([]byte, string) {
	result := kotor.Exec("ffmpeg", []string{"-v", "error", "-i", "-", "-f", "s16le", "-acodec", pcmCodec, "-"}, wav)
	if result.Status != 0 || strings.TrimSpace(result.Stderr) != "" {
		return nil, "ffmpeg: " + kotor.FirstLine(result.Stderr)
	}
	return result.Stdout, ""
}

// WAV header offsets of the first chunk id and of the format tag, when the fmt chunk comes first, and the
// IMA ADPCM format tag.
const (
	wavFirstChunkOffset = 12
	wavFormatTagOffset  = 20
	wavFormatImaAdpcm   = 0x11
)

func isImaAdpcm(wav []byte) bool {
	return len(wav) > wavFormatTagOffset+2 &&
		string(wav[wavFirstChunkOffset:wavFirstChunkOffset+4]) == "fmt " &&
		binary.LittleEndian.Uint16(wav[wavFormatTagOffset:]) == wavFormatImaAdpcm
}

// adpcmToleranceSamples is the largest difference allowed between a sample decoded by kotor.DecodeAdpcm
// and the same sample decoded by ffmpeg. kotor.DecodeAdpcm adds shifted step sizes, as the IMA reference
// decoder does, and ffmpeg multiplies exactly, so the decoded values drift apart within a block by up to
// 86 in the game files. The first sample of every block is stored in the block header and agrees exactly.
const adpcmToleranceSamples = 128

// compareSamples compares the 16-bit samples of a converted WAV file with those of its source, both decoded
// by ffmpeg. PCM sources must match exactly. For an IMA ADPCM source, the converted file holds the sample
// count of the fact chunk, which can be fewer than ffmpeg decodes from the whole last block, so the
// converted samples must match the start of the source samples within adpcmToleranceSamples.
func compareSamples(want, got []byte, adpcm bool) string {
	if !adpcm {
		if !bytes.Equal(want, got) {
			return fmt.Sprintf("%d sample bytes differ from the %d of the source", len(got), len(want))
		}
		return ""
	}

	if len(got) > len(want) {
		return fmt.Sprintf("%d sample bytes, more than the %d of the source", len(got), len(want))
	}
	largest := 0
	for i := 0; i+1 < len(got); i += 2 {
		difference := int(int16(binary.LittleEndian.Uint16(got[i:]))) - int(int16(binary.LittleEndian.Uint16(want[i:])))
		largest = max(largest, difference, -difference)
	}
	if largest > adpcmToleranceSamples {
		return fmt.Sprintf("sample difference %d above %d", largest, adpcmToleranceSamples)
	}
	return ""
}

// runResourceChecks runs the jobs of each kind of check and tallies them in the report <kind>_<stage>. A
// kind whose tool is missing is reported as UNVERIFIED. Kinds without jobs write no report.
func runResourceChecks(summary *kotor.Summary, stage string, jobs map[string][]job) {
	for _, tool := range checkTools {
		if len(jobs[tool.kind]) == 0 {
			continue
		}

		report := tool.kind + "_" + stage
		if tool.program != "" && !kotor.Has(tool.program) {
			summary.Note(tool.program + " missing: " + report + " UNVERIFIED")
			continue
		}
		summary.Tally(report, pool(jobs[tool.kind]))
	}
}

// Rims: extracted/rims/<archive>/ against rims/<archive>.rim. Every resource is checked as addResourceChecks
// describes.
func rims(summary *kotor.Summary) {
	log("rims...")
	src := filepath.Join(kotor.Game, "rims")
	out := filepath.Join(kotor.Extracted, "rims")
	jobs := map[string][]job{}
	var archiveReport []string
	for _, archive := range kotor.ListFiles(src, ".rim", false) {
		name := strings.TrimSuffix(archive, filepath.Ext(archive))
		rim, problem := kotor.ReadRim(kotor.ReadFile(filepath.Join(src, archive)))
		if problem != "" {
			archiveReport = append(archiveReport, archive+" FAIL: "+problem)
			continue
		}

		for _, entry := range rim.Entries {
			label := filepath.Join(name, entry.Resref)
			addResourceChecks(jobs, label, filepath.Join(out, label), entry.Ext, entry.Data)
		}
	}

	if len(archiveReport) > 0 {
		summary.Tally("archives_rims", archiveReport)
	}
	runResourceChecks(summary, "rims", jobs)
}

// Patch: extracted/patch/ against patch.erf. Every resource is checked as addResourceChecks describes.
func patch(summary *kotor.Summary) {
	log("patch...")
	dir := filepath.Join(kotor.Extracted, "patch")
	jobs := map[string][]job{}
	for _, entry := range kotor.ReadErf(filepath.Join(kotor.Game, "patch.erf")) {
		addResourceChecks(jobs, entry.Resref, filepath.Join(dir, entry.Resref), entry.Ext, entry.Data())
	}
	runResourceChecks(summary, "patch", jobs)
}

// Bifs: extracted/data/<archive>/ against data/<archive>.bif for each archive in kotor.ResourceBifs. Each
// resource is checked as addResourceChecks describes.
func bifs(summary *kotor.Summary) {
	log("bifs...")
	key := kotor.ReadKey()
	jobs := map[string][]job{}
	for _, name := range kotor.ResourceBifs {
		for _, resource := range kotor.BifResources("data/"+name+".bif", key) {
			label := filepath.Join(name, resource.Resref)
			addResourceChecks(jobs, label, filepath.Join(kotor.Extracted, "data", label), resource.Ext, resource.Data)
		}
	}
	runResourceChecks(summary, "bifs", jobs)
}

// checkLtrJson reads the values of a converted letter table with jq, in the order letters, single, each
// double set and each triple set, and builds the letter table again: the signature, the letter count and
// one f32 per value. It returns an error message when the result differs from the source, or an empty
// string when it matches.
func checkLtrJson(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", `.letters, (.single, .double[], .triple[] | .start[], .middle[], .end[])`, path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}

	fields := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
	built := append([]byte("LTR V1.0"), byte(len(fields[0])))
	for _, text := range fields[1:] {
		value, err := strconv.ParseFloat(text, 32)
		if err != nil {
			return "bad value " + text
		}
		built = binary.LittleEndian.AppendUint32(built, math.Float32bits(float32(value)))
	}

	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
}

// checkVisJson builds the text of a room visibility list from the JSON that jq reads and compares it with
// the source in canonical form. The canonical form has CRLF after every line, one space between a room name
// and its count, and no spaces after a name; the converted JSON keeps nothing else.
func checkVisJson(path string, source []byte) string {
	program := `.rooms[] | (.room + if .count == null then "" else " " + (.count | tostring) end), "  " + .visible[]`
	result := kotor.Exec("jq", []string{"-r", program, path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}
	built := strings.ReplaceAll(string(result.Stdout), "\n", "\r\n")

	var canonical strings.Builder
	for _, text := range strings.Split(strings.TrimSuffix(string(source), "\r\n"), "\r\n") {
		text = strings.TrimRight(text, " ")
		if !strings.HasPrefix(text, " ") {
			text = strings.Join(strings.Fields(text), " ")
		}
		canonical.WriteString(text + "\r\n")
	}

	if built != canonical.String() {
		return fmt.Sprintf("built %d bytes differ from the %d canonical source bytes", len(built), canonical.Len())
	}
	return ""
}

// lytProgram prints the lines of a room layout from its JSON: the header, each section's count line and
// entries, and the closing line, followed by the trailer as a JSON string.
const lytProgram = `"#MAXLAYOUT ASCII", "filedependancy " + .filedependancy, "beginlayout",
	((["rooms", "roomcount"], ["tracks", "trackcount"], ["obstacles", "obstaclecount"]) as [$key, $keyword]
		| "   \($keyword) \(.[$key] | length)",
		(.[$key][] | "      " + ([.model] + (.position | map(tostring)) | join(" ")))),
	"   doorhookcount \(.doorhooks | length)",
	(.doorhooks[] | "      " + ([.room, .door, (.unknown | tostring)] + (.position + .orientation | map(tostring)) | join(" "))),
	"donelayout", (.trailer | tojson)`

// checkLytJson builds the lines of a room layout from the JSON that jq reads and compares them with the
// lines of the source up to its closing line, field by field: names as text and numbers by value, because
// jq writes numbers in its own form, such as 1.26441e-07 for 1.26441e-007. White space is not compared,
// because the game reads the fields with scan formats that skip it. The text after the closing line must
// equal the trailer.
func checkLytJson(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", lytProgram, path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}
	built := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")

	var trailer string
	if err := json.Unmarshal([]byte(built[len(built)-1]), &trailer); err != nil {
		return "bad trailer: " + err.Error()
	}
	built = built[:len(built)-1]

	const closing = "\r\ndonelayout"
	text := string(source)
	end := strings.Index(text, closing)
	if end < 0 {
		return "source has no closing line"
	}

	rest := strings.TrimPrefix(text[end+len(closing):], "\r\n")
	if rest != trailer {
		return fmt.Sprintf("trailer %q differs from the source %q", trailer, rest)
	}

	lines := append(strings.Split(text[:end], "\r\n"), "donelayout")
	if len(lines) != len(built) {
		return fmt.Sprintf("built %d lines, the source has %d", len(built), len(lines))
	}
	for i := range lines {
		want, got := strings.Fields(lines[i]), strings.Fields(built[i])
		same := len(want) == len(got)
		for j := 0; same && j < len(want); j++ {
			wantValue, wantErr := strconv.ParseFloat(want[j], 64)
			gotValue, gotErr := strconv.ParseFloat(got[j], 64)
			same = want[j] == got[j] || (wantErr == nil && gotErr == nil && wantValue == gotValue)
		}
		if !same {
			return fmt.Sprintf("line %d: built %q, the source has %q", i+1, built[i], lines[i])
		}
	}
	return ""
}

// pcmCodec is ffprobe's name for 16-bit little-endian PCM.
const pcmCodec = "pcm_s16le"

// Binary 2DA layout, rebuilt by checkTwoDaCsv independently of the parser in kotor.
const (
	twoDaSignature    = "2DA V2.b\n"
	twoDaSeparatorTab = '\t'
	twoDaSeparatorNul = 0
)

// checkTwoDaCsv reads a converted 2DA table and builds the binary table again: the column labels, the row
// count, the row labels, one u16 offset per cell, the u16 size of the string data, and the string data,
// where each distinct string is stored once in the order of its first cell. The separator after each label
// is the one the source uses after its first column label. It returns an error message when the result
// differs from the source, or an empty string when it matches.
func checkTwoDaCsv(path string, source []byte) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}

	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return err.Error()
	}
	if len(rows) == 0 || len(rows[0]) < 2 || rows[0][0] != "" {
		return "bad header"
	}

	columns := rows[0][1:]
	separatorAt := len(twoDaSignature) + len(columns[0])
	if separatorAt >= len(source) {
		return "source too short"
	}
	separator := source[separatorAt]
	if separator != twoDaSeparatorTab && separator != twoDaSeparatorNul {
		return fmt.Sprintf("source separator %d", separator)
	}

	latin1 := func(text string) ([]byte, bool) {
		encoded := []byte{}
		for _, char := range text {
			if char > 0xff {
				return nil, false
			}
			encoded = append(encoded, byte(char))
		}
		return encoded, true
	}

	built := []byte(twoDaSignature)
	for _, label := range columns {
		encoded, ok := latin1(label)
		if !ok {
			return "column label outside Latin-1: " + label
		}
		built = append(append(built, encoded...), separator)
	}
	built = append(built, twoDaSeparatorNul)
	built = binary.LittleEndian.AppendUint32(built, uint32(len(rows)-1))

	var cells []string
	for _, row := range rows[1:] {
		encoded, ok := latin1(row[0])
		if !ok {
			return "row label outside Latin-1: " + row[0]
		}
		built = append(append(built, encoded...), separator)
		cells = append(cells, row[1:]...)
	}

	offsets := map[string]int{}
	var stringData []byte
	for _, cell := range cells {
		if _, seen := offsets[cell]; seen {
			continue
		}
		encoded, ok := latin1(cell)
		if !ok {
			return "cell outside Latin-1: " + cell
		}

		offsets[cell] = len(stringData)
		stringData = append(append(stringData, encoded...), twoDaSeparatorNul)
	}

	for _, cell := range cells {
		built = binary.LittleEndian.AppendUint16(built, uint16(offsets[cell]))
	}
	built = binary.LittleEndian.AppendUint16(built, uint16(len(stringData)))
	built = append(built, stringData...)

	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
}

// checkSsfJson reads the string numbers of a converted sound set with jq and builds the sound set again: the
// signature, the slot table offset 12, and one u32 per slot, 0xFFFFFFFF for null. It returns an error
// message when the result differs from the source, or an empty string when it matches.
func checkSsfJson(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", ".slots[].strref", path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}

	built := []byte("SSF V1.1")
	built = binary.LittleEndian.AppendUint32(built, kotor.SsfHeaderSize)
	for _, text := range strings.Fields(string(result.Stdout)) {
		if text == "null" {
			built = binary.LittleEndian.AppendUint32(built, kotor.SsfEmpty)
			continue
		}

		number, err := strconv.ParseUint(text, 10, 32)
		if err != nil {
			return "bad string number " + text
		}
		built = binary.LittleEndian.AppendUint32(built, uint32(number))
	}

	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
}

// ncsMnemonics maps each listing mnemonic to its opcode and type.
var ncsMnemonics = func() map[string][2]byte {
	mnemonics := map[string][2]byte{}
	for opcode := range kotor.NcsOpcodes {
		for valueType := range 256 {
			if _, valid := kotor.NcsOperands(opcode, byte(valueType)); !valid {
				continue
			}

			mnemonic := kotor.NcsMnemonic(opcode, byte(valueType))
			if _, taken := mnemonics[mnemonic]; taken {
				kotor.Fatal("mnemonic " + mnemonic + " is ambiguous")
			}
			mnemonics[mnemonic] = [2]byte{opcode, byte(valueType)}
		}
	}
	return mnemonics
}()

// sizeOpcode is the instruction that carries the program size and has no type byte.
const sizeOpcode = 0x42

// checkNcsListing assembles a script listing and returns an error message when the result differs from the
// source script, or an empty string when it matches. Comment lines start with ";", and an instruction's
// comment starts at the first ";" after its operands.
func checkNcsListing(path string, source []byte) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}

	assembled := []byte("NCS V1.0")
	for _, text := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(text, ";") {
			continue
		}

		fields := strings.Fields(text)
		if len(fields) < 2 {
			return "short line: " + text
		}
		offset, err := strconv.ParseUint(fields[0], 16, 32)
		code, known := ncsMnemonics[fields[1]]
		if err != nil || int(offset) != len(assembled) || !known {
			return "bad line: " + text
		}

		assembled = append(assembled, code[0])
		if code[0] != sizeOpcode {
			assembled = append(assembled, code[1])
		}

		rest := strings.TrimSpace(text[strings.Index(text, fields[1])+len(fields[1]):])
		layout, _ := kotor.NcsOperands(code[0], code[1])
		for _, kind := range layout {
			if kind == 's' {
				quoted, err := strconv.QuotedPrefix(rest)
				value, unquoteErr := strconv.Unquote(quoted)
				if err != nil || unquoteErr != nil {
					return "bad string: " + text
				}

				assembled = binary.BigEndian.AppendUint16(assembled, uint16(len([]rune(value))))
				for _, char := range value {
					assembled = append(assembled, byte(char))
				}
				rest = strings.TrimSpace(rest[len(quoted):])
				continue
			}

			token, remainder, _ := strings.Cut(rest, " ")
			rest = strings.TrimSpace(remainder)
			var bits uint64
			if kind == 'f' {
				value, err := strconv.ParseFloat(token, 32)
				if err != nil {
					return "bad float: " + text
				}
				bits = uint64(math.Float32bits(float32(value)))
			} else {
				value, err := strconv.ParseInt(token, 10, 64)
				if err != nil {
					return "bad operand: " + text
				}
				bits = uint64(value)
			}

			switch kotor.NcsOperandSizes[kind] {
			case 4:
				assembled = binary.BigEndian.AppendUint32(assembled, uint32(bits))
			case 2:
				assembled = binary.BigEndian.AppendUint16(assembled, uint16(bits))
			case 1:
				assembled = append(assembled, byte(bits))
			}
		}
		if rest != "" && !strings.HasPrefix(rest, ";") {
			return "extra operands: " + text
		}
	}

	if !bytes.Equal(assembled, source) {
		return fmt.Sprintf("assembled %d bytes differ from the %d source bytes", len(assembled), len(source))
	}
	return ""
}

// tlkLine is the jq program that prints the language and then one line per entry: the string number, the
// flags, and the sound and text in base64, so that line feeds in the text cannot split a line.
const tlkLine = `.language, (.entries[] | "\(.strref) \(.flags) \(.sound | @base64) \(.text | @base64)")`

// Tlk: extracted/dialog.tlk.json against dialog.tlk.
func tlk(summary *kotor.Summary) {
	if !kotor.Has("jq") {
		summary.Note("jq missing: tlk UNVERIFIED")
		return
	}

	log("tlk...")
	source := kotor.ReadFile(filepath.Join(kotor.Game, "dialog.tlk"))
	problem := checkTlkJson(filepath.Join(kotor.Extracted, "dialog.tlk.json"), source)
	summary.Tally("jq_tlk", line("dialog.tlk.json "+kotor.Verdict(problem)))
}

// latin1Bytes encodes text as Latin-1 and reports whether every character fits in one byte.
func latin1Bytes(text string) ([]byte, bool) {
	out := []byte{}
	for _, character := range text {
		if character > 0xff {
			return nil, false
		}
		out = append(out, byte(character))
	}
	return out, true
}

// checkTlkJson builds a TLK file from jq's reading of the JSON form and returns an error message when the
// result differs from the source, or an empty string when it matches. Texts are laid out in entry order,
// an entry without text has offset 0, and the three unknown fields are 0.
func checkTlkJson(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", tlkLine, path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}

	lines := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
	language, err := strconv.ParseUint(lines[0], 10, 32)
	if err != nil {
		return "bad language " + lines[0]
	}

	count := len(lines) - 1
	table := []byte(kotor.TlkSignature)
	table = binary.LittleEndian.AppendUint32(table, uint32(language))
	table = binary.LittleEndian.AppendUint32(table, uint32(count))
	table = binary.LittleEndian.AppendUint32(table, uint32(kotor.TlkHeaderSize+kotor.TlkEntrySize*count))

	var texts []byte
	for i, text := range lines[1:] {
		fields := strings.Split(text, " ")
		if len(fields) != 4 || fields[0] != strconv.Itoa(i) {
			return fmt.Sprintf("entry %d is malformed: %s", i, text)
		}

		flags, flagsErr := strconv.ParseUint(fields[1], 10, 32)
		soundText, soundErr := base64.StdEncoding.DecodeString(fields[2])
		entryText, textErr := base64.StdEncoding.DecodeString(fields[3])
		sound, soundFits := latin1Bytes(string(soundText))
		body, bodyFits := latin1Bytes(string(entryText))
		if flagsErr != nil || soundErr != nil || textErr != nil || !soundFits || !bodyFits || len(sound) > kotor.ResrefLength {
			return fmt.Sprintf("entry %d has a bad value", i)
		}

		offset := len(texts)
		if len(body) == 0 {
			offset = 0
		}

		table = binary.LittleEndian.AppendUint32(table, uint32(flags))
		table = append(table, sound...)
		table = append(table, make([]byte, kotor.ResrefLength-len(sound))...)
		table = binary.LittleEndian.AppendUint32(table, 0)
		table = binary.LittleEndian.AppendUint32(table, 0)
		table = binary.LittleEndian.AppendUint32(table, uint32(offset))
		table = binary.LittleEndian.AppendUint32(table, uint32(len(body)))
		table = binary.LittleEndian.AppendUint32(table, 0)
		texts = append(texts, body...)
	}

	built := append(table, texts...)
	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
}

// Saves: extracted/saves/<save>/ against saves/<save>/. Each GFF file and each resource of the archives is
// checked as addResourceChecks describes. The screenshot PNG must equal ImageMagick's decoding of the
// source TGA and pass pngcheck. The decoded variables of GLOBALVARS.res must build again into the source
// fields (see checkGlobalsJson).
func saves(summary *kotor.Summary) {
	log("saves...")
	src := filepath.Join(kotor.Game, "saves")
	out := filepath.Join(kotor.Extracted, "saves")
	jobs := map[string][]job{}

	// checkArchive adds the checks of the resources of an ERF archive whose folder is dir, relative to out.
	var checkArchive func(dir string, data []byte)
	checkArchive = func(dir string, data []byte) {
		for _, entry := range kotor.ErfEntries(bytes.NewReader(data), int64(len(data)), dir) {
			label := filepath.Join(dir, entry.Resref)
			if entry.Ext == "sav" {
				checkArchive(label, entry.Data())
				continue
			}
			addResourceChecks(jobs, label, filepath.Join(out, label), entry.Ext, entry.Data())
		}
	}

	saveDirs, err := os.ReadDir(src)
	if err != nil {
		kotor.Fatal(err.Error())
	}
	for _, save := range saveDirs {
		if !save.IsDir() {
			continue
		}

		files, err := os.ReadDir(filepath.Join(src, save.Name()))
		if err != nil {
			kotor.Fatal(err.Error())
		}
		for _, file := range files {
			name := file.Name()
			data := kotor.ReadFile(filepath.Join(src, save.Name(), name))
			label := filepath.Join(save.Name(), strings.TrimSuffix(name, filepath.Ext(name)))
			path := filepath.Join(out, label)
			switch strings.ToLower(filepath.Ext(name)) {
			case ".res":
				addResourceChecks(jobs, label, path, "res", data)
				if filepath.Base(label) == "GLOBALVARS" {
					jobs["globals"] = append(jobs["globals"], func() []string {
						return line(label + ".variables.json " + kotor.Verdict(checkGlobalsJson(path+".variables.json", data)))
					})
				}

			case ".tga":
				jobs["imagemagick"] = append(jobs["imagemagick"], func() []string {
					source := magickDecode("tga", data)
					if source.err != "" {
						return line(label + ".png FAIL: source TGA: " + source.err)
					}
					width, height, _ := strings.Cut(source.size, " ")
					return pngCheck(label+".png", path+".png", width, height, source.rgba)
				})
				jobs["pngcheck"] = append(jobs["pngcheck"], func() []string {
					result := kotor.Exec("pngcheck", []string{path + ".png"}, nil)
					if result.Status != 0 {
						return line(label + ".png FAIL: pngcheck: " + kotor.FirstLine(string(result.Stdout)))
					}
					return line(label + ".png PASS")
				})

			case ".sav":
				checkArchive(label, data)
			}
		}
	}
	runResourceChecks(summary, "saves", jobs)
}

// checkGlobalsJson reads the decoded variables of a saved game with jq and builds the value fields of
// GLOBALVARS.res again: ValBoolean with eight booleans per byte, high bit first; ValNumber with one signed
// byte per number; and the strings of ValString. The names must equal the Name fields of the Cat lists,
// and location_data must equal ValLocation. It returns an error message when anything differs from the
// source, or an empty string when everything matches.
func checkGlobalsJson(path string, source []byte) string {
	queries := []string{
		"[.booleans[].name]", "[.booleans[].value]", "[.numbers[].name]", "[.numbers[].value]",
		"[.strings[].name]", "[.strings[].value]", ".locations", "[.location_data]",
	}
	result := kotor.Exec("jq", []string{"-c", strings.Join(queries, ", "), path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}

	lines := strings.Split(strings.TrimSpace(string(result.Stdout)), "\n")
	if len(lines) != len(queries) {
		return fmt.Sprintf("jq printed %d lines for %d queries", len(lines), len(queries))
	}

	var booleanNames, numberNames, stringNames, stringValues, locationNames, locationData []string
	var booleans []bool
	var numbers []int
	targets := []any{&booleanNames, &booleans, &numberNames, &numbers, &stringNames, &stringValues, &locationNames, &locationData}
	for i, target := range targets {
		if err := json.Unmarshal([]byte(lines[i]), target); err != nil {
			return queries[i] + ": " + err.Error()
		}
	}

	const bitsPerByte, highBit = 8, 0x80
	bits := make([]byte, (len(booleans)+bitsPerByte-1)/bitsPerByte)
	for i, value := range booleans {
		if value {
			bits[i/bitsPerByte] |= highBit >> (i % bitsPerByte)
		}
	}

	numberBytes := make([]byte, len(numbers))
	for i, value := range numbers {
		if value < math.MinInt8 || value > math.MaxInt8 {
			return fmt.Sprintf("number %d out of range", value)
		}
		numberBytes[i] = byte(int8(value))
	}

	gff, problem := kotor.ReadGff(source)
	if problem != "" {
		return "source: " + problem
	}

	fields := map[string]kotor.GffFieldValue{}
	for _, field := range gff.Root.Fields {
		fields[field.Label] = field.Value
	}
	listTexts := func(label string) []string {
		var texts []string
		for _, item := range fields[label].List {
			for _, field := range item.Fields {
				texts = append(texts, field.Value.Text)
			}
		}
		return texts
	}

	checks := []struct {
		name      string
		got, want any
	}{
		{"boolean names", booleanNames, listTexts("CatBoolean")},
		{"number names", numberNames, listTexts("CatNumber")},
		{"string names", stringNames, listTexts("CatString")},
		{"location names", locationNames, listTexts("CatLocation")},
		{"ValBoolean", bits, fields["ValBoolean"].Bytes},
		{"ValNumber", numberBytes, fields["ValNumber"].Bytes},
		{"ValString", stringValues, listTexts("ValString")},
		{"ValLocation", locationData, []string{hex.EncodeToString(fields["ValLocation"].Bytes)}},
	}
	for _, check := range checks {
		if fmt.Sprint(check.got) != fmt.Sprint(check.want) {
			return check.name + " differ from the source"
		}
	}
	return ""
}

// Movies queue their jobs first because each one takes longest, so short jobs fill the other slots.
var stages = []kotor.Stage{
	{Name: "movies", Run: movies},
	{Name: "models", Run: models},
	{Name: "textures", Run: textures},
	{Name: "tga", Run: tgaImages},
	{Name: "music", Run: func(summary *kotor.Summary) { streamAudio(summary, "music", "streammusic") }},
	{Name: "sounds", Run: func(summary *kotor.Summary) { streamAudio(summary, "sounds", "streamsounds") }},
	{Name: "waves", Run: func(summary *kotor.Summary) { streamAudio(summary, "waves", "streamwaves") }},
	{Name: "gui", Run: gui},
	{Name: "patch", Run: patch},
	{Name: "lips", Run: lips},
	{Name: "modules", Run: modules},
	{Name: "rims", Run: rims},
	{Name: "bifs", Run: bifs},
	{Name: "tlk", Run: tlk},
	{Name: "saves", Run: saves},
}

func main() {
	kotor.RunStages(stages, os.Args[1:])
}
