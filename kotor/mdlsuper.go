// This file resolves the animations that a model inherits from its supermodel chain, as the game does. The
// game looks up an animation by name in the model, then in its supermodel, and so on, and pairs the nodes
// of the model with those of the animation by node number in a walk of both trees from the root. Layout and
// evidence: see README.md, section "Animations from the supermodel".

package kotor

import (
	"strings"
)

// MdlTree is the node tree of a model with the names of its animations. Nodes are in depth-first order,
// which is the order of the glTF nodes that ConvertMdl writes, starting at glTF node 1. Numbers holds the
// node number of each node and Children the indices of its children.
type MdlTree struct {
	Supermodel string
	Numbers    []int
	Children   [][]int
	Animations []string
}

// MdlSupermodelLink names the animations that a model takes from one model of its supermodel chain. Uri is
// the shared animation file of that model, relative to the model's glTF file. Nodes pairs a glTF node of
// the model with the glTF node of the shared file whose animation tracks it receives.
type MdlSupermodelLink struct {
	Model      string   `json:"model"`
	Uri        string   `json:"uri"`
	Animations []string `json:"animations"`
	Nodes      [][2]int `json:"nodes"`
}

// noSupermodel is the supermodel name of a model without one.
const noSupermodel = "null"

// ReadMdlTree reads the node tree, the supermodel name and the animation names of an MDL file. It returns
// false when the tree leaves the file or loops.
func ReadMdlTree(mdl []byte) (MdlTree, bool) {
	const (
		rootField         = 0x28
		animationArray    = 0x58
		supermodelField   = 0x88
		nodeNumberField   = 2
		animationNameText = 8
	)
	r := newMdlReader(mdl[min(fileHeaderSize, len(mdl)):])
	tree := MdlTree{Supermodel: r.text(supermodelField, modelNameLength)}

	seen := map[int]bool{}
	var walk func(offset, depth int) int
	walk = func(offset, depth int) int {
		if seen[offset] || depth > maxNodeDepth || !r.inBounds(offset, nodeHeaderSize, "node") {
			r.problem("node tree")
			return -1
		}

		seen[offset] = true
		number, _ := r.u16(offset + nodeNumberField)
		index := len(tree.Numbers)
		tree.Numbers = append(tree.Numbers, number)
		tree.Children = append(tree.Children, nil)

		childOffset, childCount := r.array(offset+nodeChildArray, 4, "children")
		for i := range childCount {
			child, _ := r.u32(childOffset + 4*i)
			if childIndex := walk(child, depth+1); childIndex >= 0 {
				tree.Children[index] = append(tree.Children[index], childIndex)
			}
		}
		return index
	}
	root, _ := r.u32(rootField)
	walk(root, 0)

	offsets, count := r.array(animationArray, 4, "animation offsets")
	for i := range count {
		offset, _ := r.u32(offsets + 4*i)
		tree.Animations = append(tree.Animations, r.text(offset+animationNameText, animationNameLength))
	}
	return tree, len(r.errors) == 0 && len(tree.Numbers) > 0
}

// SupermodelLinks follows the supermodel chain of a model and returns one link for each model of the chain
// that provides an animation that no nearer model holds. Names are compared without regard to case, and the
// first animation of a name in a model wins, as in the game. lookup returns the tree of a model and the URI
// of its shared animation file, or false when the model is unknown.
func SupermodelLinks(tree MdlTree, lookup func(name string) (MdlTree, string, bool)) []MdlSupermodelLink {
	held := map[string]bool{}
	for _, name := range tree.Animations {
		held[strings.ToLower(name)] = true
	}

	var links []MdlSupermodelLink
	visited := map[string]bool{}
	name := tree.Supermodel
	for !strings.EqualFold(name, noSupermodel) && name != "" && !visited[strings.ToLower(name)] {
		visited[strings.ToLower(name)] = true
		super, uri, known := lookup(name)
		if !known {
			break
		}

		link := MdlSupermodelLink{Model: name, Uri: uri, Animations: []string{}}
		for _, animation := range super.Animations {
			if !held[strings.ToLower(animation)] {
				held[strings.ToLower(animation)] = true
				link.Animations = append(link.Animations, animation)
			}
		}

		// The roots are paired. Below a pair, child i of the model's node is paired with a child of the other
		// node that has the same node number. The search starts at child i of the other node and wraps around,
		// as in the game (PC code, 0x483ED0).
		var pair func(own, other int)
		pair = func(own, other int) {
			link.Nodes = append(link.Nodes, [2]int{own + 1, other + 1})

			candidates := super.Children[other]
			for i, child := range tree.Children[own] {
				for k := range candidates {
					candidate := candidates[(i+k)%len(candidates)]
					if super.Numbers[candidate] == tree.Numbers[child] {
						pair(child, candidate)
						break
					}
				}
			}
		}
		pair(0, 0)

		if len(link.Animations) > 0 {
			links = append(links, link)
		}
		name = super.Supermodel
	}
	return links
}
