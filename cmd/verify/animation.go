// This file checks the animations, the skins and the node class headers of the model glTF files against the
// source MDL and MDX resources. jq reads the glTF file, and the source is walked here on its own, apart from
// the converter in kotor/mdl.go.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"re/kotor"
)

// Fields and sizes of the MDL structures that these checks read. Offsets count from the start of the model
// data, 12 bytes into the file.
const (
	sourceRootNode      = 0x28
	sourceAnimations    = 0x58
	sourceNameTable     = 0xb8
	sourceNameCount     = 0xc0
	sourceNodeNumber    = 0x02
	sourceNodeName      = 0x04
	sourceNodePosition  = 0x10
	sourceNodeRotation  = 0x1c
	sourceChildArray    = 0x2c
	sourceKeyArray      = 0x38
	sourceDataArray     = 0x44
	sourceNodeSize      = 80
	sourceKeySize       = 16
	sourceEventSize     = 36
	sourceStringLength  = 32
	sourceAabbNodeSize  = 40
	sourceFlagLight     = 0x02
	sourceFlagEmitter   = 0x04
	sourceFlagReference = 0x10
	sourceFlagMesh      = 0x20
	sourceFlagSkin      = 0x40
	sourceFlagDangly    = 0x100
	sourceFlagAabb      = 0x200
	unsetIndex          = 0xffffffff
)

// sourceMeshSize returns the size of the mesh header, which is 4 bytes shorter in the Xbox build.
func sourceMeshSize() int {
	if kotor.Build.Xbox {
		return 0x148
	}
	return 0x14c
}

// The controller types of position, orientation and scale, and the layout of the controller columns.
const (
	keyPosition    = 8
	keyOrientation = 20
	keyScale       = 36
	bezierBit      = 0x10
	columnMask     = 0x0f
	packedColumns  = 2
)

// Decoded values that are computed in float64 and rounded to f32, such as normalized rotations, may differ
// from the converter's by this much.
const valueTolerance = 1e-5

// sourceModel is the model data of an MDL file with readers that return 0 outside it.
type sourceModel struct {
	data  []byte
	names []string
}

func newSourceModel(mdl []byte) sourceModel {
	model := sourceModel{data: kotor.Slice(mdl, 12, len(mdl))}
	table := model.u32(sourceNameTable)
	for i := range model.u32(sourceNameCount) {
		start := model.u32(table + 4*i)
		model.names = append(model.names, kotor.CString(kotor.Slice(model.data, start, len(model.data))))
	}
	return model
}

func (m sourceModel) u32(offset int) int {
	return readUint(m.data, offset, 4)
}

func (m sourceModel) u16(offset int) int {
	return readUint(m.data, offset, 2)
}

func (m sourceModel) f32(offset int) float64 {
	return float64(math.Float32frombits(uint32(m.u32(offset))))
}

func (m sourceModel) text(offset, length int) string {
	return kotor.CString(kotor.Slice(m.data, offset, offset+length))
}

func (m sourceModel) name(node int) string {
	if index := m.u16(node + sourceNodeName); index < len(m.names) {
		return m.names[index]
	}
	return ""
}

// tree returns the offsets of the nodes below root in depth-first order, children in stored order. Each
// node is visited once.
func (m sourceModel) tree(root int) []int {
	var nodes []int
	seen := map[int]bool{}
	var walk func(offset int)
	walk = func(offset int) {
		if seen[offset] {
			return
		}
		seen[offset] = true
		nodes = append(nodes, offset)
		for i := range m.u32(offset + sourceChildArray + 4) {
			walk(m.u32(m.u32(offset+sourceChildArray) + 4*i))
		}
	}
	walk(root)
	return nodes
}

// pairNodes pairs the tree of model below root with the tree of other below otherRoot, as the game pairs a
// model with an animation (PC code, 0x483E92 to 0x483EEC). The roots are paired. Below a pair, child i of
// the model node takes the first child of the other node, searched from child i on with wrap-around, that
// has the same node number. Names play no part. It returns the pairs of node offsets in the order of the
// walk.
func pairNodes(model sourceModel, root int, other sourceModel, otherRoot int) [][2]int {
	children := func(m sourceModel, node int) []int {
		var result []int
		for i := range m.u32(node + sourceChildArray + 4) {
			result = append(result, m.u32(m.u32(node+sourceChildArray)+4*i))
		}
		return result
	}

	var pairs [][2]int
	var pair func(node, match int)
	pair = func(node, match int) {
		pairs = append(pairs, [2]int{node, match})
		candidates := children(other, match)
		for i, child := range children(model, node) {
			for k := range candidates {
				candidate := candidates[(i+k)%len(candidates)]
				if other.u16(candidate+sourceNodeNumber) == model.u16(child+sourceNodeNumber) {
					pair(child, candidate)
					break
				}
			}
		}
	}
	pair(root, otherRoot)
	return pairs
}

// jqLines runs a jq program on a file and returns its output lines.
func jqLines(program, path string) ([]string, string) {
	result := kotor.Exec("jq", []string{"-r", program, path}, nil)
	if result.Status != 0 {
		return nil, "jq: " + kotor.FirstLine(result.Stderr)
	}
	text := strings.TrimSuffix(string(result.Stdout), "\n")
	if text == "" {
		return nil, ""
	}
	return strings.Split(text, "\n"), ""
}

// lineReader hands out jq output lines one by one and records the first mismatch.
type lineReader struct {
	lines   []string
	next    int
	problem string
}

func (r *lineReader) take() string {
	r.next++
	if r.next > len(r.lines) {
		return ""
	}
	return r.lines[r.next-1]
}

func (r *lineReader) fail(text string) {
	if r.problem == "" {
		r.problem = text
	}
}

func (r *lineReader) expect(what, want string) {
	if got := r.take(); got != want {
		r.fail(fmt.Sprintf("%s %q, source %q", what, got, want))
	}
}

// expectFloat compares a number that jq printed with an f32 of the source, bit for bit after rounding.
func (r *lineReader) expectFloat(what string, want float64) {
	got := r.take()
	value, err := strconv.ParseFloat(got, 32)
	if err != nil || float32(value) != float32(want) {
		r.fail(fmt.Sprintf("%s %q, source %v", what, got, want))
	}
}

// location parses the "offset count" pair that gltfLocation prints.
func (r *lineReader) location() (int, int) {
	var start, count int
	if _, err := fmt.Sscan(r.take(), &start, &count); err != nil {
		r.fail("bad accessor location")
	}
	return start, count
}

// floatsAt reads count f32 values from the binary buffer.
func floatsAt(bin []byte, start, count int) []float64 {
	values := make([]float64, count)
	for i := range values {
		values[i] = float64(math.Float32frombits(uint32(readUint(bin, start+4*i, 4))))
	}
	return values
}

// animationProgram prints, for each glTF animation, its name and extras, then its channels with the glTF
// index of the target node, the path, the interpolation and the locations of the input and output accessors.
const animationProgram = gltfLocation + ` . as $d | .buffers[0].uri, (.animations[]? | .name,
	(.extras | .length, .transition, .root, (.events | length), (.events[] | .time, .name),
	(.tracks // [] | length), (.tracks[]? | .node, .controller, .interpolation, .width, (.input | location($d)),
		(.output | location($d))), (.constant_channel // false)),
	(.channels | length), (. as $a | .channels[] | $a.samplers[.sampler] as $s | .target.node, .target.path,
		$s.interpolation, ($s.input | location($d)), ($s.output | location($d))), "end")`

// sourceTrack is one controller of an animation node, decoded here: its target glTF node, its type, its
// key times as source bytes and its rows of values.
type sourceTrack struct {
	node   int
	kind   int
	times  []byte
	rows   [][]float64
	bezier bool
}

// unpackRotation unpacks an orientation key stored as one u32, as README.md describes.
func unpackRotation(packed uint32) []float64 {
	x := 1 - float64(packed&0x7ff)/1023
	y := 1 - float64(packed>>11&0x7ff)/1023
	z := 1 - float64(packed>>22)/511
	square := x*x + y*y + z*z
	if square < 1 {
		return []float64{x, y, z, -math.Sqrt(1 - square)}
	}
	length := math.Sqrt(square)
	return []float64{x / length, y / length, z / length, 0}
}

// sourceControllers decodes the controllers of the node at offset.
func sourceControllers(m sourceModel, offset int) []sourceTrack {
	keys := m.u32(offset + sourceKeyArray)
	data := m.u32(offset + sourceDataArray)
	var tracks []sourceTrack
	for i := range m.u32(offset + sourceKeyArray + 4) {
		key := keys + sourceKeySize*i
		kind := m.u32(key)
		rows := m.u16(key + 6)
		timeIndex := m.u16(key + 8)
		valueIndex := m.u16(key + 10)
		columns := int(m.data[min(key+12, len(m.data)-1)])

		width := columns & columnMask
		bezier := columns&bezierBit != 0
		if bezier {
			width *= 3
		}
		packed := kind == keyOrientation && columns == packedColumns
		if packed {
			width = 1
		}

		track := sourceTrack{kind: kind, bezier: bezier, times: kotor.Slice(m.data, data+4*timeIndex, data+4*(timeIndex+rows))}
		for row := range rows {
			at := data + 4*(valueIndex+row*width)
			if packed {
				track.rows = append(track.rows, unpackRotation(uint32(m.u32(at))))
				continue
			}
			var values []float64
			for column := range width {
				values = append(values, m.f32(at+4*column))
			}
			track.rows = append(track.rows, values)
		}
		tracks = append(tracks, track)
	}
	return tracks
}

// checkAnimations compares the animations of a model glTF file with the source. path is the converted file
// without extensions. Each animation node animates the model nodes that pairNodes pairs with it. Its
// position, orientation and scale controllers must be channels, in node and controller order. The input of each channel and track must equal the source key times
// byte for byte. A translation must be the rest position plus the stored offset, a rotation the unpacked or
// stored quaternion, normalized, and a scale the stored scale on all three axes. A Bezier position must be a
// cubic spline whose tangents are three times the control offsets divided by the key interval. Other
// controllers must be tracks whose output equals the stored values bit for bit.
func checkAnimations(path string, mdl []byte) string {
	lines, problem := jqLines(animationProgram, path+".gltf")
	if problem != "" {
		return problem
	}
	bin, err := os.ReadFile(path + ".bin")
	if err != nil {
		return err.Error()
	}
	reader := &lineReader{lines: lines}
	reader.take()

	model := newSourceModel(mdl)
	geometry := model.tree(model.u32(sourceRootNode))
	glTfIndex := map[int]int{}
	for i, node := range geometry {
		glTfIndex[node] = i + 1
	}

	offsets := model.u32(sourceAnimations)
	for i := range model.u32(sourceAnimations + 4) {
		header := model.u32(offsets + 4*i)
		name := model.text(header+8, sourceStringLength)
		reader.expect("animation name", name)
		reader.expectFloat("length", model.f32(header+0x50))
		reader.expectFloat("transition", model.f32(header+0x54))
		reader.expect("root", model.text(header+0x58, sourceStringLength))
		events, eventCount := model.u32(header+0x78), model.u32(header+0x7c)
		reader.expect("event count", strconv.Itoa(eventCount))
		for e := range eventCount {
			reader.expectFloat("event time", model.f32(events+sourceEventSize*e))
			reader.expect("event name", model.text(events+sourceEventSize*e+4, sourceStringLength))
		}

		type channel struct {
			node int
			path string
		}
		var channels []sourceTrack
		var paths []channel
		var tracks []sourceTrack
		pairs := pairNodes(model, geometry[0], model, model.u32(header+sourceRootNode))
		for _, node := range model.tree(model.u32(header + sourceRootNode)) {
			for _, pair := range pairs {
				if pair[1] != node {
					continue
				}
				for _, track := range sourceControllers(model, node) {
					track.node = glTfIndex[pair[0]]
					kind := map[int]string{keyPosition: "translation", keyOrientation: "rotation", keyScale: "scale"}[track.kind]
					if kind == "" {
						tracks = append(tracks, track)
						continue
					}
					channels = append(channels, track)
					paths = append(paths, channel{track.node, kind})
				}
			}
		}

		reader.expect("track count", strconv.Itoa(len(tracks)))
		for _, track := range tracks {
			reader.expect("track node", strconv.Itoa(track.node))
			reader.take()
			interpolation := "LINEAR"
			if track.bezier {
				interpolation = "BEZIER"
			}
			reader.expect("track interpolation", interpolation)
			reader.expect("track width", strconv.Itoa(len(track.rows[0])))
			start, count := reader.location()
			if count != len(track.rows) || !bytes.Equal(kotor.Slice(bin, start, start+len(track.times)), track.times) {
				reader.fail("track times differ in " + name)
			}
			start, count = reader.location()
			var want []byte
			for _, row := range track.rows {
				for _, value := range row {
					want = binary.LittleEndian.AppendUint32(want, math.Float32bits(float32(value)))
				}
			}
			if count*4 != len(want) || !bytes.Equal(kotor.Slice(bin, start, start+len(want)), want) {
				reader.fail("track values differ in " + name)
			}
		}

		// An animation without position, orientation and scale tracks has one constant channel on the root.
		constant := len(channels) == 0
		reader.expect("constant channel", strconv.FormatBool(constant))
		if constant {
			channels = []sourceTrack{{node: 1, kind: keyPosition, times: binary.LittleEndian.AppendUint32(nil, 0), rows: [][]float64{{0, 0, 0}}}}
			paths = []channel{{1, "translation"}}
		}

		reader.expect("channel count", strconv.Itoa(len(channels)))
		for c, track := range channels {
			reader.expect("channel node", strconv.Itoa(track.node))
			reader.expect("channel path", paths[c].path)
			cubic := track.kind == keyPosition && track.bezier && len(track.rows) > 1
			interpolation := "LINEAR"
			if cubic {
				interpolation = "CUBICSPLINE"
			}
			reader.expect("interpolation", interpolation)

			start, count := reader.location()
			if count != len(track.rows) || !bytes.Equal(kotor.Slice(bin, start, start+len(track.times)), track.times) {
				reader.fail(fmt.Sprintf("times of the %s channel of node %d differ in %s", paths[c].path, track.node, name))
			}
			times := floatsAt(track.times, 0, len(track.rows))

			var want []float64
			rest := geometry[track.node-1] + sourceNodePosition
			for k, row := range track.rows {
				switch track.kind {
				case keyPosition:
					var value, incoming, outgoing [3]float64
					for axis := range 3 {
						value[axis] = float64(float32(model.f32(rest+4*axis) + row[axis]))
						if cubic && k > 0 {
							incoming[axis] = -3 * row[3+axis] / (times[k] - times[k-1])
						}
						if cubic && k < len(track.rows)-1 {
							outgoing[axis] = 3 * row[6+axis] / (times[k+1] - times[k])
						}
					}
					if cubic {
						want = append(want, incoming[:]...)
					}
					want = append(want, value[:]...)
					if cubic {
						want = append(want, outgoing[:]...)
					}
				case keyOrientation:
					length := math.Sqrt(row[0]*row[0] + row[1]*row[1] + row[2]*row[2] + row[3]*row[3])
					want = append(want, row[0]/length, row[1]/length, row[2]/length, row[3]/length)
				case keyScale:
					want = append(want, row[0], row[0], row[0])
				}
			}

			start, count = reader.location()
			got := floatsAt(bin, start, len(want))
			for v := range want {
				if math.Abs(got[v]-want[v]) > valueTolerance*max(1, math.Abs(want[v])) {
					reader.fail(fmt.Sprintf("value %d of the %s channel of node %d is %v, source %v in %s", v, paths[c].path, track.node, got[v], want[v], name))
					break
				}
			}
			if count*map[int]int{keyPosition: 3, keyOrientation: 4, keyScale: 3}[track.kind] != len(want) {
				reader.fail("output count differs in " + name)
			}
		}
		reader.expect("end of animation", "end")
	}

	if reader.problem != "" {
		return reader.problem
	}
	if reader.next != len(lines) {
		return fmt.Sprintf("jq printed %d lines, %d expected", len(lines), reader.next)
	}
	return ""
}

// skinProgram prints, for each node with a skinned mesh, its name, the locations of the joints and the weights
// of the mesh, and the names of the joints of its skin, separated by tabs.
const skinProgram = gltfLocation + ` . as $d | .nodes[] | select(.extras.skin) | .name,
	($d.nodes[.extras.skin.node] as $s | $d.meshes[$s.mesh].primitives[0].attributes as $p
		| ($p.JOINTS_0 | location($d)), ($p.WEIGHTS_0 | location($d)),
		([$d.skins[$s.skin].joints[] | $d.nodes[.].name] | join("\t")))`

// checkSkins compares the skins of a model glTF file with the source. Every rendered skinned mesh with faces
// and vertex records must have a skin, in source order. Its joints must be the nodes named by the name table
// entries of its local bones, in local bone order. With the MDX data, its joints must equal the stored bone
// indices, with 0 for -1, and its weights the stored weights divided by their sum. The bone map entries and
// the bone indices are f32 on PC and i16 on Xbox.
func checkSkins(path string, mdl, mdx []byte) string {
	const (
		weightOffset = 0x0c
		indexOffset  = 0x10
		boneMap      = 0x14
		strideField  = 0xfc
		vertexCount  = 0x130
		renderByte   = 0x139
		mdxField     = 0x144
		faceCount    = 0x0c
		slots        = 4
	)
	lines, problem := jqLines(skinProgram, path+".gltf")
	if problem != "" {
		return problem
	}
	bin, err := os.ReadFile(path + ".bin")
	if err != nil {
		return err.Error()
	}
	reader := &lineReader{lines: lines}

	model := newSourceModel(mdl)
	for _, node := range model.tree(model.u32(sourceRootNode)) {
		mesh := node + sourceNodeSize
		skin := mesh + sourceMeshSize()
		vertices := model.u16(mesh + vertexCount)
		stride := model.u32(mesh + strideField)
		if model.u16(node)&sourceFlagSkin == 0 || model.data[min(mesh+renderByte, len(model.data)-1)] == 0 ||
			model.u32(mesh+faceCount) == 0 || vertices == 0 || stride == 0 || len(mdl) == 0 {
			continue
		}
		reader.expect("skinned node", model.name(node))
		jointStart, jointCount := reader.location()
		weightStart, weightCount := reader.location()

		var bones []string
		mapStart, mapCount := model.u32(skin+boneMap), model.u32(skin+boneMap+4)
		boneIndex := func(data []byte, start, i int) float64 {
			if kotor.Build.Xbox {
				return float64(int16(readUint(data, start+2*i, 2)))
			}
			return float64(math.Float32frombits(uint32(readUint(data, start+4*i, 4))))
		}
		for bone := 0; ; bone++ {
			found := ""
			for entry := range mapCount {
				if boneIndex(model.data, mapStart, entry) == float64(bone) && entry < len(model.names) {
					found = model.names[entry]
				}
			}
			if found == "" {
				break
			}
			bones = append(bones, found)
		}
		reader.expect("joints", strings.Join(bones, "\t"))

		if jointCount != vertices || weightCount != vertices {
			reader.fail(fmt.Sprintf("%d joints and %d weights for %d vertices", jointCount, weightCount, vertices))
		}
		if mdx == nil {
			continue
		}
		records := model.u32(mesh + mdxField)
		weights := model.u32(skin + weightOffset)
		indices := model.u32(skin + indexOffset)
		for v := range vertices {
			record := records + stride*v
			stored := floatsAt(mdx, record+weights, slots)
			sum := stored[0] + stored[1] + stored[2] + stored[3]
			for slot := range slots {
				bone := boneIndex(mdx, record+indices, slot)
				joint := readUint(bin, jointStart+2*(slots*v+slot), 2)
				weight := floatsAt(bin, weightStart+4*(slots*v+slot), 1)[0]
				if joint != int(max(bone, 0)) || math.Abs(weight-stored[slot]/sum) > valueTolerance {
					reader.fail(fmt.Sprintf("vertex %d of %s: joint %d weight %v, source bone %v weight %v", v, model.name(node), joint, weight, bone, stored[slot]))
				}
			}
		}
	}

	if reader.problem != "" {
		return reader.problem
	}
	if reader.next != len(lines) {
		return fmt.Sprintf("jq printed %d lines, %d expected", len(lines), reader.next)
	}
	return ""
}

// nodeProgram prints, for each node whose extras hold static controllers or a light, emitter, reference,
// dangly or AABB header, its name and those values. Absent headers print "-".
const nodeProgram = gltfLocation + ` . as $d | .nodes[] | .name as $n | .extras // empty
	| select(.light or .emitter or .reference or .dangly or .aabb or .controllers) | $n,
	(if .light then .light | "light", .flare_radius, (.flares | length),
		(.flares[] | .texture, .size, .position, .color_shift[]), .priority, .ambient_only, .dynamic_type,
		.affect_dynamic, .shadow, .generate_flare, .fading_light else "-" end),
	(if .emitter then .emitter | "emitter", .dead_space, .blast_radius, .blast_length, .branch_count,
		.control_point_smoothing, .x_grid, .y_grid, .spawn_type, .update, .render, .blend, .texture,
		.chunk_name, .two_sided_texture, .loop, .render_order, .frame_blending, .depth_texture, .flags
		else "-" end),
	(if .reference then .reference | "reference", .model, .reattachable else "-" end),
	(if .dangly then .dangly | "dangly", .displacement, .tightness, .period,
		(if .constraints then .constraints | location($d) else "none" end) else "-" end),
	(if .aabb then .aabb | "aabb", (.boxes | location($d)), (.nodes | location($d)) else "-" end),
	(.controllers // [] | length), (.controllers[]? | .values | map(tostring) | join(" ")),
	"end"`

// checkNodeExtras compares the node extras of a model glTF file with the source headers. The light, emitter
// and reference fields must equal the header fields, the dangly constraints the stored f32 values and the
// AABB accessors the tree read here depth first. Each static controller other than position and orientation
// must appear among the controller values of its node, bit for bit after rounding to f32.
func checkNodeExtras(path string, mdl []byte) string {
	lines, problem := jqLines(nodeProgram, path+".gltf")
	if problem != "" {
		return problem
	}
	bin, err := os.ReadFile(path + ".bin")
	if err != nil {
		return err.Error()
	}
	reader := &lineReader{lines: lines}
	model := newSourceModel(mdl)
	number := func(what string, offset, size int) {
		reader.expect(what, strconv.Itoa(readUint(model.data, offset, size)))
	}

	for _, node := range model.tree(model.u32(sourceRootNode)) {
		flags := model.u16(node)
		header := node + sourceNodeSize
		subclass := header + sourceMeshSize()
		var statics []sourceTrack
		for _, track := range sourceControllers(model, node) {
			if track.kind != keyPosition && track.kind != keyOrientation {
				statics = append(statics, track)
			}
		}
		if flags&(sourceFlagLight|sourceFlagEmitter|sourceFlagReference|sourceFlagDangly|sourceFlagAabb) == 0 && len(statics) == 0 {
			continue
		}
		reader.expect("node", model.name(node))

		if flags&sourceFlagLight != 0 {
			reader.expect("class", "light")
			reader.expectFloat("flare radius", model.f32(header))
			sizes, positions, shifts, textures := model.u32(header+0x10), model.u32(header+0x1c), model.u32(header+0x28), model.u32(header+0x34)
			flares := model.u32(header + 0x14)
			reader.expect("flare count", strconv.Itoa(flares))
			for f := range flares {
				reader.expect("flare texture", kotor.CString(kotor.Slice(model.data, model.u32(textures+4*f), len(model.data))))
				reader.expectFloat("flare size", model.f32(sizes+4*f))
				reader.expectFloat("flare position", model.f32(positions+4*f))
				for c := range 3 {
					reader.expectFloat("flare colour shift", model.f32(shifts+12*f+4*c))
				}
			}
			for field := range 7 {
				number("light field", header+0x40+4*field, 4)
			}
		} else {
			reader.expect("light", "-")
		}

		if flags&sourceFlagEmitter != 0 {
			reader.expect("class", "emitter")
			for _, field := range []int{0x00, 0x04, 0x08} {
				reader.expectFloat("emitter float", model.f32(header+field))
			}
			number("branch count", header+0x0c, 4)
			number("control point smoothing", header+0x10, 4)
			for _, field := range []int{0x14, 0x18, 0x1c} {
				number("emitter grid or spawn", header+field, 4)
			}
			for _, field := range []int{0x20, 0x40, 0x60, 0x80} {
				reader.expect("emitter name", model.text(header+field, sourceStringLength))
			}
			reader.expect("chunk name", model.text(header+0xa0, 16))
			number("two-sided texture", header+0xb0, 4)
			number("loop", header+0xb4, 4)
			number("render order", header+0xb8, 2)
			number("frame blending", header+0xba, 1)
			reader.expect("depth texture", model.text(header+0xbb, sourceStringLength))
			number("emitter flags", header+0xdc, 4)
		} else {
			reader.expect("emitter", "-")
		}

		if flags&sourceFlagReference != 0 {
			reader.expect("class", "reference")
			reader.expect("model", model.text(header, sourceStringLength))
			number("reattachable", header+sourceStringLength, 4)
		} else {
			reader.expect("reference", "-")
		}

		if flags&sourceFlagDangly != 0 {
			reader.expect("class", "dangly")
			reader.expectFloat("displacement", model.f32(subclass+0x0c))
			reader.expectFloat("tightness", model.f32(subclass+0x10))
			reader.expectFloat("period", model.f32(subclass+0x14))
			constraints, count := model.u32(subclass), model.u32(subclass+4)
			if count == 0 {
				reader.expect("constraints", "none")
			} else {
				start, written := reader.location()
				if written != count || !bytes.Equal(kotor.Slice(bin, start, start+4*count), kotor.Slice(model.data, constraints, constraints+4*count)) {
					reader.fail("constraints differ in " + model.name(node))
				}
			}
		} else {
			reader.expect("dangly", "-")
		}

		if flags&sourceFlagAabb != 0 {
			reader.expect("class", "aabb")
			var boxes, links []byte
			var walk func(at int) int
			count := 0
			walk = func(at int) int {
				index := count
				count++
				boxes = append(boxes, kotor.Slice(model.data, at, at+24)...)
				slot := len(links)
				links = append(links, make([]byte, 16)...)
				left, right := unsetIndex, unsetIndex
				face := model.u32(at + 0x20)
				if face == unsetIndex {
					left = walk(model.u32(at + 0x18))
					right = walk(model.u32(at + 0x1c))
				}
				for k, value := range []int{left, right, face, model.u32(at + 0x24)} {
					binary.LittleEndian.PutUint32(links[slot+4*k:], uint32(value))
				}
				return index
			}
			walk(model.u32(subclass))
			start, written := reader.location()
			if written != 2*count || !bytes.Equal(kotor.Slice(bin, start, start+len(boxes)), boxes) {
				reader.fail("aabb boxes differ in " + model.name(node))
			}
			start, written = reader.location()
			if written != count || !bytes.Equal(kotor.Slice(bin, start, start+len(links)), links) {
				reader.fail("aabb nodes differ in " + model.name(node))
			}
		} else {
			reader.expect("aabb", "-")
		}

		reader.expect("controller count", strconv.Itoa(len(statics)))
		written := map[string]int{}
		for range statics {
			var values []string
			for text := range strings.FieldsSeq(reader.take()) {
				value, err := strconv.ParseFloat(text, 32)
				if err != nil {
					reader.fail("bad controller value " + text)
				}
				values = append(values, strconv.FormatUint(uint64(math.Float32bits(float32(value))), 16))
			}
			written[strings.Join(values, " ")]++
		}
		for _, track := range statics {
			var values []string
			for _, value := range track.rows[0] {
				values = append(values, strconv.FormatUint(uint64(math.Float32bits(float32(value))), 16))
			}
			key := strings.Join(values, " ")
			if written[key] == 0 {
				reader.fail(fmt.Sprintf("controller %d of %s not written", track.kind, model.name(node)))
			}
			written[key]--
		}
		reader.expect("end of node", "end")
	}

	if reader.problem != "" {
		return reader.problem
	}
	if reader.next != len(lines) {
		return fmt.Sprintf("jq printed %d lines, %d expected", len(lines), reader.next)
	}
	return ""
}
