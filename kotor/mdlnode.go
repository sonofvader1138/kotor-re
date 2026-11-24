// This file decodes the headers that follow the 80-byte node header of lights, emitters and references, and
// the headers that follow the mesh header of skinned, dangly, AABB and lightsaber meshes. It also holds the
// rigid transforms that the skins need. Layout: see README.md, section "3D models: MDL and MDX".

package kotor

import (
	"fmt"
	"math"
)

// Node flags that select a node class with its own header after the node header.
const (
	nodeFlagLight     = 0x02
	nodeFlagEmitter   = 0x04
	nodeFlagReference = 0x10
	nodeFlagBase      = 0x01
	nodeFlagsKnown    = nodeFlagBase | nodeFlagLight | nodeFlagEmitter | nodeFlagReference | nodeFlagMesh |
		nodeFlagSkin | nodeFlagDangly | nodeFlagAabb | nodeFlagLightsaber
)

// Sizes of the class headers.
const (
	lightHeaderSize      = 0x5c
	emitterHeaderSize    = 0xe0
	referenceHeaderSize  = 0x24
	meshHeaderSize       = 0x14c
	skinHeaderSize       = 0x64
	danglyHeaderSize     = 0x1c
	aabbHeaderSize       = 4
	lightsaberHeaderSize = 0x14
)

// MdlFlare is one lens flare of a light.
type MdlFlare struct {
	Texture    string     `json:"texture"`
	Size       float32    `json:"size"`
	Position   float32    `json:"position"`
	ColorShift [3]float32 `json:"color_shift"`
}

// MdlLight holds the light header. The names of the seven u32 fields follow the names of the light
// properties in the game executable, in the order in which other descriptions of the format give them.
type MdlLight struct {
	FlareRadius   float32    `json:"flare_radius"`
	Flares        []MdlFlare `json:"flares"`
	Priority      int        `json:"priority"`
	AmbientOnly   int        `json:"ambient_only"`
	DynamicType   int        `json:"dynamic_type"`
	AffectDynamic int        `json:"affect_dynamic"`
	Shadow        int        `json:"shadow"`
	GenerateFlare int        `json:"generate_flare"`
	FadingLight   int        `json:"fading_light"`
}

// MdlEmitter holds the emitter header, which describes a particle emitter.
type MdlEmitter struct {
	DeadSpace             float32 `json:"dead_space"`
	BlastRadius           float32 `json:"blast_radius"`
	BlastLength           float32 `json:"blast_length"`
	BranchCount           int     `json:"branch_count"`
	ControlPointSmoothing float32 `json:"control_point_smoothing"`
	GridX                 int     `json:"x_grid"`
	GridY                 int     `json:"y_grid"`
	SpawnType             int     `json:"spawn_type"`
	Update                string  `json:"update"`
	Render                string  `json:"render"`
	Blend                 string  `json:"blend"`
	Texture               string  `json:"texture"`
	ChunkName             string  `json:"chunk_name"`
	TwoSidedTexture       int     `json:"two_sided_texture"`
	Loop                  int     `json:"loop"`
	RenderOrder           int     `json:"render_order"`
	FrameBlending         int     `json:"frame_blending"`
	DepthTexture          string  `json:"depth_texture"`
	Flags                 int     `json:"flags"`
}

// MdlReference holds the reference header, which attaches another model to the node.
type MdlReference struct {
	Model        string `json:"model"`
	Reattachable int    `json:"reattachable"`
}

// MdlDangly holds the dangly mesh header. Constraints is the index of an accessor with one f32 per vertex,
// absent for a mesh without vertices.
type MdlDangly struct {
	Displacement float32 `json:"displacement"`
	Tightness    float32 `json:"tightness"`
	Period       float32 `json:"period"`
	Constraints  *int    `json:"constraints,omitempty"`
}

// MdlAabb gives the accessors of an AABB tree, whose nodes are stored in depth-first order. Boxes holds the
// minimum and the maximum corner of each node as two f32[3]. Nodes holds for each node the u32 indices of its
// left and right children, its face and its split plane, with 0xFFFFFFFF for none.
type MdlAabb struct {
	Boxes int `json:"boxes"`
	Nodes int `json:"nodes"`
}

// MdlSkinExtras gives the glTF node that carries the skinned mesh and its skin. It is absent for a skinned
// mesh that is not rendered.
type MdlSkinExtras struct {
	Node int `json:"node"`
}

// MdlStatic is a controller of the model tree with its single key. A node may hold two controllers of the
// same type.
type MdlStatic struct {
	Name   string    `json:"name"`
	Values []float32 `json:"values"`
}

// MdlNodeExtras holds the extras of a glTF node: the decoded class headers, the static controller values and,
// for a rendered mesh with faces, the accessors of the face materials (u32 per face) and of the adjacent faces
// (u16[3] per face, 0xFFFF for none).
type MdlNodeExtras struct {
	Controllers   []MdlStatic    `json:"controllers,omitempty"`
	Light         *MdlLight      `json:"light,omitempty"`
	Emitter       *MdlEmitter    `json:"emitter,omitempty"`
	Reference     *MdlReference  `json:"reference,omitempty"`
	Mesh          *MdlMesh       `json:"mesh,omitempty"`
	FaceMaterials *int           `json:"face_materials,omitempty"`
	FaceAdjacent  *int           `json:"face_adjacent,omitempty"`
	Dangly        *MdlDangly     `json:"dangly,omitempty"`
	Aabb          *MdlAabb       `json:"aabb,omitempty"`
	Skin          *MdlSkinExtras `json:"skin,omitempty"`
}

// readLight decodes the light header at offset and claims its flare arrays and flare texture names.
func readLight(r *mdlReader, offset int) *MdlLight {
	const (
		unknownArray   = 0x04
		sizeArray      = 0x10
		positionArray  = 0x1c
		colorShifts    = 0x28
		textureArray   = 0x34
		integerFields  = 0x40
		colorShiftSize = 12
	)
	r.claim(offset, lightHeaderSize, "light header")
	if _, count := r.array(offset+unknownArray, 4, "light array"); count != 0 {
		r.problem(fmt.Sprintf("light array at 0x04 has %d entries", count))
	}

	sizes, sizeCount := r.array(offset+sizeArray, 4, "flare sizes")
	positions, positionCount := r.array(offset+positionArray, 4, "flare positions")
	shifts, shiftCount := r.array(offset+colorShifts, colorShiftSize, "flare colour shifts")
	textures, textureCount := r.array(offset+textureArray, 4, "flare textures")
	if positionCount != sizeCount || shiftCount != sizeCount || textureCount != sizeCount {
		r.problem("flare arrays differ in length")
	}

	light := &MdlLight{FlareRadius: r.single(offset), Flares: []MdlFlare{}}
	for i := range min(sizeCount, positionCount, shiftCount, textureCount) {
		textureOffset, _ := r.u32(textures + 4*i)
		light.Flares = append(light.Flares, MdlFlare{
			Texture:    r.claimString(textureOffset, "flare texture"),
			Size:       r.single(sizes + 4*i),
			Position:   r.single(positions + 4*i),
			ColorShift: r.triple(shifts + colorShiftSize*i),
		})
	}

	fields := make([]int, 7)
	for i := range fields {
		fields[i], _ = r.u32(offset + integerFields + 4*i)
	}
	light.Priority, light.AmbientOnly, light.DynamicType, light.AffectDynamic = fields[0], fields[1], fields[2], fields[3]
	light.Shadow, light.GenerateFlare, light.FadingLight = fields[4], fields[5], fields[6]
	return light
}

// readEmitter decodes the emitter header at offset.
func readEmitter(r *mdlReader, offset int) *MdlEmitter {
	const (
		shortNameLength = 16
		depthTexture    = 0xbb
	)
	r.claim(offset, emitterHeaderSize, "emitter header")
	word := func(field int) int {
		value, _ := r.u32(offset + field)
		return value
	}
	renderOrder, _ := r.u16(offset + 0xb8)
	frameBlending, _ := r.u8(offset + 0xba)
	return &MdlEmitter{
		DeadSpace:             r.single(offset),
		BlastRadius:           r.single(offset + 0x04),
		BlastLength:           r.single(offset + 0x08),
		BranchCount:           word(0x0c),
		ControlPointSmoothing: r.single(offset + 0x10),
		GridX:                 word(0x14),
		GridY:                 word(0x18),
		SpawnType:             word(0x1c),
		Update:                r.text(offset+0x20, textureNameLength),
		Render:                r.text(offset+0x40, textureNameLength),
		Blend:                 r.text(offset+0x60, textureNameLength),
		Texture:               r.text(offset+0x80, textureNameLength),
		ChunkName:             r.text(offset+0xa0, shortNameLength),
		TwoSidedTexture:       word(0xb0),
		Loop:                  word(0xb4),
		RenderOrder:           renderOrder,
		FrameBlending:         frameBlending,
		DepthTexture:          r.text(offset+depthTexture, textureNameLength),
		Flags:                 word(0xdc),
	}
}

// readReference decodes the reference header at offset.
func readReference(r *mdlReader, offset int) *MdlReference {
	r.claim(offset, referenceHeaderSize, "reference header")
	reattachable, _ := r.u32(offset + textureNameLength)
	return &MdlReference{Model: r.text(offset, textureNameLength), Reattachable: reattachable}
}

// mdlSkin is a decoded skin header. BoneMap gives the local bone index of each entry of the name table, or -1.
// Rotations (x, y, z, w) and Translations hold the stored inverse bind pose of each entry of the name table.
// Bones lists the name table entries of the local bones in local bone order.
type mdlSkin struct {
	weightOffset int
	indexOffset  int
	boneMap      []int
	rotations    [][4]float64
	translations []vec3
	bones        []int
}

// readSkin decodes the skin header at offset and claims its arrays.
func readSkin(r *mdlReader, offset int, name string) mdlSkin {
	const (
		quaternionSize   = 16
		boneSlots        = 16
		boneNumbers      = 0x40
		rotationArray    = 0x1c
		translationArray = 0x28
		thirdArray       = 0x34
	)
	r.claim(offset, skinHeaderSize, "skin header")
	if _, count := r.array(offset, 4, "skin array at 0x00"); count != 0 {
		r.problem(fmt.Sprintf("skin array at 0x00 has %d entries in %s", count, name))
	}

	var skin mdlSkin
	skin.weightOffset, _ = r.u32(offset + 0x0c)
	skin.indexOffset, _ = r.u32(offset + 0x10)
	mapOffset, _ := r.u32(offset + 0x14)
	mapCount, _ := r.u32(offset + 0x18)
	r.claim(mapOffset, 4*mapCount, "bone map")
	rotations, rotationCount := r.array(offset+rotationArray, quaternionSize, "bone rotations")
	translations, translationCount := r.array(offset+translationArray, vertexSize, "bone translations")
	_, thirdCount := r.array(offset+thirdArray, 4, "skin array at 0x34")
	if rotationCount != mapCount || translationCount != mapCount || thirdCount != mapCount {
		r.problem("skin arrays differ in length in " + name)
		return skin
	}

	for i := range mapCount {
		bone := -1
		if values, ok := r.f32(mapOffset+4*i, 1); ok && values[0] >= 0 {
			bone = int(values[0])
		}
		skin.boneMap = append(skin.boneMap, bone)

		// The rotation is stored as w, x, y, z.
		rotation, _ := r.f32(rotations+quaternionSize*i, 4)
		translation, _ := r.f32(translations+vertexSize*i, 3)
		if rotation == nil || translation == nil {
			return skin
		}
		skin.rotations = append(skin.rotations, [4]float64{rotation[1], rotation[2], rotation[3], rotation[0]})
		skin.translations = append(skin.translations, vec3{translation[0], translation[1], translation[2]})
	}

	// The local bones are numbered from 0 without gaps. The first 16 of them are also listed by name table
	// entry in the u16[16] at 0x40.
	skin.bones = make([]int, 0, mapCount)
	for bone := 0; ; bone++ {
		entry := -1
		for i, mapped := range skin.boneMap {
			if mapped == bone {
				if entry >= 0 {
					r.problem(fmt.Sprintf("bone %d appears twice in %s", bone, name))
				}
				entry = i
			}
		}
		if entry < 0 {
			break
		}
		if bone < boneSlots {
			if listed, _ := r.u16(offset + boneNumbers + 2*bone); listed != entry {
				r.problem(fmt.Sprintf("bone %d is listed as %d, mapped from %d in %s", bone, listed, entry, name))
			}
		}
		skin.bones = append(skin.bones, entry)
	}
	for _, mapped := range skin.boneMap {
		if mapped >= len(skin.bones) {
			r.problem(fmt.Sprintf("bone numbers have a gap before %d in %s", mapped, name))
		}
	}
	return skin
}

// readDangly decodes the dangly mesh header at offset and claims its constraints and its copy of the vertex
// positions, which must equal the vertex array of the mesh header. It returns the header and the constraints.
func readDangly(r *mdlReader, offset int, verts []vec3, name string) (*MdlDangly, []float32) {
	r.claim(offset, danglyHeaderSize, "dangly header")
	constraintOffset, count := r.array(offset, 4, "constraints")
	if count != len(verts) {
		r.problem(fmt.Sprintf("%d constraints for %d vertices in %s", count, len(verts), name))
		count = 0
	}

	copyOffset, _ := r.u32(offset + 0x18)
	r.claim(copyOffset, vertexSize*len(verts), "dangly vertex copy")
	for i, vertex := range verts {
		if values, ok := r.f32(copyOffset+vertexSize*i, 3); !ok || vec3(values) != vertex {
			r.problem("dangly vertex copy differs from the vertices in " + name)
			break
		}
	}

	constraints := make([]float32, count)
	for i := range constraints {
		constraints[i] = r.single(constraintOffset + 4*i)
	}
	return &MdlDangly{
		Displacement: r.single(offset + 0x0c),
		Tightness:    r.single(offset + 0x10),
		Period:       r.single(offset + 0x14),
	}, constraints
}

// mdlAabbNode is one node of an AABB tree, with its children as indices in depth-first order.
type mdlAabbNode struct {
	low, high   [3]float32
	left, right int
	face        int
	plane       int
}

// readAabb decodes the AABB tree whose root offset is in the header at offset and claims its nodes. It returns
// the nodes in depth-first order, and whether the leaves hold every face once and every box encloses what lies
// below it. Many trees store their boxes rounded to two decimals, so a box may miss by up to aabbSlack.
func readAabb(r *mdlReader, offset int, verts []vec3, faces [][3]int, name string) ([]mdlAabbNode, bool) {
	const (
		aabbNodeSize = 40
		childField   = 0x18
		aabbSlack    = 0.006
	)
	consistent := true
	r.claim(offset, aabbHeaderSize, "aabb header")
	root, _ := r.u32(offset)

	var nodes []mdlAabbNode
	leaves := make([]int, len(faces))
	seen := map[int]bool{}
	encloses := func(outer mdlAabbNode, low, high [3]float64) bool {
		for axis := range 3 {
			if low[axis] < float64(outer.low[axis])-aabbSlack || high[axis] > float64(outer.high[axis])+aabbSlack {
				return false
			}
		}
		return true
	}

	var walk func(at, depth int) int
	walk = func(at, depth int) int {
		if seen[at] || depth > maxNodeDepth {
			r.problem("aabb tree loop in " + name)
			return -1
		}
		seen[at] = true
		r.claim(at, aabbNodeSize, "aabb node")

		bounds, ok := r.f32(at, 6)
		left, _ := r.u32(at + childField)
		right, _ := r.u32(at + childField + 4)
		face, _ := r.u32(at + childField + 8)
		plane, _ := r.u32(at + childField + 12)
		if !ok {
			return -1
		}

		index := len(nodes)
		node := mdlAabbNode{left: -1, right: -1, face: -1, plane: plane}
		for axis := range 3 {
			node.low[axis], node.high[axis] = float32(bounds[axis]), float32(bounds[3+axis])
		}
		nodes = append(nodes, node)

		if face != none {
			if left != 0 || right != 0 || plane != 0 || face >= len(faces) {
				r.problem(fmt.Sprintf("aabb leaf with children %d %d, plane %d and face %d in %s", left, right, plane, face, name))
				return index
			}
			nodes[index].face = face
			leaves[face]++
			var low, high [3]float64
			for corner, vertex := range faces[face] {
				for axis := range 3 {
					if corner == 0 || verts[vertex][axis] < low[axis] {
						low[axis] = verts[vertex][axis]
					}
					if corner == 0 || verts[vertex][axis] > high[axis] {
						high[axis] = verts[vertex][axis]
					}
				}
			}
			consistent = consistent && encloses(node, low, high)
			return index
		}

		if plane == 0 || plane&(plane-1) != 0 || plane > 0x20 {
			r.problem(fmt.Sprintf("aabb split plane %d in %s", plane, name))
		}
		nodes[index].left = walk(left, depth+1)
		nodes[index].right = walk(right, depth+1)
		for _, child := range []int{nodes[index].left, nodes[index].right} {
			if child < 0 {
				continue
			}
			var low, high [3]float64
			for axis := range 3 {
				low[axis], high[axis] = float64(nodes[child].low[axis]), float64(nodes[child].high[axis])
			}
			consistent = consistent && encloses(node, low, high)
		}
		return index
	}
	walk(root, 0)

	for _, count := range leaves {
		consistent = consistent && count == 1
	}
	return nodes, consistent
}

// readLightsaber decodes the lightsaber header at offset. It claims the three vertex arrays, whose positions
// must equal the vertex array of the mesh header, and returns the texture coordinates and the normals.
func readLightsaber(r *mdlReader, offset int, verts []vec3, name string) ([][2]float64, []vec3) {
	const uvSize = 8
	r.claim(offset, lightsaberHeaderSize, "lightsaber header")
	positionOffset, _ := r.u32(offset)
	uvOffset, _ := r.u32(offset + 4)
	normalOffset, _ := r.u32(offset + 8)
	r.claim(positionOffset, vertexSize*len(verts), "lightsaber positions")
	r.claim(uvOffset, uvSize*len(verts), "lightsaber texture coordinates")
	r.claim(normalOffset, vertexSize*len(verts), "lightsaber normals")

	uvs := make([][2]float64, 0, len(verts))
	normals := make([]vec3, 0, len(verts))
	for i, vertex := range verts {
		position, positionOk := r.f32(positionOffset+vertexSize*i, 3)
		uv, uvOk := r.f32(uvOffset+uvSize*i, 2)
		normal, normalOk := r.f32(normalOffset+vertexSize*i, 3)
		if !positionOk || !uvOk || !normalOk {
			return nil, nil
		}
		if vec3(position) != vertex {
			r.problem("lightsaber positions differ from the vertices in " + name)
			return nil, nil
		}
		uvs = append(uvs, [2]float64{uv[0], uv[1]})
		normals = append(normals, vec3(normal))
	}
	return uvs, normals
}

// mdlTransform is a rigid transform: a rotation as a unit quaternion (x, y, z, w), then a translation.
type mdlTransform struct {
	rotation    [4]float64
	translation vec3
}

var identityTransform = mdlTransform{rotation: [4]float64{0, 0, 0, 1}}

func quaternionMultiply(a, b [4]float64) [4]float64 {
	return [4]float64{
		a[3]*b[0] + a[0]*b[3] + a[1]*b[2] - a[2]*b[1],
		a[3]*b[1] - a[0]*b[2] + a[1]*b[3] + a[2]*b[0],
		a[3]*b[2] + a[0]*b[1] - a[1]*b[0] + a[2]*b[3],
		a[3]*b[3] - a[0]*b[0] - a[1]*b[1] - a[2]*b[2],
	}
}

func quaternionRotate(q [4]float64, v vec3) vec3 {
	conjugate := [4]float64{-q[0], -q[1], -q[2], q[3]}
	rotated := quaternionMultiply(quaternionMultiply(q, [4]float64{v[0], v[1], v[2], 0}), conjugate)
	return vec3{rotated[0], rotated[1], rotated[2]}
}

// transformCompose returns the transform that applies child first and then parent.
func transformCompose(parent, child mdlTransform) mdlTransform {
	moved := quaternionRotate(parent.rotation, child.translation)
	return mdlTransform{
		rotation:    quaternionMultiply(parent.rotation, child.rotation),
		translation: vec3{parent.translation[0] + moved[0], parent.translation[1] + moved[1], parent.translation[2] + moved[2]},
	}
}

func transformInverse(t mdlTransform) mdlTransform {
	conjugate := [4]float64{-t.rotation[0], -t.rotation[1], -t.rotation[2], t.rotation[3]}
	moved := quaternionRotate(conjugate, t.translation)
	return mdlTransform{rotation: conjugate, translation: vec3{-moved[0], -moved[1], -moved[2]}}
}

// transformMatrix returns the 4x4 matrix of a transform in column-major order, as glTF stores it.
func transformMatrix(t mdlTransform) []float32 {
	matrix := make([]float32, 16)
	for column, axis := range []vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
		rotated := quaternionRotate(t.rotation, axis)
		for row := range 3 {
			matrix[4*column+row] = float32(rotated[row])
		}
	}
	for row := range 3 {
		matrix[12+row] = float32(t.translation[row])
	}
	matrix[15] = 1
	return matrix
}

// transformDistance returns the angle in radians between the rotations of two transforms and the distance
// between their translations.
func transformDistance(a, b mdlTransform) (float64, float64) {
	dot := 0.0
	for i := range 4 {
		dot += a.rotation[i] * b.rotation[i]
	}
	angle := 2 * math.Acos(min(1, math.Abs(dot)))
	distance := 0.0
	for axis := range 3 {
		delta := a.translation[axis] - b.translation[axis]
		distance += delta * delta
	}
	return angle, math.Sqrt(distance)
}
