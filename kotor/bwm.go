// BWM V1.0: a binary walkmesh, the triangle mesh that defines where characters can walk. Resource types
// 2016 (area room, wok), 2052 (door, dwk) and 2053 (placeable, pwk), all in data/models.bif. A 136-byte
// header holds the walkmesh type, four use positions, a position and nine table descriptors. The tables
// follow in a fixed order and end exactly at the end of the file. Layout: see FILE_FORMATS.md.
package kotor

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	TypeWok = 2016
	TypeDwk = 2052
	TypePwk = 2053
)

const (
	bwmSignature  = "BWM V1.0"
	bwmHeaderSize = 0x88
)

// Walkmesh types stored at offset 0x08.
const (
	BwmTypeObject = 0
	BwmTypeArea   = 1
)

// Sizes of one entry of each table.
const (
	bwmVectorSize    = 12
	bwmFaceSize      = 12
	bwmScalarSize    = 4
	bwmAabbSize      = 44
	bwmAdjacencySize = 12
	bwmEdgeSize      = 8
)

// bwmNone marks a missing face, child or transition.
const bwmNone = -1

// The three edges of a face are numbered 3 * face + k. Edge k runs from vertex k to vertex (k + 1) mod 3.
const edgesPerFace = 3

type BwmAabb struct {
	Box     [6]float64 // minimum x, y, z, then maximum x, y, z
	Face    int        // bwmNone for an inner node
	Unknown uint32     // 4 in every node
	Split   uint32     // 0 in a leaf; otherwise 1, 2, 4, 8, 16 or 32
	Left    int
	Right   int
}

type BwmEdge struct {
	Edge       int // 3 * face + k
	Transition int // index of a room in the area's layout, or bwmNone
}

type Bwm struct {
	Type uint32

	// RelativeUse and AbsoluteUse hold two points each. Position is the walkmesh node's position in its
	// model. In a placeable walkmesh the absolute points and the position are 0.
	RelativeUse [2][3]float64
	AbsoluteUse [2][3]float64
	Position    [3]float64

	Vertices  [][3]float64
	Faces     [][3]int
	Materials []uint32
	Normals   [][3]float64
	Distances []float64

	// AabbRoot is meaningful only when there are AABB nodes. An AABB is an axis-aligned bounding box.
	AabbRoot  uint32
	Aabbs     []BwmAabb
	Adjacency [][3]int
	Edges     []BwmEdge

	// Perimeters holds the end of each perimeter in Edges, exclusive.
	Perimeters []int

	// Irregularities that the parser accepts. Asymmetric counts adjacency entries whose partner does not
	// point back. Unindexed counts faces without an AABB leaf. NormalsMatching counts the faces whose stored
	// normal agrees with the vertex order, out of NormalsChecked non-degenerate faces with a non-zero normal.
	Asymmetric      int
	Unindexed       int
	NormalsMatching int
	NormalsChecked  int
	ZeroNormals     int

	Errors []string
}

// bwmTable is one table descriptor of the header: the offset of its count, or -1 when the count is
// implied by the face count, and the offset of its data offset.
type bwmTable struct {
	name        string
	countOffset int
	dataOffset  int
	entrySize   int
}

// The tables in file order. Materials, normals and distances have one entry per face.
var bwmTables = []bwmTable{
	{"vertices", 0x48, 0x4C, bwmVectorSize},
	{"faces", 0x50, 0x54, bwmFaceSize},
	{"materials", -1, 0x58, bwmScalarSize},
	{"normals", -1, 0x5C, bwmVectorSize},
	{"distances", -1, 0x60, bwmScalarSize},
	{"aabbs", 0x64, 0x68, bwmAabbSize},
	{"adjacency", 0x70, 0x74, bwmAdjacencySize},
	{"edges", 0x78, 0x7C, bwmEdgeSize},
	{"perimeters", 0x80, 0x84, bwmScalarSize},
}

const bwmAabbRootOffset = 0x6C

// ParseBwm reads a walkmesh. It requires the tables to lie in file order without gaps and to end exactly
// at the end of the file, and every index to refer to an existing entry.
func ParseBwm(data []byte) *Bwm {
	bwm := &Bwm{}
	fail := func(format string, args ...any) {
		bwm.Errors = append(bwm.Errors, fmt.Sprintf(format, args...))
	}
	if len(data) < bwmHeaderSize || string(data[:len(bwmSignature)]) != bwmSignature {
		fail("no BWM V1.0 header")
		return bwm
	}

	u32At := func(offset int) uint32 { return binary.LittleEndian.Uint32(data[offset:]) }
	i32At := func(offset int) int { return int(int32(u32At(offset))) }
	f32At := func(offset int) float64 { return float64(math.Float32frombits(u32At(offset))) }
	vectorAt := func(offset int) [3]float64 { return [3]float64{f32At(offset), f32At(offset + 4), f32At(offset + 8)} }

	bwm.Type = u32At(0x08)
	bwm.RelativeUse = [2][3]float64{vectorAt(0x0C), vectorAt(0x18)}
	bwm.AbsoluteUse = [2][3]float64{vectorAt(0x24), vectorAt(0x30)}
	bwm.Position = vectorAt(0x3C)
	bwm.AabbRoot = u32At(bwmAabbRootOffset)

	faceCount := int(u32At(0x50))
	counts := map[string]int{}
	starts := map[string]int{}
	expected := bwmHeaderSize
	for _, table := range bwmTables {
		count := faceCount
		if table.countOffset >= 0 {
			count = int(u32At(table.countOffset))
		}
		start := int(u32At(table.dataOffset))
		counts[table.name] = count
		starts[table.name] = start
		if count == 0 {
			continue
		}

		if start != expected {
			fail("%s at %d, expected %d", table.name, start, expected)
		}
		expected = start + count*table.entrySize
		if start < bwmHeaderSize || expected > len(data) {
			fail("%s out of bounds (%d+%d*%d>%d)", table.name, start, count, table.entrySize, len(data))
			return bwm
		}
	}
	if expected != len(data) {
		fail("tables end at %d, file size %d", expected, len(data))
	}

	for i := range counts["vertices"] {
		bwm.Vertices = append(bwm.Vertices, vectorAt(starts["vertices"]+bwmVectorSize*i))
	}

	vertexCount := len(bwm.Vertices)
	for i := range faceCount {
		offset := starts["faces"] + bwmFaceSize*i
		face := [3]int{int(u32At(offset)), int(u32At(offset + 4)), int(u32At(offset + 8))}
		for _, index := range face {
			if index >= vertexCount {
				fail("face %d: vertex %d of %d", i, index, vertexCount)
				return bwm
			}
		}
		bwm.Faces = append(bwm.Faces, face)
		bwm.Materials = append(bwm.Materials, u32At(starts["materials"]+bwmScalarSize*i))
		bwm.Normals = append(bwm.Normals, vectorAt(starts["normals"]+bwmVectorSize*i))
		bwm.Distances = append(bwm.Distances, f32At(starts["distances"]+bwmScalarSize*i))
	}

	for i, face := range bwm.Faces {
		a, b, c := bwm.Vertices[face[0]], bwm.Vertices[face[1]], bwm.Vertices[face[2]]
		u := vec3{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
		v := vec3{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
		cross := vec3{u[1]*v[2] - u[2]*v[1], u[2]*v[0] - u[0]*v[2], u[0]*v[1] - u[1]*v[0]}
		crossLength := math.Sqrt(cross[0]*cross[0] + cross[1]*cross[1] + cross[2]*cross[2])

		normal := bwm.Normals[i]
		if normal == [3]float64{} {
			bwm.ZeroNormals++
			continue
		}
		if crossLength < minCrossLength {
			continue
		}
		bwm.NormalsChecked++
		cosine := (cross[0]*normal[0] + cross[1]*normal[1] + cross[2]*normal[2]) / crossLength
		if cosine > minNormalCosine {
			bwm.NormalsMatching++
		}
	}

	// The AABB nodes form a binary tree. Every node must be reached once from the root, and every leaf
	// must name an existing face.
	aabbCount := counts["aabbs"]
	for i := range aabbCount {
		offset := starts["aabbs"] + bwmAabbSize*i
		node := BwmAabb{
			Face:    i32At(offset + 24),
			Unknown: u32At(offset + 28),
			Split:   u32At(offset + 32),
			Left:    i32At(offset + 36),
			Right:   i32At(offset + 40),
		}
		for k := range node.Box {
			node.Box[k] = f32At(offset + 4*k)
		}

		inner := node.Face == bwmNone
		if node.Face >= faceCount || node.Face < bwmNone {
			fail("AABB %d: face %d of %d", i, node.Face, faceCount)
		}
		if inner && (node.Left < 0 || node.Left >= aabbCount || node.Right < 0 || node.Right >= aabbCount) {
			fail("AABB %d: children %d and %d of %d", i, node.Left, node.Right, aabbCount)
		}
		if !inner && (node.Left != bwmNone || node.Right != bwmNone) {
			fail("AABB %d: leaf with children", i)
		}
		bwm.Aabbs = append(bwm.Aabbs, node)
	}

	if aabbCount > 0 && len(bwm.Errors) == 0 {
		visits := make([]int, aabbCount)
		leaves := make([]bool, faceCount)
		pending := []int{int(bwm.AabbRoot)}
		if int(bwm.AabbRoot) >= aabbCount {
			fail("AABB root %d of %d", bwm.AabbRoot, aabbCount)
			pending = nil
		}
		for len(pending) > 0 && len(bwm.Errors) == 0 {
			index := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			visits[index]++
			if visits[index] > 1 {
				fail("AABB %d reached twice", index)
				break
			}

			node := bwm.Aabbs[index]
			if node.Face == bwmNone {
				pending = append(pending, node.Left, node.Right)
			} else {
				leaves[node.Face] = true
			}
		}
		for i, count := range visits {
			if count == 0 {
				fail("AABB %d not reached from the root", i)
				break
			}
		}
		for _, indexed := range leaves {
			if !indexed {
				bwm.Unindexed++
			}
		}
	}

	// Each adjacency row belongs to the face of the same index. An entry names the edge of the
	// neighbouring face, or is bwmNone.
	adjacencyCount := counts["adjacency"]
	if adjacencyCount > faceCount {
		fail("%d adjacency rows for %d faces", adjacencyCount, faceCount)
		return bwm
	}
	boundary := map[int]bool{}
	for i := range adjacencyCount {
		offset := starts["adjacency"] + bwmAdjacencySize*i
		row := [3]int{i32At(offset), i32At(offset + 4), i32At(offset + 8)}
		for k, edge := range row {
			if edge == bwmNone {
				boundary[edgesPerFace*i+k] = true
				continue
			}
			if edge < 0 || edge >= edgesPerFace*adjacencyCount {
				fail("adjacency %d: edge %d of %d", i, edge, edgesPerFace*adjacencyCount)
				return bwm
			}
		}
		bwm.Adjacency = append(bwm.Adjacency, row)
	}
	for i, row := range bwm.Adjacency {
		for k, edge := range row {
			if edge != bwmNone && bwm.Adjacency[edge/edgesPerFace][edge%edgesPerFace] != edgesPerFace*i+k {
				bwm.Asymmetric++
			}
		}
	}

	// The edge list holds every boundary edge of the adjacency table once.
	for i := range counts["edges"] {
		offset := starts["edges"] + bwmEdgeSize*i
		edge := BwmEdge{Edge: i32At(offset), Transition: i32At(offset + 4)}
		if !boundary[edge.Edge] {
			fail("edge %d: %d is not a boundary edge", i, edge.Edge)
			return bwm
		}
		if edge.Transition < bwmNone {
			fail("edge %d: transition %d", i, edge.Transition)
		}
		delete(boundary, edge.Edge)
		bwm.Edges = append(bwm.Edges, edge)
	}
	if len(boundary) > 0 {
		fail("%d boundary edges missing from the edge list", len(boundary))
	}

	// Each perimeter is a closed loop: every edge ends where the next one starts, and the last edge ends
	// where the first starts. Vertices are compared by position.
	edgeVertex := func(edge, end int) [3]float64 {
		face := bwm.Faces[edge/edgesPerFace]
		return bwm.Vertices[face[(edge%edgesPerFace+end)%edgesPerFace]]
	}
	start := 0
	for i := range counts["perimeters"] {
		end := int(u32At(starts["perimeters"] + bwmScalarSize*i))
		if end <= start || end > len(bwm.Edges) {
			fail("perimeter %d ends at %d after %d (%d edges)", i, end, start, len(bwm.Edges))
			return bwm
		}
		for j := start; j < end; j++ {
			next := j + 1
			if next == end {
				next = start
			}
			if edgeVertex(bwm.Edges[j].Edge, 1) != edgeVertex(bwm.Edges[next].Edge, 0) {
				fail("perimeter %d is not closed at edge %d", i, j)
				return bwm
			}
		}
		bwm.Perimeters = append(bwm.Perimeters, end)
		start = end
	}
	if start != len(bwm.Edges) {
		fail("perimeters cover %d of %d edges", start, len(bwm.Edges))
	}
	return bwm
}

func ReportBwm(label string, bwm *Bwm) string {
	transitions := 0
	for _, edge := range bwm.Edges {
		if edge.Transition != bwmNone {
			transitions++
		}
	}
	return fmt.Sprintf("%s type=%d verts=%d faces=%d aabbs=%d adjacency=%d edges=%d transitions=%d perimeters=%d asymmetric=%d unindexed=%d zero_normals=%d normals_ok=%d/%d %s",
		label, bwm.Type, len(bwm.Vertices), len(bwm.Faces), len(bwm.Aabbs), len(bwm.Adjacency), len(bwm.Edges),
		transitions, len(bwm.Perimeters), bwm.Asymmetric, bwm.Unindexed, bwm.ZeroNormals, bwm.NormalsMatching,
		bwm.NormalsChecked, Verdict(bwm.Errors[:min(5, len(bwm.Errors))]...))
}

// BwmObj writes a walkmesh as Wavefront OBJ. Faces keep their file order; a "usemtl" line names the
// surface material whenever it changes. materialName maps a material number to its MTL name.
func BwmObj(bwm *Bwm, label, mtllib string, materialName func(uint32) string) string {
	var out strings.Builder
	format := func(value float64) string { return fmt.Sprintf("%.6f", value) }

	fmt.Fprintf(&out, "# walkmesh %s\nmtllib %s\no %s\n", label, mtllib, label)
	for _, vertex := range bwm.Vertices {
		fmt.Fprintf(&out, "v %s %s %s\n", format(vertex[0]), format(vertex[1]), format(vertex[2]))
	}

	current := ""
	for i, face := range bwm.Faces {
		name := materialName(bwm.Materials[i])
		if name != current {
			fmt.Fprintf(&out, "usemtl %s\n", name)
			current = name
		}
		fmt.Fprintf(&out, "f %d %d %d\n", face[0]+1, face[1]+1, face[2]+1)
	}
	return out.String()
}
