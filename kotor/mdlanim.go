// This file decodes the controllers of MDL nodes, which are keyframed tracks of node properties, and the
// animations of a model, which are separate node trees whose controllers animate the nodes of the model with
// the same names. Position, orientation and scale tracks become glTF animation channels. The other tracks are
// kept in the extras of the glTF animation. Layout: see README.md, section "3D models: MDL and MDX".

package kotor

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Controller types that every node class understands.
const (
	controllerPosition    = 8
	controllerOrientation = 20
	controllerScale       = 36
)

// A controller key is 16 bytes: the u32 type, an i16, the u16 key count, the u16 index of the first time and
// the u16 index of the first value in the controller data, and the u8 column count. Bit 4 of the column
// count marks Bezier keys, which store three values per column. An orientation with two columns stores each
// key as one packed u32.
const (
	controllerKeySize           = 16
	controllerKeyArray          = 0x38
	controllerDataArray         = 0x44
	controllerBezierFlag        = 0x10
	controllerColumnMask        = 0x0f
	bezierParts                 = 3
	compressedQuaternionColumns = 2
)

// The names of the controller types for each node class, as the game executable spells the property names.
// The numbers are those of other descriptions of the format, and the value ranges agree with them.
var controllerNames = map[int]map[int]string{
	nodeFlagBase: {controllerPosition: "position", controllerOrientation: "orientation", controllerScale: "scale"},
	nodeFlagMesh: {100: "selfillumcolor", 132: "alpha"},
	nodeFlagLight: {76: "color", 88: "radius", 96: "shadowradius", 100: "verticaldisplacement",
		140: "multiplier"},
	nodeFlagEmitter: {80: "alphaEnd", 84: "alphaStart", 88: "birthrate", 92: "bounce_co", 96: "combinetime",
		100: "drag", 104: "fps", 108: "frameEnd", 112: "frameStart", 116: "grav", 120: "lifeExp", 124: "mass",
		128: "p2p_bezier2", 132: "p2p_bezier3", 136: "particleRot", 140: "randvel", 144: "sizeStart",
		148: "sizeEnd", 152: "sizeStart_y", 156: "sizeEnd_y", 160: "spread", 164: "threshold", 168: "velocity",
		172: "xsize", 176: "ysize", 180: "blurlength", 184: "lightningDelay", 188: "lightningRadius",
		192: "lightningScale", 196: "lightningSubDiv", 200: "lightningZigzag", 216: "alphaMid",
		220: "percentStart", 224: "percentMid", 228: "percentEnd", 232: "sizeMid", 236: "sizeMid_y",
		240: "m_fRandomBirthRate", 252: "targetsize", 256: "numcontrolpts", 260: "controlptradius",
		264: "controlptdelay", 268: "tangentspread", 272: "tangentlength", 284: "colorMid", 380: "colorEnd",
		392: "colorStart", 502: "detonate"},
}

// controllerName returns the name of a controller type on a node with the given flags.
func controllerName(flags, kind int) string {
	for _, class := range []int{nodeFlagBase, nodeFlagMesh, nodeFlagLight, nodeFlagEmitter} {
		if name, known := controllerNames[class][kind]; known && (class == nodeFlagBase || flags&class != 0) {
			return name
		}
	}
	return fmt.Sprintf("controller_%d", kind)
}

// mdlController is one decoded controller. Each row holds the values of one key: the columns, or for a
// Bezier key the columns, then the incoming and the outgoing control offsets. A packed orientation is
// unpacked to x, y, z, w.
type mdlController struct {
	kind   int
	field  int
	times  []float64
	rows   [][]float64
	bezier bool
}

// unpackQuaternion unpacks an orientation stored as one u32: x in bits 0 to 10 and y in bits 11 to 21, both
// mapped from 0..2046 to 1..-1, and z in bits 22 to 31, mapped from 0..1022 to 1..-1. w is the non-positive
// value that makes the quaternion unit length, or 0 when x, y and z are already too long, in which case
// they are normalized.
func unpackQuaternion(packed int) []float64 {
	const (
		wideMask  = 0x7ff
		wideScale = 1023.0
		zShift    = 22
		yShift    = 11
		zScale    = 511.0
	)
	x := 1 - float64(packed&wideMask)/wideScale
	y := 1 - float64((packed>>yShift)&wideMask)/wideScale
	z := 1 - float64(packed>>zShift)/zScale
	square := x*x + y*y + z*z
	if square < 1 {
		return []float64{x, y, z, -math.Sqrt(1 - square)}
	}
	length := math.Sqrt(square)
	return []float64{x / length, y / length, z / length, 0}
}

// readControllers decodes the controllers of the node at offset and claims their keys and data. Every value
// of the data array must belong to exactly one controller, unless the node has no controllers.
func readControllers(r *mdlReader, offset int, name string) []mdlController {
	keys, keyCount := r.array(offset+controllerKeyArray, controllerKeySize, "controller keys")
	data, dataCount := r.array(offset+controllerDataArray, 4, "controller data")
	uses := make([]int, dataCount)
	use := func(start, count int) bool {
		if start+count > dataCount {
			r.problem(fmt.Sprintf("controller values %d+%d outside %d in %s", start, count, dataCount, name))
			return false
		}
		for i := start; i < start+count; i++ {
			uses[i]++
		}
		return true
	}

	var controllers []mdlController
	for i := range keyCount {
		key := keys + controllerKeySize*i
		kind, _ := r.u32(key)
		field, _ := r.u16(key + 4)
		rows, _ := r.u16(key + 6)
		timeIndex, _ := r.u16(key + 8)
		valueIndex, _ := r.u16(key + 10)
		columns, _ := r.u8(key + 12)

		bezier := columns&controllerBezierFlag != 0
		packed := kind == controllerOrientation && columns == compressedQuaternionColumns
		width := columns & controllerColumnMask
		if bezier {
			width *= bezierParts
		}
		if packed {
			width = 1
		}
		if columns&^(controllerColumnMask|controllerBezierFlag) != 0 || width == 0 {
			r.problem(fmt.Sprintf("controller %d has column byte %#x in %s", kind, columns, name))
			continue
		}
		if !use(timeIndex, rows) || !use(valueIndex, rows*width) {
			continue
		}

		controller := mdlController{kind: kind, field: int(int16(field)), bezier: bezier}
		controller.times, _ = r.f32(data+4*timeIndex, rows)
		for row := range rows {
			at := data + 4*(valueIndex+row*width)
			if packed {
				value, _ := r.u32(at)
				controller.rows = append(controller.rows, unpackQuaternion(value))
				continue
			}
			values, _ := r.f32(at, width)
			controller.rows = append(controller.rows, values)
		}
		if len(controller.times) == rows && len(controller.rows) == rows {
			controllers = append(controllers, controller)
		}
	}

	if keyCount > 0 && slices.ContainsFunc(uses, func(count int) bool { return count != 1 }) {
		r.problem("controller data not used exactly once in " + name)
	}
	return controllers
}

// MdlEvent is a named event at a time of an animation, such as a footstep sound.
type MdlEvent struct {
	Time float32 `json:"time"`
	Name string  `json:"name"`
}

// MdlTrack is an animated controller that glTF cannot express as a channel. Input is the accessor of the key
// times, and Output the accessor of the values, Width floats per key. Bezier keys hold the values, then the
// incoming and the outgoing control offsets.
type MdlTrack struct {
	Node          int    `json:"node"`
	Controller    string `json:"controller"`
	Interpolation string `json:"interpolation"`
	Width         int    `json:"width"`
	Input         int    `json:"input"`
	Output        int    `json:"output"`
}

// MdlAnimationExtras holds the animation header and the tracks that are not glTF channels. ConstantChannel
// marks an animation without position, orientation and scale tracks, which gets one constant channel because
// glTF requires at least one.
type MdlAnimationExtras struct {
	Length          float32    `json:"length"`
	Transition      float32    `json:"transition"`
	Root            string     `json:"root"`
	Events          []MdlEvent `json:"events"`
	Tracks          []MdlTrack `json:"tracks,omitempty"`
	ConstantChannel bool       `json:"constant_channel,omitempty"`
}

// mdlTarget is a node of the model tree that animations can address: its glTF node, its MDL flags, its name
// and its rest translation.
type mdlTarget struct {
	node        int
	flags       int
	name        string
	translation vec3
}

// findTarget returns the node of the model tree with the given node number and name. Names are compared
// without regard to case.
func findTarget(targets map[int][]mdlTarget, number int, name string) (mdlTarget, bool) {
	var found []mdlTarget
	for _, candidate := range targets[number] {
		if strings.EqualFold(candidate.name, name) {
			found = append(found, candidate)
		}
	}
	if len(found) != 1 {
		return mdlTarget{}, false
	}
	return found[0], true
}

// Fields of the animation header, which starts with a 0x50-byte geometry header like the model header.
const (
	animationHeaderSize = 0x88
	animationType       = 5
	animationEventSize  = 36
	animationNameLength = 32
)

// The first two words of every animation header.
var animationClassAddresses = [2]int{0x004134f0, 0x0043ece0}

// readAnimations decodes the animations of the model and adds them to doc as glTF animations. targets maps
// each node number to the nodes of the model tree with that number, names gives the name table, and root is
// the root node of the model tree. An animation node animates the model node with the same node number and
// name.
func readAnimations(r *mdlReader, doc *Gltf, stats *MdlStats, targets map[int][]mdlTarget, names []string, root mdlTarget) {
	const (
		animationArray  = 0x58
		reservedStart   = 0x30
		reservedEnd     = 0x4c
		trailingField   = 0x84
		lengthField     = 0x50
		transitionField = 0x54
		rootNameField   = 0x58
		eventArray      = 0x78
	)
	offsets, count := r.array(animationArray, 4, "animation offsets")
	for i := range count {
		offset, _ := r.u32(offsets + 4*i)
		r.claim(offset, animationHeaderSize, "animation header")
		name := r.text(offset+8, animationNameLength)
		address0, _ := r.u32(offset)
		address1, _ := r.u32(offset + 4)
		kind, _ := r.u8(offset + reservedEnd)
		trailing, _ := r.u32(offset + trailingField)
		if [2]int{address0, address1} != animationClassAddresses || kind != animationType || trailing != 0 ||
			slices.ContainsFunc(Slice(r.data, offset+reservedStart, offset+reservedEnd), func(b byte) bool { return b != 0 }) {
			r.problem("unexpected animation header of " + name)
		}

		extras := &MdlAnimationExtras{
			Length:     r.single(offset + lengthField),
			Transition: r.single(offset + transitionField),
			Root:       r.text(offset+rootNameField, animationNameLength),
			Events:     []MdlEvent{},
		}
		events, eventCount := r.array(offset+eventArray, animationEventSize, "animation events")
		for e := range eventCount {
			at := events + animationEventSize*e
			extras.Events = append(extras.Events, MdlEvent{Time: r.single(at), Name: r.text(at+4, animationNameLength)})
		}

		animation := gltfAnimation{Name: name, Channels: []gltfChannel{}, Samplers: []gltfSampler{}, Extras: extras}
		// When two animation nodes animate the same node, the first in tree order is kept.
		channel := func(node int, path string, interpolation string, times, values []float32, width int) {
			if slices.ContainsFunc(animation.Channels, func(other gltfChannel) bool {
				return other.Target == gltfTarget{Node: node, Path: path}
			}) {
				stats.DuplicateTracks++
				return
			}
			input := gltfFloats(doc, times, 1, true, gltfNoTarget)
			output := gltfFloats(doc, values, width, false, gltfNoTarget)
			animation.Samplers = append(animation.Samplers, gltfSampler{Input: input, Interpolation: interpolation, Output: output})
			animation.Channels = append(animation.Channels, gltfChannel{Sampler: len(animation.Samplers) - 1, Target: gltfTarget{Node: node, Path: path}})
		}

		seen := map[int]bool{}
		var walk func(at, parent, depth int)
		walk = func(at, parent, depth int) {
			if seen[at] || depth > maxNodeDepth {
				r.problem("animation node loop in " + name)
				return
			}
			seen[at] = true
			stats.AnimationNodes++
			r.claim(at, nodeHeaderSize, "animation node")

			flags, _ := r.u16(at)
			number, _ := r.u16(at + 2)
			nameIndex, _ := r.u16(at + 4)
			owner, _ := r.u32(at + nodeOwnerField)
			parentOffset, _ := r.u32(at + nodeParentField)
			pose, _ := r.f32(at+0x10, 7)
			if flags != nodeFlagBase || owner != offset || parentOffset != parent || !slices.Equal(pose, []float64{0, 0, 0, 1, 0, 0, 0}) {
				r.problem("unexpected animation node header in " + name)
			}

			controllers := readControllers(r, at, name)
			nodeName := ""
			if nameIndex < len(names) {
				nodeName = names[nameIndex]
			}
			target, known := findTarget(targets, number, nodeName)
			if !known {
				stats.UnmatchedAnimationNodes++
			}
			for _, controller := range controllers {
				if !known {
					break
				}
				stats.Keys += len(controller.times)
				times := make([]float32, len(controller.times))
				for k, time := range controller.times {
					times[k] = float32(time)
				}

				var values []float32
				switch controller.kind {
				case controllerPosition:
					// A position key is an offset from the rest position. A Bezier key stores control
					// offsets at about one third of the way to its neighbours, which become the tangents of
					// a glTF cubic spline.
					interpolation := "LINEAR"
					cubic := controller.bezier && len(times) > 1
					if cubic {
						interpolation = "CUBICSPLINE"
					}
					for k, row := range controller.rows {
						var value, incoming, outgoing [3]float32
						for axis := range 3 {
							value[axis] = float32(target.translation[axis] + row[axis])
							if cubic && k > 0 {
								incoming[axis] = float32(-bezierParts * row[3+axis] / (controller.times[k] - controller.times[k-1]))
							}
							if cubic && k < len(times)-1 {
								outgoing[axis] = float32(bezierParts * row[6+axis] / (controller.times[k+1] - controller.times[k]))
							}
						}
						if cubic {
							values = append(values, incoming[:]...)
						}
						values = append(values, value[:]...)
						if cubic {
							values = append(values, outgoing[:]...)
						}
					}
					channel(target.node, "translation", interpolation, times, values, 3)
				case controllerOrientation:
					for _, row := range controller.rows {
						length := math.Sqrt(row[0]*row[0] + row[1]*row[1] + row[2]*row[2] + row[3]*row[3])
						for axis := range 4 {
							values = append(values, float32(row[axis]/length))
						}
					}
					channel(target.node, "rotation", "LINEAR", times, values, 4)
				case controllerScale:
					for _, row := range controller.rows {
						values = append(values, float32(row[0]), float32(row[0]), float32(row[0]))
					}
					channel(target.node, "scale", "LINEAR", times, values, 3)
				default:
					for _, row := range controller.rows {
						for _, value := range row {
							values = append(values, float32(value))
						}
					}
					interpolation := "LINEAR"
					if controller.bezier {
						interpolation = "BEZIER"
					}
					extras.Tracks = append(extras.Tracks, MdlTrack{
						Node:          target.node,
						Controller:    controllerName(target.flags, controller.kind),
						Interpolation: interpolation,
						Width:         len(controller.rows[0]),
						Input:         gltfFloats(doc, times, 1, true, gltfNoTarget),
						Output:        gltfFloats(doc, values, 1, false, gltfNoTarget),
					})
				}
				if (controller.kind == controllerOrientation || controller.kind == controllerScale) && controller.bezier {
					r.problem(fmt.Sprintf("Bezier keys for controller %d in %s", controller.kind, name))
				}
			}

			children, childCount := r.array(at+nodeChildArray, 4, "animation children")
			for c := range childCount {
				child, _ := r.u32(children + 4*c)
				walk(child, at, depth+1)
			}
		}
		treeRoot, _ := r.u32(offset + 0x28)
		walk(treeRoot, 0, 0)

		if len(animation.Channels) == 0 {
			extras.ConstantChannel = true
			stats.ConstantChannels++
			rest := root.translation
			channel(root.node, "translation", "LINEAR", []float32{0}, []float32{float32(rest[0]), float32(rest[1]), float32(rest[2])}, 3)
		}
		stats.Animations++
		stats.Channels += len(animation.Channels)
		stats.Tracks += len(extras.Tracks)
		doc.Animations = append(doc.Animations, animation)
	}
}
