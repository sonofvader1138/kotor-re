// This file builds glTF 2.0 documents: a JSON file (.gltf) that describes the scene and a binary buffer
// (.bin) that holds the vertex, index and face data. Models and walkmeshes are written in this form.
// Specification: https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html

package kotor

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"net/url"
	"slices"
)

// Component types and buffer view targets from the glTF specification.
const (
	GltfUnsignedShort = 5123
	GltfUnsignedInt   = 5125
	GltfFloat         = 5126

	gltfArrayBuffer        = 34962
	gltfElementArrayBuffer = 34963
)

// gltfNoTarget leaves the target of a buffer view unset, as for animation, skin and face data, which no mesh
// primitive reads as vertex data.
const gltfNoTarget = 0

// The game uses +Z as up and glTF uses +Y. A rotation of -90 degrees about X, as the quaternion
// (x, y, z, w), turns the one into the other.
var gltfZUpToYUp = [4]float32{-math.Sqrt2 / 2, 0, 0, math.Sqrt2 / 2}

// glTF requires 4-byte alignment for vertex attributes. Every buffer view starts on such a boundary.
const gltfAlignment = 4

type gltfAsset struct {
	Version   string `json:"version"`
	Generator string `json:"generator"`
}

type gltfScene struct {
	Name   string `json:"name,omitempty"`
	Nodes  []int  `json:"nodes"`
	Extras any    `json:"extras,omitempty"`
}

type gltfTarget struct {
	Node int    `json:"node"`
	Path string `json:"path"`
}

type gltfChannel struct {
	Sampler int        `json:"sampler"`
	Target  gltfTarget `json:"target"`
}

type gltfSampler struct {
	Input         int    `json:"input"`
	Interpolation string `json:"interpolation"`
	Output        int    `json:"output"`
}

type gltfAnimation struct {
	Name     string        `json:"name,omitempty"`
	Channels []gltfChannel `json:"channels"`
	Samplers []gltfSampler `json:"samplers"`
	Extras   any           `json:"extras,omitempty"`
}

type gltfSkin struct {
	Name                string `json:"name,omitempty"`
	InverseBindMatrices int    `json:"inverseBindMatrices"`
	Joints              []int  `json:"joints"`
}

type GltfNode struct {
	Name        string      `json:"name,omitempty"`
	Children    []int       `json:"children,omitempty"`
	Translation *[3]float32 `json:"translation,omitempty"`
	Rotation    *[4]float32 `json:"rotation,omitempty"`
	Mesh        *int        `json:"mesh,omitempty"`
	Skin        *int        `json:"skin,omitempty"`
	Extras      any         `json:"extras,omitempty"`
}

type gltfPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
	Material   *int           `json:"material,omitempty"`
}

type gltfMesh struct {
	Name       string          `json:"name,omitempty"`
	Primitives []gltfPrimitive `json:"primitives"`
}

type gltfTextureInfo struct {
	Index int `json:"index"`
}

// The metallic factor defaults to 1 in glTF, which would render every material as metal. It is always
// written as 0, because the game files carry no such value.
type gltfPbr struct {
	BaseColorFactor  *[4]float32      `json:"baseColorFactor,omitempty"`
	BaseColorTexture *gltfTextureInfo `json:"baseColorTexture,omitempty"`
	MetallicFactor   float32          `json:"metallicFactor"`
}

type gltfMaterial struct {
	Name                 string  `json:"name"`
	PbrMetallicRoughness gltfPbr `json:"pbrMetallicRoughness"`
}

type gltfTexture struct {
	Source int `json:"source"`
}

type gltfImage struct {
	Uri string `json:"uri"`
}

type gltfAccessor struct {
	BufferView    int       `json:"bufferView"`
	ByteOffset    int       `json:"byteOffset,omitempty"`
	ComponentType int       `json:"componentType"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	Min           []float32 `json:"min,omitempty"`
	Max           []float32 `json:"max,omitempty"`
}

type gltfBufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	Target     int `json:"target,omitempty"`
}

type gltfBuffer struct {
	Uri        string `json:"uri"`
	ByteLength int    `json:"byteLength"`
}

// Gltf is a glTF document under construction, together with its binary buffer.
type Gltf struct {
	Asset       gltfAsset        `json:"asset"`
	Scene       int              `json:"scene"`
	Scenes      []gltfScene      `json:"scenes"`
	Nodes       []GltfNode       `json:"nodes"`
	Meshes      []gltfMesh       `json:"meshes,omitempty"`
	Animations  []gltfAnimation  `json:"animations,omitempty"`
	Skins       []gltfSkin       `json:"skins,omitempty"`
	Materials   []gltfMaterial   `json:"materials,omitempty"`
	Textures    []gltfTexture    `json:"textures,omitempty"`
	Images      []gltfImage      `json:"images,omitempty"`
	Accessors   []gltfAccessor   `json:"accessors,omitempty"`
	BufferViews []gltfBufferView `json:"bufferViews,omitempty"`
	Buffers     []gltfBuffer     `json:"buffers,omitempty"`

	Bin []byte `json:"-"`
}

// newGltf starts a document with one scene, whose single root node turns the game's Z-up coordinates into
// glTF's Y-up coordinates. The content goes below node 0.
func newGltf(sceneName string) *Gltf {
	rotation := gltfZUpToYUp
	return &Gltf{
		Asset:  gltfAsset{Version: "2.0", Generator: "re/kotor"},
		Scenes: []gltfScene{{Name: sceneName, Nodes: []int{0}}},
		Nodes:  []GltfNode{{Rotation: &rotation}},
	}
}

// gltfView appends data to the binary buffer as a new buffer view and returns its index. A target of 0
// leaves the target unset, as for data that no mesh primitive reads.
func gltfView(doc *Gltf, data []byte, target int) int {
	for len(doc.Bin)%gltfAlignment != 0 {
		doc.Bin = append(doc.Bin, 0)
	}

	doc.BufferViews = append(doc.BufferViews, gltfBufferView{ByteOffset: len(doc.Bin), ByteLength: len(data), Target: target})
	doc.Bin = append(doc.Bin, data...)
	return len(doc.BufferViews) - 1
}

// gltfAccessorAt adds an accessor of count elements at byteOffset in a buffer view and returns its index.
func gltfAccessorAt(doc *Gltf, view, byteOffset, componentType, count int, kind string) int {
	doc.Accessors = append(doc.Accessors, gltfAccessor{
		BufferView:    view,
		ByteOffset:    byteOffset,
		ComponentType: componentType,
		Count:         count,
		Type:          kind,
	})
	return len(doc.Accessors) - 1
}

var gltfVectorTypes = map[int]string{1: "SCALAR", 2: "VEC2", 3: "VEC3", 4: "VEC4", 16: "MAT4"}

// gltfFloats stores vectors of width floats in a buffer view with the given target and returns the accessor
// index. glTF requires the bounds of positions and of animation times, so bounded accessors get their minimum
// and maximum.
func gltfFloats(doc *Gltf, values []float32, width int, bounded bool, target int) int {
	data := make([]byte, 0, 4*len(values))
	for _, value := range values {
		data = binary.LittleEndian.AppendUint32(data, math.Float32bits(value))
	}

	count := len(values) / width
	accessor := gltfAccessorAt(doc, gltfView(doc, data, target), 0, GltfFloat, count, gltfVectorTypes[width])
	if bounded && count > 0 {
		low := slices.Clone(values[:width])
		high := slices.Clone(values[:width])
		for i, value := range values {
			low[i%width] = min(low[i%width], value)
			high[i%width] = max(high[i%width], value)
		}
		doc.Accessors[accessor].Min = low
		doc.Accessors[accessor].Max = high
	}
	return accessor
}

// gltfIndexView stores vertex indices as u32 in a new buffer view and returns its index. Callers add one
// accessor per primitive.
func gltfIndexView(doc *Gltf, indices []int) int {
	data := make([]byte, 0, 4*len(indices))
	for _, index := range indices {
		data = binary.LittleEndian.AppendUint32(data, uint32(index))
	}
	return gltfView(doc, data, gltfElementArrayBuffer)
}

// gltfMaterialAdd adds a material and returns its index. A non-empty image URI becomes its base color
// texture, and a non-nil color its base color factor.
func gltfMaterialAdd(doc *Gltf, name, imageUri string, color *[4]float32) int {
	material := gltfMaterial{Name: name, PbrMetallicRoughness: gltfPbr{BaseColorFactor: color}}
	if imageUri != "" {
		doc.Images = append(doc.Images, gltfImage{Uri: (&url.URL{Path: imageUri}).EscapedPath()})
		doc.Textures = append(doc.Textures, gltfTexture{Source: len(doc.Images) - 1})
		material.PbrMetallicRoughness.BaseColorTexture = &gltfTextureInfo{Index: len(doc.Textures) - 1}
	}

	doc.Materials = append(doc.Materials, material)
	return len(doc.Materials) - 1
}

// gltfEncode finishes a document whose binary buffer is stored at binUri and returns its JSON text,
// indented with tabs.
func gltfEncode(doc *Gltf, binUri string) ([]byte, error) {
	doc.Buffers = []gltfBuffer{{Uri: (&url.URL{Path: binUri}).EscapedPath(), ByteLength: len(doc.Bin)}}
	text, err := json.MarshalIndent(doc, "", "\t")
	if err != nil {
		return nil, err
	}
	return append(text, '\n'), nil
}
