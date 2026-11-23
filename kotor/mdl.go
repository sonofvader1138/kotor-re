// This file converts MDL + MDX models to Wavefront OBJ. Layout: see FILE_FORMATS.md. MDL offsets are
// relative to file offset 12.

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

// A face is 32 bytes. It starts with the f32[3] face normal and ends with the u16[3] vertex indices at 0x1A.
const (
	faceSize           = 32
	faceVerticesOffset = 26
	vertexSize         = 12
)

// Offset of the render byte in the mesh header. A mesh whose render byte is 0 is not drawn.
const meshRenderOffset = 0x139

// Deeper node trees are treated as corrupt.
const maxNodeDepth = 200

// The MDX copy of a vertex position must match the MDL copy within this distance.
const positionTolerance = 1e-5

// Faces with a smaller cross product are degenerate and their normal is not checked.
const minCrossLength = 1e-9

// A stored face normal agrees with the winding when the cosine between them exceeds this value.
const minNormalCosine = 0.99

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

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

type MdlStats struct {
	Nodes           int
	Hidden          int
	Meshes          int
	Verts           int
	Faces           int
	Mismatch        int
	NormalsMatching int
	NormalsChecked  int

	// HdrNodes is the node count field of the header, as text: "?" when it lies outside the file.
	HdrNodes string
}

type MdlResult struct {
	Obj      string
	Stats    MdlStats
	Textures []string
	Errors   []string
}

// ConvertMdl writes mtllib, when not empty, as the material library at the top of the OBJ.
// Meshes whose render byte is 0 are skipped; the game does not appear to draw them.
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
	for i := 0; i < nameCount; i++ {
		nameOffset, ok := u32(nameTableOffset + 4*i)
		name := ""
		if ok {
			name = readString(nameOffset, nameLength)
		}
		names = append(names, name)
	}

	var out strings.Builder
	out.WriteString("# converted from " + label + "\n")
	if mtllib != "" {
		out.WriteString("mtllib " + mtllib + "\n")
	}

	textures := map[string]bool{}
	vbase := 1
	seen := map[int]bool{}

	mesh := func(header int, name string, rotation quat, position vec3) {
		faceOffset, _ := u32(header + 0x08)
		faceCount, _ := u32(header + 0x0c)
		texture := readString(header+0x58, textureNameLength)
		stride, _ := u32(header + 0xfc)
		positionOffset, _ := u32(header + 0x104)
		uvOffset, _ := u32(header + 0x110)
		vertexCount, _ := u16(header + 0x130)
		mdxOffset, _ := u32(header + 0x144)
		vertexOffset, ok := u32(header + 0x148)
		if !ok {
			return
		}

		inBounds(faceOffset, faceSize*faceCount, "faces "+name)
		inBounds(vertexOffset, vertexSize*vertexCount, "verts "+name)

		hasMdx := stride > 0 && vertexCount > 0 && len(mdx) > 0
		if hasMdx && mdxOffset+stride*vertexCount > len(mdx) {
			errors = append(errors, "mdx range "+name)
			hasMdx = false
		}
		if render, ok := u8(header + meshRenderOffset); ok && render == 0 {
			stats.Hidden++
			return
		}

		hasUv := hasMdx && uvOffset != none
		stats.Meshes++
		stats.Verts += vertexCount
		stats.Faces += faceCount

		out.WriteString("o " + name + "\n")
		material := strings.ToLower(texture)
		if texture != "" && material != "null" {
			out.WriteString("usemtl " + material + "\n")
			textures[material] = true
		}

		verts := make([]vec3, 0, vertexCount)
		for i := 0; i < vertexCount; i++ {
			values, _ := f32(vertexOffset+vertexSize*i, 3)
			vertex := vec3{values[0], values[1], values[2]}
			verts = append(verts, vertex)

			if hasMdx && positionOffset != none {
				mdxPosition := mdxOffset + stride*i + positionOffset
				for axis := 0; axis < 3; axis++ {
					if math.Abs(vertex[axis]-mdxFloat(mdxPosition+4*axis)) > positionTolerance {
						stats.Mismatch++
						break
					}
				}
			}

			world := qrot(rotation, vertex)
			out.WriteString("v " + formatNumber(world[0]+position[0]) + " " + formatNumber(world[1]+position[1]) +
				" " + formatNumber(world[2]+position[2]) + "\n")

			// vt for every vertex keeps v/vt indices aligned
			u, v := 0.0, 0.0
			if hasUv {
				u = mdxFloat(mdxOffset + stride*i + uvOffset)
				v = mdxFloat(mdxOffset + stride*i + uvOffset + 4)
			}
			out.WriteString("vt " + formatNumber(u) + " " + formatNumber(v) + "\n")
		}

		for face := 0; face < faceCount; face++ {
			faceEntry := faceOffset + faceSize*face
			vertexIndices := faceEntry + faceVerticesOffset
			var ix [3]int
			valid := true
			for corner := range ix {
				index, ok := u16(vertexIndices + 2*corner)
				ix[corner] = index
				valid = valid && ok && index < vertexCount
			}
			if !valid {
				errors = append(errors, fmt.Sprintf("face index >= %d in %s", vertexCount, name))
				continue
			}

			// stored normal must match winding
			normal, _ := f32(faceEntry, 3)
			corner0, corner1, corner2 := verts[ix[0]], verts[ix[1]], verts[ix[2]]
			var edge1, edge2 vec3
			for axis := 0; axis < 3; axis++ {
				edge1[axis] = corner1[axis] - corner0[axis]
				edge2[axis] = corner2[axis] - corner0[axis]
			}
			cross := vec3{
				edge1[1]*edge2[2] - edge1[2]*edge2[1],
				edge1[2]*edge2[0] - edge1[0]*edge2[2],
				edge1[0]*edge2[1] - edge1[1]*edge2[0],
			}
			length := math.Sqrt(cross[0]*cross[0] + cross[1]*cross[1] + cross[2]*cross[2])
			if length > minCrossLength {
				stats.NormalsChecked++
				if (cross[0]*normal[0]+cross[1]*normal[1]+cross[2]*normal[2])/length > minNormalCosine {
					stats.NormalsMatching++
				}
			}

			fmt.Fprintf(&out, "f %d/%d %d/%d %d/%d\n",
				ix[0]+vbase, ix[0]+vbase, ix[1]+vbase, ix[1]+vbase, ix[2]+vbase, ix[2]+vbase)
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
			mesh(offset+nodeHeaderSize, name, worldRotation, worldPosition)
		}

		for i := 0; i < childCount; i++ {
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

	return MdlResult{Obj: out.String(), Stats: stats, Textures: sorted, Errors: errors}
}

func ReportMdl(label string, result MdlResult) string {
	stats := result.Stats
	return fmt.Sprintf("%s nodes=%d meshes=%d hidden=%d verts=%d faces=%d mdl/mdx_mismatch=%d hdrnodes=%s normals_ok=%d/%d %s",
		label, stats.Nodes, stats.Meshes, stats.Hidden, stats.Verts, stats.Faces, stats.Mismatch,
		stats.HdrNodes, stats.NormalsMatching, stats.NormalsChecked, Verdict(result.Errors[:min(5, len(result.Errors))]...))
}
