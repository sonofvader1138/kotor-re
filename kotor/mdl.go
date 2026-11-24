// This file converts MDL models with their MDX vertex data to Wavefront OBJ, together with the decoded mesh
// headers as JSON and the face fields as CSV. Layout: see README.md. MDL offsets are relative to file offset 12.

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
	nameLength        = 64
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

// The vertex records of a skinned mesh carry 32 more bytes, which the skin header describes.
const skinVertexExtra = 32

// MdlFacesCsvHeader is the first line of a face CSV file. Each row gives a face of a mesh in the OBJ file, its
// material and its three adjacent faces, with -1 for none.
const MdlFacesCsvHeader = "mesh,face,material,adjacent_0,adjacent_1,adjacent_2"

// Deeper node trees are treated as corrupt.
const maxNodeDepth = 200

// The MDX copy of a vertex position must match the MDL copy within this distance.
const positionTolerance = 1e-5

// Faces with a smaller cross product are degenerate and their normal is not checked.
const minCrossLength = 1e-9

// A stored face normal agrees with the winding when the cosine between them exceeds this value.
const minNormalCosine = 0.99

// Tolerances for the derived mesh header values: the plane distance, the radius and the surface area.
const (
	planeTolerance        = 1e-3
	radiusTolerance       = 1e-3
	areaRelativeTolerance = 1e-3
	boundsTolerance       = 1e-4
)

type vec3 = [3]float64

// quat is [w, x, y, z].
type quat = [4]float64

// qrot rotates a vector by a unit quaternion.
func qrot(q quat, v vec3) vec3 {
	w, x, y, z := q[0], q[1], q[2], q[3]
	vx, vy, vz := v[0], v[1], v[2]
	tx := 2 * (y*vz - z*vy)
	ty := 2 * (z*vx - x*vz)
	tz := 2 * (x*vy - y*vx)
	return vec3{
		vx + w*tx + y*tz - z*ty,
		vy + w*ty + z*tx - x*tz,
		vz + w*tz + x*ty - y*tx,
	}
}

func qmul(a, b quat) quat {
	return quat{
		a[0]*b[0] - a[1]*b[1] - a[2]*b[2] - a[3]*b[3],
		a[0]*b[1] + a[1]*b[0] + a[2]*b[3] - a[3]*b[2],
		a[0]*b[2] - a[1]*b[3] + a[2]*b[0] + a[3]*b[1],
		a[0]*b[3] + a[1]*b[2] - a[2]*b[1] + a[3]*b[0],
	}
}

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

	// HdrNodes is the node count field of the header, as text: "?" when it lies outside the file.
	HdrNodes string
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

type MdlResult struct {
	Obj      string
	FacesCsv string
	Meshes   []MdlMesh
	Stats    MdlStats
	Textures []string
	Errors   []string
}

// ConvertMdl converts a model and returns the OBJ text, the face CSV, the decoded mesh headers, its statistics,
// its texture names and its errors. When mtllib is not empty, the OBJ names it as its material library. Meshes
// whose render byte is 0 are left out of the OBJ and the CSV, because the game does not appear to draw them,
// but their headers are decoded.
func ConvertMdl(mdl, mdx []byte, label, mtllib string) MdlResult {
	data := mdl[min(fileHeaderSize, len(mdl)):]
	length := len(data)
	var errors []string
	var stats MdlStats

	inBounds := func(offset, count int, what string) bool {
		if offset < 0 || offset+count > length {
			errors = append(errors, fmt.Sprintf("%s out of bounds (%d+%d>%d)", what, offset, count, length))
			return false
		}
		return true
	}
	u32 := func(offset int) (int, bool) {
		if !inBounds(offset, 4, "u32") {
			return 0, false
		}
		return int(binary.LittleEndian.Uint32(data[offset:])), true
	}
	u16 := func(offset int) (int, bool) {
		if !inBounds(offset, 2, "u16") {
			return 0, false
		}
		return int(binary.LittleEndian.Uint16(data[offset:])), true
	}
	u8 := func(offset int) (int, bool) {
		if !inBounds(offset, 1, "u8") {
			return 0, false
		}
		return int(data[offset]), true
	}
	f32 := func(offset, count int) ([]float64, bool) {
		if !inBounds(offset, 4*count, "f32") {
			return nil, false
		}
		values := make([]float64, count)
		for i := range values {
			values[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset+4*i:])))
		}
		return values, true
	}
	triple := func(offset int) [3]float32 {
		values, ok := f32(offset, 3)
		if !ok {
			return [3]float32{}
		}
		return [3]float32{float32(values[0]), float32(values[1]), float32(values[2])}
	}
	single := func(offset int) float32 {
		values, ok := f32(offset, 1)
		if !ok {
			return 0
		}
		return float32(values[0])
	}
	mdxFloat := func(offset int) float64 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(mdx[offset:])))
	}
	readString := func(offset, count int) string {
		return CString(Slice(data, offset, offset+count))
	}

	root, _ := u32(0x28)
	stats.HdrNodes = "?"
	if hdrNodes, ok := u32(0x2c); ok {
		stats.HdrNodes = strconv.Itoa(hdrNodes)
	}

	nameTableOffset, _ := u32(0xb8)
	nameCount, _ := u32(0xc0)
	names := make([]string, 0, nameCount)
	for i := range nameCount {
		nameOffset, ok := u32(nameTableOffset + 4*i)
		name := ""
		if ok {
			name = readString(nameOffset, nameLength)
		}
		names = append(names, name)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "# converted from %s\n", label)
	if mtllib != "" {
		fmt.Fprintf(&out, "mtllib %s\n", mtllib)
	}

	var facesCsv strings.Builder
	facesCsv.WriteString(MdlFacesCsvHeader + "\n")

	textures := map[string]bool{}
	var meshes []MdlMesh
	vbase := 1
	seen := map[int]bool{}

	mesh := func(header int, name string, flags int, rotation quat, position vec3) {
		problem := func(text string) {
			errors = append(errors, text+" in "+name)
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

		address0, _ := u32(header)
		address1, _ := u32(header + 4)
		if [2]int{address0, address1} != meshClassAddresses[class] {
			problem(fmt.Sprintf("class addresses %#x %#x for a %s mesh", address0, address1, kind))
		}

		// Each array descriptor is an offset, a count and a capacity. The count equals the capacity.
		for _, array := range []int{meshFaceArray, meshIndexCounts, meshIndexOffsets, meshCounters} {
			count, _ := u32(array + header + 4)
			capacity, _ := u32(array + header + 8)
			if count != capacity {
				problem(fmt.Sprintf("array at %#x has count %d and capacity %d", array, count, capacity))
			}
		}

		faceOffset, _ := u32(header + meshFaceArray)
		faceCount, _ := u32(header + meshFaceArray + 4)
		texture := readString(header+meshTexture, textureNameLength)
		stride, _ := u32(header + meshVertexStride)
		vertexFlags, _ := u32(header + meshVertexFlags)
		vertexCount, _ := u16(header + meshVertexCount)
		uvSetCount, _ := u16(header + meshUvSetCount)
		mdxOffset, _ := u32(header + meshMdxOffset)
		vertexOffset, ok := u32(header + meshVertexOffset)
		if !ok {
			return
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
		meshes = append(meshes, decoded)

		if !inBounds(faceOffset, faceSize*faceCount, "faces "+name) ||
			!inBounds(vertexOffset, vertexSize*vertexCount, "verts "+name) {
			return
		}

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
				return
			}

			current.normal, _ = f32(entry, 3)
			distance, _ := f32(entry+facePlaneOffset, 1)
			current.distance = distance[0]
			current.material, _ = u32(entry + faceMaterialOffset)
			faces = append(faces, current)
		}

		// The index list repeats the vertex indices of the faces, three per face. The 79 meshes without an
		// index list are lightsaber meshes and meshes without vertices.
		indexLists, _ := u32(header + meshIndexCounts + 4)
		if indexLists == 1 {
			countOffset, _ := u32(header + meshIndexCounts)
			listPointer, _ := u32(header + meshIndexOffsets)
			indexCount, _ := u32(countOffset)
			listOffset, _ := u32(listPointer)
			if indexCount != cornersPerFace*faceCount {
				problem(fmt.Sprintf("index count %d for %d faces", indexCount, faceCount))
			} else if inBounds(listOffset, faceIndexSize*indexCount, "index list "+name) {
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
			// The counter is read only to check that its offset lies inside the file.
			counterOffset, _ := u32(header + meshCounters)
			u32(counterOffset)
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
		if render == 0 {
			stats.Hidden++
			return
		}

		positionOffset, hasPosition := componentOffsets["position"]
		uvOffset, hasUv := componentOffsets["uv1"]
		hasUv = hasUv && hasMdx
		stats.Meshes++
		stats.Verts += vertexCount
		stats.Faces += faceCount

		fmt.Fprintf(&out, "o %s\n", name)
		material := strings.ToLower(texture)
		if texture != "" && material != "null" {
			fmt.Fprintf(&out, "usemtl %s\n", material)
			textures[material] = true
		}

		for i, vertex := range verts {
			if hasMdx && hasPosition {
				mdxPosition := mdxOffset + stride*i + positionOffset
				for axis := range 3 {
					if math.Abs(vertex[axis]-mdxFloat(mdxPosition+4*axis)) > positionTolerance {
						stats.Mismatch++
						break
					}
				}
			}

			world := qrot(rotation, vertex)
			fmt.Fprintf(&out, "v %.6f %.6f %.6f\n", world[0]+position[0], world[1]+position[1], world[2]+position[2])

			// Every vertex gets a texture coordinate, so that vertex and texture coordinate indices agree.
			u, v := 0.0, 0.0
			if hasUv {
				u = mdxFloat(mdxOffset + stride*i + uvOffset)
				v = mdxFloat(mdxOffset + stride*i + uvOffset + 4)
			}
			fmt.Fprintf(&out, "vt %.6f %.6f\n", u, v)
		}

		for i, current := range faces {
			ix := current.corners
			fmt.Fprintf(&out, "f %d/%d %d/%d %d/%d\n",
				ix[0]+vbase, ix[0]+vbase, ix[1]+vbase, ix[1]+vbase, ix[2]+vbase, ix[2]+vbase)

			fmt.Fprintf(&facesCsv, "%s,%d,%d", name, i, current.material)
			for _, adjacent := range current.adjacent {
				if adjacent == noAdjacentFace {
					adjacent = -1
				}
				fmt.Fprintf(&facesCsv, ",%d", adjacent)
			}
			facesCsv.WriteString("\n")
		}

		vbase += vertexCount
	}

	var node func(offset int, parentRotation quat, parentPosition vec3, depth int)
	node = func(offset int, parentRotation quat, parentPosition vec3, depth int) {
		if seen[offset] {
			errors = append(errors, fmt.Sprintf("node loop at %d", offset))
			return
		}
		if depth > maxNodeDepth {
			errors = append(errors, "depth")
			return
		}

		seen[offset] = true
		stats.Nodes++

		flags, _ := u16(offset)
		nameIndex, _ := u16(offset + 4)
		pos, posOk := f32(offset+0x10, 3)
		rotation, rotationOk := f32(offset+0x1c, 4)
		childOffset, _ := u32(offset + 0x2c)
		childCount, _ := u32(offset + 0x30)
		if !posOk || !rotationOk {
			return
		}

		worldRotation := qmul(parentRotation, quat{rotation[0], rotation[1], rotation[2], rotation[3]})
		rotated := qrot(parentRotation, vec3{pos[0], pos[1], pos[2]})
		worldPosition := vec3{
			rotated[0] + parentPosition[0],
			rotated[1] + parentPosition[1],
			rotated[2] + parentPosition[2],
		}
		name := fmt.Sprintf("node%d", offset)
		if nameIndex < len(names) {
			name = names[nameIndex]
		}
		if flags&nodeFlagMesh != 0 {
			mesh(offset+nodeHeaderSize, name, flags, worldRotation, worldPosition)
		}

		for i := range childCount {
			if child, ok := u32(childOffset + 4*i); ok {
				node(child, worldRotation, worldPosition, depth+1)
			}
		}
	}

	node(root, quat{1, 0, 0, 0}, vec3{0, 0, 0}, 0)

	sorted := make([]string, 0, len(textures))
	for texture := range textures {
		sorted = append(sorted, texture)
	}
	slices.Sort(sorted)

	return MdlResult{
		Obj:      out.String(),
		FacesCsv: facesCsv.String(),
		Meshes:   meshes,
		Stats:    stats,
		Textures: sorted,
		Errors:   errors,
	}
}

func ReportMdl(label string, result MdlResult) string {
	stats := result.Stats
	return fmt.Sprintf("%s nodes=%d meshes=%d hidden=%d verts=%d faces=%d mdl/mdx_mismatch=%d hdrnodes=%s "+
		"normals_ok=%d/%d planes_ok=%d/%d adjacent_ok=%d/%d adjacent_outside=%d radius_ok=%d/%d area_ok=%d/%d %s",
		label, stats.Nodes, stats.Meshes, stats.Hidden, stats.Verts, stats.Faces, stats.Mismatch, stats.HdrNodes,
		stats.NormalsMatching, stats.NormalsChecked, stats.PlanesMatching, stats.PlanesChecked,
		stats.AdjacentMatching, stats.AdjacentChecked, stats.AdjacentOutside, stats.RadiusMatching,
		stats.RadiusChecked, stats.AreaMatching, stats.AreaChecked,
		Verdict(result.Errors[:min(5, len(result.Errors))]...))
}
