package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"re/kotor"
)

// Fields of the model header and the animation header that the supermodel check reads.
const (
	sourceSupermodel    = 0x88
	sourceAnimationName = 0x08
)

// supermodelSource is the MDL data of a model of the BIF archives, and the folder of its converted files.
type supermodelSource struct {
	archive string
	resref  string
	mdl     []byte
}

// supermodelSources reads the models of the BIF archives in kotor.Build.ModelBifs once, by lowercase resref.
// Supermodel names are resolved against these archives only.
var supermodelSources = sync.OnceValue(func() map[string]supermodelSource {
	key := kotor.ReadKey()
	sources := map[string]supermodelSource{}
	for _, name := range kotor.Build.ModelBifs {
		for _, resource := range kotor.BifResources(kotor.BifPath(name), key) {
			if resource.Ext == "mdl" {
				sources[strings.ToLower(resource.Resref)] = supermodelSource{archive: name, resref: resource.Resref, mdl: resource.Data}
			}
		}
	}
	return sources
})

// supermodelLink is one entry of extras.supermodel_animations of a model glTF scene.
type supermodelLink struct {
	Model      string   `json:"model"`
	Uri        string   `json:"uri"`
	Animations []string `json:"animations"`
	Nodes      [][2]int `json:"nodes"`
}

// animationNames returns the names of the animations of a source model in stored order.
func animationNames(model sourceModel) []string {
	var names []string
	offsets := model.u32(sourceAnimations)
	for i := range model.u32(sourceAnimations + 4) {
		names = append(names, model.text(model.u32(offsets+4*i)+sourceAnimationName, sourceStringLength))
	}
	return names
}

// expectedLinks follows the supermodel chain of a source model as the game does. An animation comes from the
// nearest model of the chain that holds its name, compared without regard to case. The nodes are paired from
// the roots down as pairNodes describes. A model of the chain that provides no animation gets no link.
func expectedLinks(model sourceModel, sources map[string]supermodelSource) []supermodelLink {
	held := map[string]bool{}
	for _, name := range animationNames(model) {
		held[strings.ToLower(name)] = true
	}
	own := model.tree(model.u32(sourceRootNode))
	ownIndex := map[int]int{}
	for i, node := range own {
		ownIndex[node] = i + 1
	}

	var links []supermodelLink
	visited := map[string]bool{}
	name := model.text(sourceSupermodel, sourceStringLength)
	for {
		source, known := sources[strings.ToLower(name)]
		if !known || visited[strings.ToLower(name)] {
			return links
		}
		visited[strings.ToLower(name)] = true

		super := newSourceModel(source.mdl)
		link := supermodelLink{Model: name, Animations: []string{}}
		for _, animation := range animationNames(super) {
			if !held[strings.ToLower(animation)] {
				held[strings.ToLower(animation)] = true
				link.Animations = append(link.Animations, animation)
			}
		}

		superIndex := map[int]int{}
		for i, node := range super.tree(super.u32(sourceRootNode)) {
			superIndex[node] = i + 1
		}
		for _, pair := range pairNodes(model, own[0], super, super.u32(sourceRootNode)) {
			link.Nodes = append(link.Nodes, [2]int{ownIndex[pair[0]], superIndex[pair[1]]})
		}

		if len(link.Animations) > 0 {
			links = append(links, link)
		}
		name = super.text(sourceSupermodel, sourceStringLength)
	}
}

// checkSupermodelLinks compares extras.supermodel_animations of a model glTF file, read with jq, with the
// links that the source model and the BIF archives give (see expectedLinks). Each link's file must exist next
// to the model, hold every listed animation and have a node for every paired index. path is the converted
// file without extensions.
func checkSupermodelLinks(path string, mdl []byte) string {
	result := kotor.Exec("jq", []string{"-c", ".scenes[0].extras.supermodel_animations // []", path + ".gltf"}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}
	var links []supermodelLink
	if err := json.Unmarshal(result.Stdout, &links); err != nil {
		return err.Error()
	}

	expected := expectedLinks(newSourceModel(mdl), supermodelSources())
	if len(links) != len(expected) {
		return fmt.Sprintf("%d supermodel links, %d expected", len(links), len(expected))
	}

	nodeCount := func(file string) (int, []string, string) {
		result := kotor.Exec("jq", []string{"-r", "(.nodes | length), (.animations // [] | .[].name)", file}, nil)
		if result.Status != 0 {
			return 0, nil, "jq: " + kotor.FirstLine(result.Stderr)
		}
		lines := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
		count, err := strconv.Atoi(lines[0])
		if err != nil {
			return 0, nil, err.Error()
		}
		return count, lines[1:], ""
	}
	ownNodes, _, problem := nodeCount(path + ".gltf")
	if problem != "" {
		return problem
	}

	for i, link := range links {
		want := expected[i]
		switch {
		case link.Model != want.Model:
			return fmt.Sprintf("link %d names %s, expected %s", i, link.Model, want.Model)
		case !slices.Equal(link.Animations, want.Animations):
			return fmt.Sprintf("link %s lists other animations than expected", link.Model)
		case !slices.Equal(link.Nodes, want.Nodes):
			return fmt.Sprintf("link %s pairs other nodes than expected", link.Model)
		}

		source := supermodelSources()[strings.ToLower(link.Model)]
		file := filepath.Join(filepath.Dir(path), filepath.FromSlash(link.Uri))
		target := filepath.Join(kotor.Extract, kotor.Build.Data, source.archive, source.resref+".anim.gltf")
		if filepath.Clean(file) != target {
			return fmt.Sprintf("link %s points to %s, expected %s", link.Model, link.Uri, target)
		}
		if _, err := os.Stat(file); err != nil {
			return "missing shared animation file " + link.Uri
		}

		sharedNodes, names, problem := nodeCount(file)
		if problem != "" {
			return problem
		}
		for _, animation := range link.Animations {
			if !slices.Contains(names, animation) {
				return fmt.Sprintf("%s does not hold the animation %s", link.Uri, animation)
			}
		}
		for _, pair := range link.Nodes {
			if pair[0] < 1 || pair[0] >= ownNodes || pair[1] < 1 || pair[1] >= sharedNodes {
				return fmt.Sprintf("link %s pairs nodes outside the files", link.Model)
			}
		}
	}
	return ""
}
