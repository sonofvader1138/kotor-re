// This file converts MDL models with their MDX vertex data to glTF 2.0. The node tree, the rendered meshes
// and their textures become glTF nodes, meshes and materials. The decoded mesh headers and the face fields
// become extras of the nodes. Layout: see README.md. MDL offsets are relative to file offset 12.

package kotor

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

const none = 0xffffffff

// The first 12 bytes of an MDL file are a file header. All offsets count from the end of it.
const fileHeaderSize = 12

// A node starts with an 80-byte header. Bit 5 of its flags marks a mesh node, whose mesh header follows.
const (
	nodeHeaderSize    = 80
	nodeFlagMesh      = 0x20
	textureNameLength = 32
)

// Node flags that select a mesh subclass. A node with none of them holds a plain triangle mesh.
const (
	nodeFlagSkin       = 0x40
	nodeFlagDangly     = 0x100
	nodeFlagAabb       = 0x200
	nodeFlagLightsaber = 0x800
)

// The first two words of a mesh header are a pair of addresses in the executable, one pair per mesh class.
// AABB and lightsaber meshes use the pair of the plain triangle mesh.
var meshClassAddresses = map[string][2]int{
	"trimesh": {0x00405750, 0x00405760},
	"dangly":  {0x00405740, 0x00405730},
	"skin":    {0x00405710, 0x00405720},
}

// A face is 32 bytes: the f32[3] face normal, the f32 plane distance, the u32 material, the u16[3] adjacent
// faces and the u16[3] vertex indices.
const (
	faceSize            = 32
	facePlaneOffset     = 0x0c
	faceMaterialOffset  = 0x10
	faceAdjacentOffset  = 0x14
	faceVerticesOffset  = 0x1a
	vertexSize          = 12
	noAdjacentFace      = 0xffff
	cornersPerFace      = 3
	faceIndexSize       = 2
	indexArrayEntrySize = 4
)

// Offsets of the mesh header fields.
const (
	meshFaceArray        = 0x08
	meshBoundsMin        = 0x14
	meshBoundsMax        = 0x20
	meshRadius           = 0x2c
	meshAveragePoint     = 0x30
	meshDiffuse          = 0x3c
	meshAmbient          = 0x48
	meshTransparencyHint = 0x54
	meshTexture          = 0x58
	meshLightmap         = 0x78
	meshIndexCounts      = 0xb0
	meshIndexOffsets     = 0xbc
	meshCounters         = 0xc8
	meshUvScrollEnabled  = 0xe8
	meshUvScroll         = 0xec
	meshVertexStride     = 0xfc
	meshVertexFlags      = 0x100
	meshVertexCount      = 0x130
	meshUvSetCount       = 0x132
	meshLightmapped      = 0x134
	meshShadow           = 0x137
	meshRender           = 0x139
	meshSurfaceArea      = 0x13c
	meshMdxOffset        = 0x144
	meshVertexOffset     = 0x148
)

// Each vertex component of an MDX vertex record has a bit in the vertex flags and an offset in the mesh header
// that gives its position within the record, or 0xFFFFFFFF when the component is absent.
var vertexComponents = []struct {
	bit    int
	offset int
	name   string
	size   int
}{
	{0x01, 0x104, "position", 12},
	{0x20, 0x108, "normal", 12},
	{0x02, 0x110, "uv1", 8},
	{0x04, 0x114, "uv2", 8},
	{0x80, 0x120, "tangent_space", 36},
}

// The vertex records of a skinned mesh carry 32 more bytes, which the skin header describes: four f32 bone
// weights and four f32 local bone indices. The weights of a vertex sum to 1 within weightTolerance.
const (
	mdxAlignment    = 16
	plainTerminator = 1e7
	skinTerminator  = 1e6
	skinVertexExtra = 32
	skinWeightsSize = 16
	skinSlots       = 4
	weightTolerance = 1e-3
)

// The model header is 0xC4 bytes. Its first two words are constant, and its geometry type is 2.
const (
	modelHeaderSize = 0xc4
	modelType       = 2
	modelNameLength = 32
)

var modelClassAddresses = [2]int{0x00413670, 0x00405520}

// Fields of the node header after the flags, the node number and the name index.
const (
	nodeOwnerField  = 0x08
	nodeParentField = 0x0c
	nodeChildArray  = 0x2c
)

// A stored inverse bind pose agrees with the computed one when the rotations differ by less than
// bindAngleTolerance radians and the translations by less than bindDistanceTolerance.
const (
	bindAngleTolerance    = 0.01
	bindDistanceTolerance = 1e-3
)

// The glTF node that carries a skinned mesh is named after the mesh's node with this suffix.
const skinNodeSuffix = "_skin"

// An inverse bind matrix is a 4x4 matrix of f32.
const matrixSize = 16

// Deeper node trees are treated as corrupt.
const maxNodeDepth = 200

// The MDX copy of a vertex position must match the MDL copy within this distance.
const positionTolerance = 1e-5

// Faces with a smaller cross product are degenerate and their normal is not checked.
const minCrossLength = 1e-9

// A stored face normal agrees with the winding when the cosine between them exceeds this value.
const minNormalCosine = 0.99

// A vertex normal counts as unit length when its length differs from 1 by less than this value, and as zero
// when its length is below zeroLength. glTF requires unit normals. Zero normals are replaced (see the mesh
// conversion), and a mesh with any other normal is written without normals.
const (
	unitTolerance = 1e-3
	zeroLength    = 1e-6
)

// upAxis replaces a zero normal of a vertex that no face uses.
var upAxis = vec3{0, 0, 1}

// Tolerances for the derived mesh header values: the plane distance, the radius and the surface area.
const (
	planeTolerance        = 1e-3
	radiusTolerance       = 1e-3
	areaRelativeTolerance = 1e-3
	boundsTolerance       = 1e-4
)

type vec3 = [3]float64

// MdlStats counts what the converter found. Each "Matching" count goes with a "Checked" count.
type MdlStats struct {
	Nodes           int
	Hidden          int
	Meshes          int
	Verts           int
	Faces           int
	Mismatch        int
	NormalsMatching int
	NormalsChecked  int

	PlanesMatching   int
	PlanesChecked    int
	AdjacentMatching int
	AdjacentChecked  int
	AdjacentOutside  int
	RadiusMatching   int
	RadiusChecked    int
	AreaMatching     int
	AreaChecked      int

	// NormalsReplaced counts the zero MDX normals that are replaced. NormalsOmitted counts the rendered
	// meshes with MDX normals that are written without them, because a normal is neither zero nor of unit
	// length.
	NormalsReplaced int
	NormalsOmitted  int

	// HdrNodes is the node count field of the header, as text: "?" when it lies outside the file.
	HdrNodes string

	// Animations, AnimationNodes, Keys, Channels and Tracks count the animations, the nodes of their trees,
	// the keys of the controllers that animate nodes of the model, the glTF channels and the other tracks.
	// UnmatchedAnimationNodes counts animation nodes whose name is no node of the model tree, and
	// ConstantChannels the animations that get a constant channel. DuplicateTracks counts the tracks that are
	// dropped because an earlier animation node animates the same property of the same node.
	Animations              int
	AnimationNodes          int
	Keys                    int
	Channels                int
	Tracks                  int
	UnmatchedAnimationNodes int
	ConstantChannels        int
	DuplicateTracks         int

	// ControllersMatching counts the position and orientation controllers of the model tree whose single
	// key equals the position or rotation in the node header.
	ControllersMatching int
	ControllersChecked  int

	// Skins counts the glTF skins and Bones their joints. BindMatching counts the stored inverse bind poses
	// that agree with the ones computed from the node tree.
	Skins        int
	Bones        int
	BindMatching int
	BindChecked  int
	WeightsBad   int

	// AabbMatching counts the AABB trees whose leaves hold every face once and whose boxes enclose what lies
	// below them.
	AabbMatching int
	AabbChecked  int

	// Unclaimed counts the bytes of the model data that no decoded structure covers, and Shared the bytes
	// that two structures cover. MdxPadding counts the MDX bytes that align the records of a mesh, and
	// MdxUnclaimed the other MDX bytes outside every vertex record.
	Unclaimed    int
	Shared       int
	MdxPadding   int
	MdxUnclaimed int
}

// MdlUvScroll holds the texture animation of a mesh whose scroll flag is set.
type MdlUvScroll struct {
	Direction   [2]float32 `json:"direction"`
	Jitter      float32    `json:"jitter"`
	JitterSpeed float32    `json:"jitter_speed"`
}

// MdlMesh holds the decoded fields of one mesh header.
type MdlMesh struct {
	Name             string       `json:"name"`
	Kind             string       `json:"kind"`
	Render           bool         `json:"render"`
	Shadow           bool         `json:"shadow"`
	Lightmapped      bool         `json:"lightmapped"`
	Texture          string       `json:"texture"`
	Lightmap         string       `json:"lightmap"`
	Diffuse          [3]float32   `json:"diffuse"`
	Ambient          [3]float32   `json:"ambient"`
	TransparencyHint uint32       `json:"transparency_hint"`
	UvScroll         *MdlUvScroll `json:"uv_scroll,omitempty"`
	BoundsMin        [3]float32   `json:"bounds_min"`
	BoundsMax        [3]float32   `json:"bounds_max"`
	Radius           float32      `json:"radius"`
	AveragePoint     [3]float32   `json:"average_point"`
	SurfaceArea      float32      `json:"surface_area"`
	VertexCount      int          `json:"vertex_count"`
	FaceCount        int          `json:"face_count"`
	VertexComponents []string     `json:"vertex_components"`
}

// MdlModel holds the decoded fields of the model header, which become the extras of the glTF scene.
type MdlModel struct {
	Classification int     `json:"classification"`
	Supermodel     string  `json:"supermodel"`
	AnimationScale float32 `json:"animation_scale"`
}

type MdlResult struct {
	Gltf     []byte
	Bin      []byte
	Stats    MdlStats
	Textures []string
	Errors   []string
}

// mdlReader reads the model data of an MDL file, or an MDX file. It collects the errors of reads outside the
// data, and it counts for each byte how many structures claim it, so that bytes that no structure explains,
// and bytes that two structures share, can be counted.
type mdlReader struct {
	data   []byte
	errors []string
	claims []uint8
}

func newMdlReader(data []byte) *mdlReader {
	return &mdlReader{data: data, claims: make([]uint8, len(data))}
}

func (r *mdlReader) problem(text string) {
	r.errors = append(r.errors, text)
}

func (r *mdlReader) inBounds(offset, count int, what string) bool {
	if offset < 0 || count < 0 || offset+count > len(r.data) {
		r.problem(fmt.Sprintf("%s out of bounds (%d+%d>%d)", what, offset, count, len(r.data)))
		return false
	}
	return true
}

func (r *mdlReader) u32(offset int) (int, bool) {
	if !r.inBounds(offset, 4, "u32") {
		return 0, false
	}
	return int(binary.LittleEndian.Uint32(r.data[offset:])), true
}

func (r *mdlReader) u16(offset int) (int, bool) {
	if !r.inBounds(offset, 2, "u16") {
		return 0, false
	}
	return int(binary.LittleEndian.Uint16(r.data[offset:])), true
}

func (r *mdlReader) u8(offset int) (int, bool) {
	if !r.inBounds(offset, 1, "u8") {
		return 0, false
	}
	return int(r.data[offset]), true
}

func (r *mdlReader) f32(offset, count int) ([]float64, bool) {
	if !r.inBounds(offset, 4*count, "f32") {
		return nil, false
	}
	values := make([]float64, count)
	for i := range values {
		values[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(r.data[offset+4*i:])))
	}
	return values, true
}

// triple reads an f32[3], or zeros when it lies outside the data.
func (r *mdlReader) triple(offset int) [3]float32 {
	values, ok := r.f32(offset, 3)
	if !ok {
		return [3]float32{}
	}
	return [3]float32{float32(values[0]), float32(values[1]), float32(values[2])}
}

func (r *mdlReader) single(offset int) float32 {
	values, ok := r.f32(offset, 1)
	if !ok {
		return 0
	}
	return float32(values[0])
}

// text reads a NUL-terminated string from a field of count bytes.
func (r *mdlReader) text(offset, count int) string {
	return CString(Slice(r.data, offset, offset+count))
}

// claim marks count bytes at offset as part of a structure.
func (r *mdlReader) claim(offset, count int, what string) {
	if !r.inBounds(offset, count, what) {
		return
	}
	for i := offset; i < offset+count; i++ {
		r.claims[i] = min(r.claims[i], math.MaxUint8-1) + 1
	}
}

// claimString marks a NUL-terminated string at offset, with its NUL byte, and returns it.
func (r *mdlReader) claimString(offset int, what string) string {
	if !r.inBounds(offset, 1, what) {
		return ""
	}
	end := offset
	for end < len(r.data) && r.data[end] != 0 {
		end++
	}
	r.claim(offset, min(end+1, len(r.data))-offset, what)
	return string(r.data[offset:end])
}

// array reads the array descriptor at offset: the offset of the array, its count and its capacity. The
// count must equal the capacity. The array of count elements of elementSize bytes is claimed.
func (r *mdlReader) array(offset, elementSize int, what string) (int, int) {
	start, _ := r.u32(offset)
	count, _ := r.u32(offset + 4)
	capacity, _ := r.u32(offset + 8)
	if count != capacity {
		r.problem(fmt.Sprintf("%s has count %d and capacity %d", what, count, capacity))
	}
	if count > 0 {
		r.claim(start, elementSize*count, what)
	}
	return start, count
}

// unclaimed returns the number of bytes that no structure claims and the number that more than one claims.
func (r *mdlReader) unclaimed() (int, int) {
	free, shared := 0, 0
	for _, claims := range r.claims {
		if claims == 0 {
			free++
		}
		if claims > 1 {
			shared++
		}
	}
	return free, shared
}

// ConvertMdl converts a model to the JSON text of a glTF file, whose binary buffer is stored at binUri, and
// that buffer. It also returns its statistics, its texture names and its errors. The label names the scene.
// texturePath gives the image URI of a texture name, relative to the glTF file, or "" when the texture is not
// known; it may be nil. Every node of the tree becomes a glTF node in depth-first order, with its position
// and rotation relative to its parent. Meshes whose render byte is 0 get no geometry, because the game does
// not appear to draw them, but their headers are decoded. The model's animations become glTF animations, and
// its skinned meshes glTF skins.
func ConvertMdl(mdl, mdx []byte, label, binUri string, texturePath func(string) string) MdlResult {
	reader := newMdlReader(mdl[min(fileHeaderSize, len(mdl)):])
	mdxReader := newMdlReader(mdx)
	var stats MdlStats

	inBounds := reader.inBounds
	u32 := reader.u32
	u16 := reader.u16
	u8 := reader.u8
	f32 := reader.f32
	triple := reader.triple
	single := reader.single
	readString := reader.text
	mdxFloat := func(offset int) float64 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(mdx[offset:])))
	}

	// The model header. Its first two words are constant, and its geometry type at 0x4C is 2.
	reader.claim(0, modelHeaderSize, "model header")
	address0, _ := u32(0)
	address1, _ := u32(4)
	geometryType, _ := u8(0x4c)
	if [2]int{address0, address1} != modelClassAddresses || geometryType != modelType {
		reader.problem(fmt.Sprintf("model header starts with %#x %#x and has type %d", address0, address1, geometryType))
	}

	root, _ := u32(0x28)
	stats.HdrNodes = "?"
	if hdrNodes, ok := u32(0x2c); ok {
		stats.HdrNodes = strconv.Itoa(hdrNodes)
	}

	nameTableOffset, nameCount := reader.array(0xb8, 4, "name table")
	names := make([]string, 0, nameCount)
	for i := range nameCount {
		nameOffset, _ := u32(nameTableOffset + 4*i)
		names = append(names, reader.claimString(nameOffset, "node name"))
	}

	doc := newGltf(label)
	classification, _ := u8(0x50)
	doc.Scenes[0].Extras = MdlModel{
		Classification: classification,
		Supermodel:     readString(0x88, modelNameLength),
		AnimationScale: single(0x84),
	}
	materials := map[string]int{}
	textures := map[string]bool{}
	seen := map[int]bool{}

	// targets maps each node number to the nodes of the model tree with that number, and namedNodes each name
	// table index to the first glTF node with that name. locals and parents hold the transform
	// of each glTF node relative to its parent, and the parent's index. glTF node 0 turns Z-up into Y-up and is
	// the identity in model space.
	targets := map[int][]mdlTarget{}
	namedNodes := map[int]int{}
	locals := []mdlTransform{identityTransform}
	parents := []int{-1}
	type pendingSkin struct {
		node   int
		mesh   int
		skin   mdlSkin
		name   string
		extras *MdlNodeExtras
	}
	var skins []pendingSkin

	mesh := func(header int, name string, flags int, node int, extras *MdlNodeExtras) *int {
		problem := func(text string) {
			reader.problem(text + " in " + name)
		}

		kind := "trimesh"
		class := "trimesh"
		switch {
		case flags&nodeFlagSkin != 0:
			kind, class = "skin", "skin"
		case flags&nodeFlagDangly != 0:
			kind, class = "dangly", "dangly"
		case flags&nodeFlagAabb != 0:
			kind = "aabb"
		case flags&nodeFlagLightsaber != 0:
			kind = "lightsaber"
		}

		reader.claim(header, meshHeaderSize, "mesh header")
		address0, _ := u32(header)
		address1, _ := u32(header + 4)
		if [2]int{address0, address1} != meshClassAddresses[class] {
			problem(fmt.Sprintf("class addresses %#x %#x for a %s mesh", address0, address1, kind))
		}

		// Each array descriptor is an offset, a count and a capacity, and the count equals the capacity.
		faceOffset, faceCount := reader.array(header+meshFaceArray, faceSize, "faces")
		indexCounts, indexLists := reader.array(header+meshIndexCounts, indexArrayEntrySize, "index counts")
		indexOffsets, _ := reader.array(header+meshIndexOffsets, indexArrayEntrySize, "index offsets")
		reader.array(header+meshCounters, indexArrayEntrySize, "counters")

		texture := readString(header+meshTexture, textureNameLength)
		stride, _ := u32(header + meshVertexStride)
		vertexFlags, _ := u32(header + meshVertexFlags)
		vertexCount, _ := u16(header + meshVertexCount)
		uvSetCount, _ := u16(header + meshUvSetCount)
		mdxOffset, _ := u32(header + meshMdxOffset)
		vertexOffset, ok := u32(header + meshVertexOffset)
		if !ok {
			return nil
		}
		render, _ := u8(header + meshRender)
		lightmapped, _ := u8(header + meshLightmapped)
		shadow, _ := u8(header + meshShadow)
		transparencyHint, _ := u32(header + meshTransparencyHint)
		scrollEnabled, _ := u32(header + meshUvScrollEnabled)

		// The vertex flags name the components of an MDX vertex record. Each set bit goes with a component
		// offset, and the record size is the sum of the component sizes.
		componentOffsets := map[string]int{}
		var componentNames []string
		knownBits := 0
		recordSize := 0
		for _, component := range vertexComponents {
			offset, _ := u32(header + component.offset)
			knownBits |= component.bit
			present := vertexFlags&component.bit != 0
			if present != (offset != none) {
				problem(fmt.Sprintf("vertex flags %#x disagree with the %s offset", vertexFlags, component.name))
			}
			if present {
				componentOffsets[component.name] = offset
				componentNames = append(componentNames, component.name)
				recordSize += component.size
			}
		}
		if kind == "skin" {
			recordSize += skinVertexExtra
		}
		if vertexFlags&^knownBits != 0 {
			problem(fmt.Sprintf("unknown vertex flags %#x", vertexFlags))
		}
		if vertexFlags != 0 && stride != recordSize {
			problem(fmt.Sprintf("vertex record size %d, expected %d", stride, recordSize))
		}
		uvSets := 0
		for _, uv := range []string{"uv1", "uv2"} {
			if _, ok := componentOffsets[uv]; ok {
				uvSets++
			}
		}
		if kind != "lightsaber" && uvSetCount != uvSets {
			problem(fmt.Sprintf("UV set count %d with %d UV sets", uvSetCount, uvSets))
		}

		if componentNames == nil {
			componentNames = []string{}
		}
		decoded := MdlMesh{
			Name:             name,
			Kind:             kind,
			Render:           render != 0,
			Shadow:           shadow != 0,
			Lightmapped:      lightmapped != 0,
			Texture:          texture,
			Lightmap:         readString(header+meshLightmap, textureNameLength),
			Diffuse:          triple(header + meshDiffuse),
			Ambient:          triple(header + meshAmbient),
			TransparencyHint: uint32(transparencyHint),
			BoundsMin:        triple(header + meshBoundsMin),
			BoundsMax:        triple(header + meshBoundsMax),
			Radius:           single(header + meshRadius),
			AveragePoint:     triple(header + meshAveragePoint),
			SurfaceArea:      single(header + meshSurfaceArea),
			VertexCount:      vertexCount,
			FaceCount:        faceCount,
			VertexComponents: componentNames,
		}
		if scrollEnabled != 0 {
			scroll := triple(header + meshUvScroll)
			decoded.UvScroll = &MdlUvScroll{
				Direction:   [2]float32{scroll[0], scroll[1]},
				Jitter:      scroll[2],
				JitterSpeed: single(header + meshUvScroll + 12),
			}
		}
		extras.Mesh = &decoded

		if !inBounds(faceOffset, faceSize*faceCount, "faces "+name) ||
			!inBounds(vertexOffset, vertexSize*vertexCount, "verts "+name) {
			return nil
		}
		reader.claim(vertexOffset, vertexSize*vertexCount, "vertices")

		verts := make([]vec3, 0, vertexCount)
		for i := range vertexCount {
			values, _ := f32(vertexOffset+vertexSize*i, 3)
			verts = append(verts, vec3{values[0], values[1], values[2]})
		}

		// The bounding box encloses the vertices and the origin of the node.
		for _, vertex := range append([]vec3{{0, 0, 0}}, verts...) {
			for axis := range 3 {
				if vertex[axis] < float64(decoded.BoundsMin[axis])-boundsTolerance ||
					vertex[axis] > float64(decoded.BoundsMax[axis])+boundsTolerance {
					problem("vertex outside the bounding box")
					break
				}
			}
		}

		// The radius is the largest distance of a vertex from the average point.
		if len(verts) > 0 {
			farthest := 0.0
			for _, vertex := range verts {
				distance := 0.0
				for axis := range 3 {
					delta := vertex[axis] - float64(decoded.AveragePoint[axis])
					distance += delta * delta
				}
				farthest = max(farthest, math.Sqrt(distance))
			}
			stats.RadiusChecked++
			if math.Abs(farthest-float64(decoded.Radius)) < radiusTolerance {
				stats.RadiusMatching++
			}
		}

		// The faces are read once. The vertex indices must be valid for everything that follows.
		type face struct {
			corners  [3]int
			normal   []float64
			distance float64
			material int
			adjacent [3]int
		}
		faces := make([]face, 0, faceCount)
		faceCorners := make([][3]int, 0, faceCount)
		for i := range faceCount {
			entry := faceOffset + faceSize*i
			var current face
			valid := true
			for corner := range cornersPerFace {
				index, _ := u16(entry + faceVerticesOffset + faceIndexSize*corner)
				current.corners[corner] = index
				valid = valid && index < vertexCount

				adjacent, _ := u16(entry + faceAdjacentOffset + faceIndexSize*corner)
				current.adjacent[corner] = adjacent
			}
			if !valid {
				problem(fmt.Sprintf("face index >= %d", vertexCount))
				return nil
			}

			current.normal, _ = f32(entry, 3)
			distance, _ := f32(entry+facePlaneOffset, 1)
			current.distance = distance[0]
			current.material, _ = u32(entry + faceMaterialOffset)
			faces = append(faces, current)
			faceCorners = append(faceCorners, current.corners)
		}

		// The index list repeats the vertex indices of the faces, three per face. The 79 meshes without an
		// index list are lightsaber meshes and meshes without vertices.
		if indexLists == 1 {
			indexCount, _ := u32(indexCounts)
			listOffset, _ := u32(indexOffsets)
			if indexCount != cornersPerFace*faceCount {
				problem(fmt.Sprintf("index count %d for %d faces", indexCount, faceCount))
			} else if inBounds(listOffset, faceIndexSize*indexCount, "index list "+name) {
				reader.claim(listOffset, faceIndexSize*indexCount, "index list")
				for i, current := range faces {
					for corner := range cornersPerFace {
						index, _ := u16(listOffset + faceIndexSize*(cornersPerFace*i+corner))
						if index != current.corners[corner] {
							problem("index list differs from the faces")
							break
						}
					}
				}
			}
		}

		// The header of the mesh subclass follows the mesh header.
		subclass := header + meshHeaderSize
		var skin mdlSkin
		var saberUvs [][2]float64
		var saberNormals []vec3
		switch kind {
		case "skin":
			skin = readSkin(reader, subclass, name)
			if skin.weightOffset != recordSize-skinVertexExtra || skin.indexOffset != skin.weightOffset+skinWeightsSize {
				problem(fmt.Sprintf("bone weights at %d and bone indices at %d in a %d-byte record", skin.weightOffset, skin.indexOffset, recordSize))
			}
		case "dangly":
			var constraints []float32
			extras.Dangly, constraints = readDangly(reader, subclass, verts, name)
			if len(constraints) > 0 {
				accessor := gltfFloats(doc, constraints, 1, false, gltfNoTarget)
				extras.Dangly.Constraints = &accessor
			}
		case "aabb":
			tree, consistent := readAabb(reader, subclass, verts, faceCorners, name)
			stats.AabbChecked++
			if consistent {
				stats.AabbMatching++
			}
			if len(tree) > 0 {
				var boxes []float32
				var links []byte
				for _, node := range tree {
					boxes = append(boxes, node.low[:]...)
					boxes = append(boxes, node.high[:]...)
					for _, value := range []int{node.left, node.right, node.face, node.plane} {
						links = binary.LittleEndian.AppendUint32(links, uint32(value))
					}
				}
				extras.Aabb = &MdlAabb{
					Boxes: gltfFloats(doc, boxes, 3, false, gltfNoTarget),
					Nodes: gltfAccessorAt(doc, gltfView(doc, links, gltfNoTarget), 0, GltfUnsignedInt, len(tree), "VEC4"),
				}
			}
		case "lightsaber":
			saberUvs, saberNormals = readLightsaber(reader, subclass, verts, name)
		}

		// The surface area is the sum of the face areas in plain and AABB meshes, and 0 in skinned, dangly and
		// lightsaber meshes. Each stored normal is checked against the winding order, and each plane distance
		// against the first vertex.
		edgeFaces := map[[2]vec3][]int{}
		edgeKey := func(a, b vec3) [2]vec3 {
			if slices.Compare(a[:], b[:]) > 0 {
				a, b = b, a
			}
			return [2]vec3{a, b}
		}
		area := 0.0
		for i, current := range faces {
			corner0, corner1, corner2 := verts[current.corners[0]], verts[current.corners[1]], verts[current.corners[2]]
			var edge1, edge2 vec3
			for axis := range 3 {
				edge1[axis] = corner1[axis] - corner0[axis]
				edge2[axis] = corner2[axis] - corner0[axis]
			}
			cross := vec3{
				edge1[1]*edge2[2] - edge1[2]*edge2[1],
				edge1[2]*edge2[0] - edge1[0]*edge2[2],
				edge1[0]*edge2[1] - edge1[1]*edge2[0],
			}
			crossLength := math.Sqrt(cross[0]*cross[0] + cross[1]*cross[1] + cross[2]*cross[2])
			area += crossLength / 2

			normal := current.normal
			if crossLength > minCrossLength {
				stats.NormalsChecked++
				if (cross[0]*normal[0]+cross[1]*normal[1]+cross[2]*normal[2])/crossLength > minNormalCosine {
					stats.NormalsMatching++
				}
			}

			stats.PlanesChecked++
			if math.Abs(normal[0]*corner0[0]+normal[1]*corner0[1]+normal[2]*corner0[2]+current.distance) < planeTolerance {
				stats.PlanesMatching++
			}

			for corner := range cornersPerFace {
				key := edgeKey(verts[current.corners[corner]], verts[current.corners[(corner+1)%cornersPerFace]])
				edgeFaces[key] = append(edgeFaces[key], i)
			}
		}
		if kind != "trimesh" && kind != "aabb" {
			area = 0
		}
		stats.AreaChecked++
		if math.Abs(area-float64(decoded.SurfaceArea)) <= areaRelativeTolerance*max(1, area) {
			stats.AreaMatching++
		}

		// Adjacent face e shares the edge from corner e to corner e + 1, compared by vertex position.
		for i, current := range faces {
			for corner, adjacent := range current.adjacent {
				if adjacent == noAdjacentFace {
					continue
				}
				if adjacent >= faceCount {
					stats.AdjacentOutside++
					continue
				}
				key := edgeKey(verts[current.corners[corner]], verts[current.corners[(corner+1)%cornersPerFace]])
				stats.AdjacentChecked++
				if adjacent != i && slices.Contains(edgeFaces[key], adjacent) {
					stats.AdjacentMatching++
				}
			}
		}

		hasMdx := stride > 0 && vertexCount > 0 && len(mdx) > 0
		if hasMdx && mdxOffset+stride*vertexCount > len(mdx) {
			problem("mdx range")
			hasMdx = false
		}
		if hasMdx {
			mdxReader.claim(mdxOffset, stride*vertexCount, "vertex records")

			// One terminator record follows the records of each mesh. Its position holds 1,000,000 three
			// times in skinned meshes and 10,000,000 three times in the others.
			terminator := mdxOffset + stride*vertexCount
			marker := float64(plainTerminator)
			if kind == "skin" {
				marker = skinTerminator
			}
			if terminator+stride > len(mdx) || mdxFloat(terminator) != marker || mdxFloat(terminator+4) != marker ||
				mdxFloat(terminator+8) != marker {
				problem("no terminator record")
			} else {
				mdxReader.claim(terminator, stride, "terminator record")
			}
		}
		if render == 0 {
			stats.Hidden++
			return nil
		}

		stats.Meshes++
		stats.Verts += vertexCount
		stats.Faces += faceCount
		if faceCount == 0 {
			return nil
		}

		// The positions come from the MDL and must match their MDX copies. The normals and texture
		// coordinates come from the MDX, or for a lightsaber mesh from the arrays of its header. glTF puts
		// v = 0 at the top of an image and the game at the bottom (see "Pixel data layout" in README.md), so
		// v becomes 1 - v.
		positionOffset, hasPosition := componentOffsets["position"]
		normalOffset, normalPresent := componentOffsets["normal"]
		hasNormal := normalPresent && hasMdx
		// A zero normal becomes the normalized sum of the stored normals of the faces that use the vertex.
		faceNormalSums := make([]vec3, vertexCount)
		for _, current := range faces {
			for _, corner := range current.corners {
				for axis := range 3 {
					faceNormalSums[corner][axis] += current.normal[axis]
				}
			}
		}
		length := func(vector vec3) float64 {
			return math.Sqrt(vector[0]*vector[0] + vector[1]*vector[1] + vector[2]*vector[2])
		}

		positions := make([]float32, 0, 3*vertexCount)
		var normals []float32
		uvSetsWritten := map[string][]float32{}
		for i, vertex := range verts {
			record := mdxOffset + stride*i
			if hasMdx && hasPosition {
				for axis := range 3 {
					if math.Abs(vertex[axis]-mdxFloat(record+positionOffset+4*axis)) > positionTolerance {
						stats.Mismatch++
						break
					}
				}
			}
			positions = append(positions, float32(vertex[0]), float32(vertex[1]), float32(vertex[2]))

			if hasNormal {
				normal := vec3{mdxFloat(record + normalOffset), mdxFloat(record + normalOffset + 4), mdxFloat(record + normalOffset + 8)}
				if length(normal) < zeroLength {
					stats.NormalsReplaced++
					normal = upAxis
					if sum := faceNormalSums[i]; length(sum) >= zeroLength {
						normal = vec3{sum[0] / length(sum), sum[1] / length(sum), sum[2] / length(sum)}
					}
				}
				hasNormal = math.Abs(length(normal)-1) < unitTolerance
				normals = append(normals, float32(normal[0]), float32(normal[1]), float32(normal[2]))
			}

			for _, uv := range []string{"uv1", "uv2"} {
				if offset, present := componentOffsets[uv]; present && hasMdx {
					uvSetsWritten[uv] = append(uvSetsWritten[uv], float32(mdxFloat(record+offset)), float32(1-mdxFloat(record+offset+4)))
				}
			}
		}
		if normalPresent && hasMdx && !hasNormal {
			stats.NormalsOmitted++
		}
		if saberUvs != nil {
			hasNormal = true
			for i, uv := range saberUvs {
				normal := saberNormals[i]
				hasNormal = hasNormal && math.Abs(length(normal)-1) < unitTolerance
				normals = append(normals, float32(normal[0]), float32(normal[1]), float32(normal[2]))
				uvSetsWritten["uv1"] = append(uvSetsWritten["uv1"], float32(uv[0]), float32(1-uv[1]))
			}
		}

		attributes := map[string]int{"POSITION": gltfFloats(doc, positions, 3, true, gltfArrayBuffer)}
		if hasNormal {
			attributes["NORMAL"] = gltfFloats(doc, normals, 3, false, gltfArrayBuffer)
		}
		// glTF numbers texture coordinate sets from 0 without gaps, so a mesh with only the second set
		// writes it as TEXCOORD_0.
		uvSet := 0
		for _, uv := range []string{"uv1", "uv2"} {
			if values, present := uvSetsWritten[uv]; present {
				attributes[fmt.Sprintf("TEXCOORD_%d", uvSet)] = gltfFloats(doc, values, 2, false, gltfArrayBuffer)
				uvSet++
			}
		}

		// The 32 extra bytes of a skinned vertex record hold four f32 bone weights and four f32 local bone
		// indices. An unused slot has weight 0 and index -1, and becomes joint 0. The stored weights sum to 1
		// only within about 1e-6, and glTF requires a closer sum, so they are divided by their sum and the
		// largest becomes 1 minus the others.
		skinned := kind == "skin" && hasMdx && len(skin.bones) > 0
		if skinned {
			var joints []byte
			weights := make([]float32, 0, skinSlots*vertexCount)
			for i := range vertexCount {
				record := mdxOffset + stride*i
				var slots [skinSlots]float64
				sum := 0.0
				largest := 0
				for slot := range skinSlots {
					weight := mdxFloat(record + skin.weightOffset + 4*slot)
					bone := mdxFloat(record + skin.indexOffset + 4*slot)
					joint := 0
					switch {
					case weight == 0 && bone == -1:
					case weight != 0 && bone == math.Trunc(bone) && bone >= 0 && int(bone) < len(skin.bones):
						joint = int(bone)
					default:
						stats.WeightsBad++
					}
					slots[slot] = weight
					sum += weight
					if weight > slots[largest] {
						largest = slot
					}
					joints = binary.LittleEndian.AppendUint16(joints, uint16(joint))
				}
				if math.Abs(sum-1) > weightTolerance {
					stats.WeightsBad++
				}

				var normalized [skinSlots]float32
				others := 0.0
				for slot, weight := range slots {
					if slot != largest {
						normalized[slot] = float32(weight / sum)
						others += float64(normalized[slot])
					}
				}
				normalized[largest] = float32(1 - others)
				weights = append(weights, normalized[:]...)
			}
			attributes["JOINTS_0"] = gltfAccessorAt(doc, gltfView(doc, joints, gltfArrayBuffer), 0, GltfUnsignedShort, vertexCount, "VEC4")
			attributes["WEIGHTS_0"] = gltfFloats(doc, weights, skinSlots, false, gltfArrayBuffer)
		}

		indices := make([]int, 0, cornersPerFace*faceCount)
		var faceMaterials, faceAdjacent []byte
		for _, current := range faces {
			indices = append(indices, current.corners[:]...)
			faceMaterials = binary.LittleEndian.AppendUint32(faceMaterials, uint32(current.material))
			for _, adjacent := range current.adjacent {
				faceAdjacent = binary.LittleEndian.AppendUint16(faceAdjacent, uint16(adjacent))
			}
		}
		indexView := gltfIndexView(doc, indices)
		primitive := gltfPrimitive{
			Attributes: attributes,
			Indices:    gltfAccessorAt(doc, indexView, 0, GltfUnsignedInt, len(indices), "SCALAR"),
		}

		// glTF forbids a texture on a primitive without texture coordinates, so such a mesh gets a material of
		// the same name without the texture.
		material := strings.ToLower(texture)
		if texture != "" && material != "null" {
			_, hasUv := attributes["TEXCOORD_0"]
			key := fmt.Sprintf("%s %t", material, hasUv)
			index, known := materials[key]
			if !known {
				uri := ""
				if texturePath != nil && hasUv {
					uri = texturePath(material)
				}
				index = gltfMaterialAdd(doc, material, uri, nil)
				materials[key] = index
			}
			primitive.Material = &index
			textures[material] = true
		}

		materialAccessor := gltfAccessorAt(doc, gltfView(doc, faceMaterials, gltfNoTarget), 0, GltfUnsignedInt, faceCount, "SCALAR")
		adjacentAccessor := gltfAccessorAt(doc, gltfView(doc, faceAdjacent, gltfNoTarget), 0, GltfUnsignedShort, faceCount, "VEC3")
		extras.FaceMaterials = &materialAccessor
		extras.FaceAdjacent = &adjacentAccessor

		doc.Meshes = append(doc.Meshes, gltfMesh{Name: name, Primitives: []gltfPrimitive{primitive}})
		index := len(doc.Meshes) - 1
		if skinned {
			skins = append(skins, pendingSkin{node: node, mesh: index, skin: skin, name: name, extras: extras})
		}
		return &index
	}

	// Each node points at the model header (offset 0) and at its parent node. The position and orientation
	// controllers of a node repeat its header values, and its other controllers hold static values.
	var node func(offset, parentOffset, parentNode, depth int) int
	node = func(offset, parentOffset, parentNode, depth int) int {
		if seen[offset] {
			reader.problem(fmt.Sprintf("node loop at %d", offset))
			return -1
		}
		if depth > maxNodeDepth {
			reader.problem("depth")
			return -1
		}

		seen[offset] = true
		stats.Nodes++
		reader.claim(offset, nodeHeaderSize, "node")

		flags, _ := u16(offset)
		number, _ := u16(offset + 2)
		nameIndex, _ := u16(offset + 4)
		owner, _ := u32(offset + nodeOwnerField)
		parent, _ := u32(offset + nodeParentField)
		pos, posOk := f32(offset+0x10, 3)
		rotation, rotationOk := f32(offset+0x1c, 4)
		if !posOk || !rotationOk {
			return -1
		}
		if owner != 0 || parent != parentOffset {
			reader.problem(fmt.Sprintf("node at %d names the model at %d and the parent at %d", offset, owner, parent))
		}
		if flags&^nodeFlagsKnown != 0 {
			reader.problem(fmt.Sprintf("unknown node flags %#x", flags))
		}

		// The game stores the rotation as w, x, y, z and glTF as x, y, z, w.
		name := fmt.Sprintf("node%d", offset)
		if nameIndex < len(names) {
			name = names[nameIndex]
		}
		index := len(doc.Nodes)
		doc.Nodes = append(doc.Nodes, GltfNode{
			Name:        name,
			Translation: &[3]float32{float32(pos[0]), float32(pos[1]), float32(pos[2])},
			Rotation:    &[4]float32{float32(rotation[1]), float32(rotation[2]), float32(rotation[3]), float32(rotation[0])},
		})
		translation := vec3{pos[0], pos[1], pos[2]}
		locals = append(locals, mdlTransform{rotation: [4]float64{rotation[1], rotation[2], rotation[3], rotation[0]}, translation: translation})
		parents = append(parents, parentNode)
		targets[number] = append(targets[number], mdlTarget{node: index, flags: flags, name: name, translation: translation})
		if _, known := namedNodes[nameIndex]; !known {
			namedNodes[nameIndex] = index
		}

		extras := &MdlNodeExtras{}
		for _, controller := range readControllers(reader, offset, name) {
			if len(controller.rows) != 1 {
				reader.problem(fmt.Sprintf("controller %d of %s has %d keys", controller.kind, name, len(controller.rows)))
				continue
			}

			var values []float32
			for _, value := range controller.rows[0] {
				values = append(values, float32(value))
			}
			switch controller.kind {
			case controllerPosition:
				stats.ControllersChecked++
				if slices.Equal(values, doc.Nodes[index].Translation[:]) {
					stats.ControllersMatching++
				}
			case controllerOrientation:
				// The header rotation and the controller differ by rounding in some nodes.
				stats.ControllersChecked++
				stored := mdlTransform{rotation: [4]float64(controller.rows[0])}
				if angle, _ := transformDistance(stored, locals[index]); angle < bindAngleTolerance {
					stats.ControllersMatching++
				}
			default:
				extras.Controllers = append(extras.Controllers, MdlStatic{Name: controllerName(flags, controller.kind), Values: values})
			}
		}

		classHeader := offset + nodeHeaderSize
		if flags&nodeFlagLight != 0 {
			extras.Light = readLight(reader, classHeader)
		}
		if flags&nodeFlagEmitter != 0 {
			extras.Emitter = readEmitter(reader, classHeader)
		}
		if flags&nodeFlagReference != 0 {
			extras.Reference = readReference(reader, classHeader)
		}
		if flags&nodeFlagMesh != 0 {
			doc.Nodes[index].Mesh = mesh(classHeader, name, flags, index, extras)
		}
		if extras.Controllers != nil || extras.Light != nil || extras.Emitter != nil || extras.Reference != nil || extras.Mesh != nil {
			doc.Nodes[index].Extras = extras
		}

		childOffset, childCount := reader.array(offset+nodeChildArray, 4, "children")
		var children []int
		for i := range childCount {
			if child, ok := u32(childOffset + 4*i); ok {
				if childIndex := node(child, offset, index, depth+1); childIndex >= 0 {
					children = append(children, childIndex)
				}
			}
		}
		doc.Nodes[index].Children = children
		return index
	}

	rootIndex := node(root, 0, 0, 0)
	if rootIndex >= 0 {
		doc.Nodes[0].Children = []int{rootIndex}
	}

	// The transform of each node in model space. A parent precedes its children.
	globals := make([]mdlTransform, len(locals))
	for i, local := range locals {
		globals[i] = local
		if parents[i] >= 0 {
			globals[i] = transformCompose(globals[parents[i]], local)
		}
	}

	// glTF ignores the transform of the node of a skinned mesh and recommends that it be a root node, so each
	// skinned mesh moves to a new root node. Its joints are the nodes of its local bones. The inverse bind
	// matrix of a joint maps the space of the mesh's node into the space of the joint in the rest pose, so
	// the mesh appears where its node is. The stored inverse bind poses are compared with these.
	for _, pending := range skins {
		var joints []int
		var matrices []float32
		meshGlobal := globals[pending.node]
		for _, entry := range pending.skin.bones {
			joint, known := namedNodes[entry]
			if !known {
				reader.problem(fmt.Sprintf("bone %d of %s is no node", entry, pending.name))
				break
			}
			bind := transformCompose(transformInverse(globals[joint]), meshGlobal)
			stored := mdlTransform{rotation: pending.skin.rotations[entry], translation: pending.skin.translations[entry]}
			angle, distance := transformDistance(bind, stored)
			stats.BindChecked++
			if angle < bindAngleTolerance && distance < bindDistanceTolerance {
				stats.BindMatching++
			}
			joints = append(joints, joint)
			matrices = append(matrices, transformMatrix(bind)...)
		}
		if len(joints) != len(pending.skin.bones) {
			continue
		}

		doc.Skins = append(doc.Skins, gltfSkin{
			Name:                pending.name,
			InverseBindMatrices: gltfFloats(doc, matrices, matrixSize, false, gltfNoTarget),
			Joints:              joints,
		})
		skinIndex := len(doc.Skins) - 1
		meshIndex := pending.mesh
		doc.Nodes = append(doc.Nodes, GltfNode{Name: pending.name + skinNodeSuffix, Mesh: &meshIndex, Skin: &skinIndex})
		skinned := len(doc.Nodes) - 1
		doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, skinned)
		doc.Nodes[pending.node].Mesh = nil
		pending.extras.Skin = &MdlSkinExtras{Node: skinned}
		stats.Skins++
		stats.Bones += len(joints)
	}

	if rootIndex >= 0 {
		readAnimations(reader, doc, &stats, targets, names, mdlTarget{node: rootIndex, translation: locals[rootIndex].translation})
	}

	stats.Unclaimed, stats.Shared = reader.unclaimed()
	// Between the records of two meshes lie as many bytes as the end offset of the first, modulo 16. These
	// padding bytes are counted apart from the other bytes that no record covers.
	start := -1
	for i := range len(mdx) + 1 {
		free := i < len(mdx) && mdxReader.claims[i] == 0
		if free && start < 0 {
			start = i
		}
		if !free && start >= 0 {
			if i < len(mdx) && i-start == start%mdxAlignment {
				stats.MdxPadding += i - start
			} else {
				stats.MdxUnclaimed += i - start
			}
			start = -1
		}
	}
	errors := append(reader.errors, mdxReader.errors...)
	text, err := gltfEncode(doc, binUri)
	if err != nil {
		errors = append(errors, err.Error())
	}

	sorted := make([]string, 0, len(textures))
	for texture := range textures {
		sorted = append(sorted, texture)
	}
	slices.Sort(sorted)

	return MdlResult{
		Gltf:     text,
		Bin:      doc.Bin,
		Stats:    stats,
		Textures: sorted,
		Errors:   errors,
	}
}

func ReportMdl(label string, result MdlResult) string {
	stats := result.Stats
	return fmt.Sprintf("%s nodes=%d meshes=%d hidden=%d verts=%d faces=%d mdl/mdx_mismatch=%d hdrnodes=%s "+
		"normals_ok=%d/%d planes_ok=%d/%d adjacent_ok=%d/%d adjacent_outside=%d radius_ok=%d/%d area_ok=%d/%d "+
		"normals_replaced=%d normals_omitted=%d controllers_ok=%d/%d animations=%d animation_nodes=%d keys=%d "+
		"channels=%d tracks=%d unmatched_animation_nodes=%d constant_channels=%d duplicate_tracks=%d skins=%d bones=%d bind_ok=%d/%d "+
		"weights_bad=%d aabb_ok=%d/%d unclaimed=%d shared=%d mdx_padding=%d mdx_unclaimed=%d %s",
		label, stats.Nodes, stats.Meshes, stats.Hidden, stats.Verts, stats.Faces, stats.Mismatch, stats.HdrNodes,
		stats.NormalsMatching, stats.NormalsChecked, stats.PlanesMatching, stats.PlanesChecked,
		stats.AdjacentMatching, stats.AdjacentChecked, stats.AdjacentOutside, stats.RadiusMatching,
		stats.RadiusChecked, stats.AreaMatching, stats.AreaChecked, stats.NormalsReplaced, stats.NormalsOmitted,
		stats.ControllersMatching, stats.ControllersChecked, stats.Animations, stats.AnimationNodes, stats.Keys,
		stats.Channels, stats.Tracks, stats.UnmatchedAnimationNodes, stats.ConstantChannels, stats.DuplicateTracks, stats.Skins, stats.Bones,
		stats.BindMatching, stats.BindChecked, stats.WeightsBad, stats.AabbMatching, stats.AabbChecked, stats.Unclaimed, stats.Shared, stats.MdxPadding, stats.MdxUnclaimed,
		Verdict(result.Errors[:min(5, len(result.Errors))]...))
}
