// Command extract converts all supported game assets into ~/re/extract/<platform> and writes each parser's report
// to ~/re/temp/reports/<platform>. It does not check the outputs with independent tools; the verify command does that.
// Movies and the Xbox WMA files require ffmpeg. All stages run at the same time. Each stage writes its own outputs and reports
// and keeps its own summary lines, which are printed in stage order at the end.
//
// usage: go run ./cmd/extract [-platform pc|xbox] [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui] [erfs] [lips] [modules] [rims] [bifs] [tlk] [saves]   (default: all)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"re/kotor"
)

func packErf(pack string) []kotor.ErfEntry {
	return kotor.ReadErf(filepath.Join(kotor.Game, "TexturePacks", pack+".erf"))
}

// prune removes files with the given extensions from dir and its subfolders that were not written in this
// run. keep holds paths relative to dir.
func prune(dir string, keep map[string]bool, exts []string) {
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		relative, _ := filepath.Rel(dir, path)
		if slices.Contains(exts, filepath.Ext(path)) && !keep[relative] {
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		kotor.Fatal(err.Error())
	}
}

// texturePaths maps each texture resref to its PNG file, relative to extract. On PC the textures are those
// of the texture packs in kotor.Build.MaterialPacks. On Xbox they are the TXB textures of the BIF archives in
// kotor.Build.ResourceBifs and kotor.Build.ModelBifs, and then those of the ERF archives in kotor.Build.Erfs.
// The first pack or archive in that order wins, so they are read in reverse. The TXB textures of the RIM archives are added by rimTexturePaths.
func texturePaths() map[string]string {
	paths := map[string]string{}
	for _, pack := range slices.Backward(kotor.Build.MaterialPacks) {
		for _, entry := range packErf(pack) {
			if entry.Ext == "tpc" {
				paths[entry.Resref] = filepath.Join("TexturePacks", pack, entry.Resref+".png")
			}
		}
	}

	// The TXB textures of the ERF archives rank below those of the BIF archives.
	for _, name := range kotor.Build.Erfs {
		for _, entry := range kotor.ReadErf(filepath.Join(kotor.Game, name+".erf")) {
			if entry.Ext == "txb" {
				paths[entry.Resref] = filepath.Join(name, entry.Resref+".png")
			}
		}
	}

	key := kotor.ReadKey()
	for _, name := range slices.Backward(slices.Concat(kotor.Build.ResourceBifs, kotor.Build.ModelBifs)) {
		bif := slices.Index(key.Bifs, kotor.BifPath(name))
		for _, entry := range key.Keys {
			if int(entry.Bif) == bif && entry.Type == kotor.TypeTxb {
				paths[entry.Resref] = filepath.Join(kotor.Build.Data, name, entry.Resref+".png")
			}
		}
	}
	return paths
}

// modelCounts collects what the models of one stage leave out, for the stage summary.
type modelCounts struct {
	skipped         int
	hidden          int
	normalsReplaced int
	normalsOmitted  int
	missing         map[string]bool
}

// supermodelModel is a model of the BIF archives that other models can name as their supermodel.
type supermodelModel struct {
	tree    kotor.MdlTree
	archive string
	resref  string
}

// supermodelIndex holds the models of the BIF archives in kotor.Build.ModelBifs by lowercase resref, and the
// resrefs of those that provide animations to another model of these archives. Each of the latter gets a
// shared animation file. Supermodel names are resolved only against these archives.
type supermodelIndex struct {
	models map[string]supermodelModel
	shared map[string]bool
}

// supermodels reads the index once, because the stages run at the same time.
var supermodels = sync.OnceValue(func() supermodelIndex {
	key := kotor.ReadKey()
	index := supermodelIndex{models: map[string]supermodelModel{}, shared: map[string]bool{}}
	for _, name := range kotor.Build.ModelBifs {
		for _, resource := range kotor.BifResources(kotor.BifPath(name), key) {
			if resource.Ext != "mdl" {
				continue
			}

			if tree, ok := kotor.ReadMdlTree(resource.Data); ok {
				index.models[strings.ToLower(resource.Resref)] = supermodelModel{tree: tree, archive: name, resref: resource.Resref}
			}
		}
	}

	for _, model := range index.models {
		for _, link := range kotor.SupermodelLinks(model.tree, index.lookup(kotor.Extract)) {
			index.shared[strings.ToLower(link.Model)] = true
		}
	}
	return index
})

// sharedAnimationSuffix ends the name of the shared animation file of a supermodel, <resref>.anim.gltf, and of
// its buffer, <resref>.anim.bin.
const sharedAnimationSuffix = ".anim"

// lookup returns the lookup function of kotor.SupermodelLinks for a glTF file in the folder out. The URI of
// a shared animation file is relative to out.
func (index supermodelIndex) lookup(out string) func(string) (kotor.MdlTree, string, bool) {
	return func(name string) (kotor.MdlTree, string, bool) {
		model, known := index.models[strings.ToLower(name)]
		if !known {
			return kotor.MdlTree{}, "", false
		}

		target := filepath.Join(kotor.Extract, kotor.Build.Data, model.archive, model.resref+sharedAnimationSuffix+".gltf")
		relative, err := filepath.Rel(out, target)
		if err != nil {
			kotor.Fatal(err.Error())
		}
		return model.tree, filepath.ToSlash(relative), true
	}
}

// writeModel converts one MDL resource and its MDX data to <resref>.gltf and <resref>.bin in out. The glTF
// file refers to each texture by the path of its PNG file relative to out, and its scene extras list the
// animations that it inherits from its supermodel chain (see kotor.SupermodelLinks). It returns its report
// line and the names of the files it wrote. A model without rendered geometry and without animations of its
// own is counted and not written, even when it inherits animations. When the model provides animations to another model, its node
// tree and animations are also written to the shared animation file <resref>.anim.gltf and .anim.bin.
func writeModel(out, label, resref string, mdl, mdx []byte, texturePath map[string]string,
	counts *modelCounts) (string, []string) {
	imageUri := func(texture string) string {
		path, ok := texturePath[texture]
		if !ok {
			return ""
		}

		relative, err := filepath.Rel(out, filepath.Join(kotor.Extract, path))
		if err != nil {
			kotor.Fatal(err.Error())
		}
		return filepath.ToSlash(relative)
	}

	index := supermodels()
	var links []kotor.MdlSupermodelLink
	if tree, ok := kotor.ReadMdlTree(mdl); ok {
		links = kotor.SupermodelLinks(tree, index.lookup(out))
	}

	model := kotor.ConvertMdl(mdl, mdx, resref, resref+".bin", imageUri, kotor.MdlOptions{Supermodels: links, Xbox: kotor.Build.Xbox})
	counts.hidden += model.Stats.Hidden
	counts.normalsReplaced += model.Stats.NormalsReplaced
	counts.normalsOmitted += model.Stats.NormalsOmitted
	if len(model.Errors) > 0 {
		return kotor.ReportMdl(label, model), nil
	}
	if model.Stats.Meshes == 0 && model.Stats.Animations == 0 {
		counts.skipped++
		return kotor.ReportMdl(label, model), nil
	}

	for _, texture := range model.Textures {
		if _, ok := texturePath[texture]; !ok {
			counts.missing[texture] = true
		}
	}

	kotor.WriteFile(filepath.Join(out, resref+".gltf"), model.Gltf)
	kotor.WriteFile(filepath.Join(out, resref+".bin"), model.Bin)
	files := []string{resref + ".gltf", resref + ".bin"}

	// Only the BIF stage writes shared animation files, into the folder that the URIs of the links name.
	shared := resref + sharedAnimationSuffix
	owner, known := index.models[strings.ToLower(resref)]
	if known && index.shared[strings.ToLower(resref)] && filepath.Clean(out) == filepath.Join(kotor.Extract, kotor.Build.Data, owner.archive) {
		animations := kotor.ConvertMdl(mdl, mdx, shared, shared+".bin", nil, kotor.MdlOptions{AnimationsOnly: true, Xbox: kotor.Build.Xbox})
		if len(animations.Errors) > 0 {
			return label + " FAIL: shared animation file: " + animations.Errors[0], files
		}

		kotor.WriteFile(filepath.Join(out, shared+".gltf"), animations.Gltf)
		kotor.WriteFile(filepath.Join(out, shared+".bin"), animations.Bin)
		files = append(files, shared+".gltf", shared+".bin")
	}
	return kotor.ReportMdl(label, model), files
}

func noteModels(summary *kotor.Summary, stage string, counts *modelCounts) {
	missingNames := make([]string, 0, len(counts.missing))
	for texture := range counts.missing {
		missingNames = append(missingNames, texture)
	}
	slices.Sort(missingNames)
	*summary = append(*summary,
		fmt.Sprintf("%s: models without rendered geometry and animations skipped: %d", stage, counts.skipped),
		fmt.Sprintf("%s: non-rendered meshes omitted: %d", stage, counts.hidden),
		fmt.Sprintf("%s: zero normals replaced: %d", stage, counts.normalsReplaced),
		fmt.Sprintf("%s: meshes written without normals, because a normal is not unit length: %d", stage, counts.normalsOmitted),
		fmt.Sprintf("%s: model textures not found: %d (%s)", stage, len(missingNames), strings.Join(missingNames, " ")))
}

// Models: <data>/<archive>.bif -> extract/<data>/<archive>/*.gltf + *.bin (passing, with geometry or
// animations), one folder per archive in kotor.Build.ModelBifs, where <data> is kotor.Build.Data. The glTF node extras hold the decoded node headers, mesh headers,
// static controller values and face fields, and the model's animations and skins are glTF animations and
// skins. The walkmeshes of an archive become <resref>.<wok|dwk|pwk>.gltf + .bin in the same folder. The
// Xbox model archives also hold TXB textures, which writeResource converts into the same folder.
func convertModels(summary *kotor.Summary) {
	fmt.Println("models...")
	texturePath := texturePaths()
	materials := surfaceMaterials()
	key := kotor.ReadKey()

	var report []string
	var walkmeshReport []string
	counts := &modelCounts{missing: map[string]bool{}}
	walkmeshes := map[string]int{}
	var textureReport []string
	for _, name := range kotor.Build.ModelBifs {
		out := filepath.Join(kotor.Extract, kotor.Build.Data, name)
		kotor.MakeDir(out)

		resources := kotor.BifResources(kotor.BifPath(name), key)
		mdx := map[string][]byte{}
		for _, resource := range resources {
			if resource.Ext == "mdx" {
				mdx[resource.Resref] = resource.Data
			}
		}

		written := map[string]bool{}
		for _, resource := range resources {
			label := name + "/" + resource.Resref + "." + resource.Ext
			var line string
			var files []string
			switch resource.Ext {
			case "mdl":
				line, files = writeModel(out, label, resource.Resref, resource.Data, mdx[resource.Resref], texturePath, counts)
				report = append(report, line)

			case "wok", "dwk", "pwk":
				line, files = writeWalkmesh(out, resource, materials)
				walkmeshReport = append(walkmeshReport, line)
				if files == nil {
					walkmeshes["empty "+resource.Ext]++
				}

			case "txb":
				line, files = writeResource(out, label, resource, nil, nil)
				textureReport = append(textureReport, line)
			}

			for _, file := range files {
				written[file] = true
			}
		}

		prune(out, written, []string{".gltf", ".bin", ".png", ".txi"})
	}

	summary.Tally("models", report)
	if len(textureReport) > 0 {
		summary.Tally("model_textures", textureReport)
	}
	noteModels(summary, "models", counts)
	summary.Tally("walkmeshes", walkmeshReport)
	summary.Note(fmt.Sprintf("walkmeshes: without faces, not written: %d wok, %d dwk, %d pwk",
		walkmeshes["empty wok"], walkmeshes["empty dwk"], walkmeshes["empty pwk"]))
}

// surfaceMaterial is one row of surfacemat.2da. The name is the row number and the label, such as
// "07_Nonwalk", because several rows share the label "CRAP".
type surfaceMaterial struct {
	row      int
	name     string
	walkable bool
}

// goldenAngle spreads the hues of consecutive materials apart.
const goldenAngle = 137.507764

// color gives each material its own hue. Walkable materials are bright and the others dark.
func (material surfaceMaterial) color() (float64, float64, float64) {
	hue := math.Mod(float64(material.row)*goldenAngle, 360) / 60
	value := 0.35
	if material.walkable {
		value = 0.9
	}

	const saturation = 0.6
	chroma := value * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue, 2)-1))
	rgb := [6][3]float64{{chroma, x, 0}, {x, chroma, 0}, {0, chroma, x}, {0, x, chroma}, {x, 0, chroma}, {chroma, 0, x}}[int(hue)%6]
	low := value - chroma
	return rgb[0] + low, rgb[1] + low, rgb[2] + low
}

// tables returns the resources of the archive that holds the 2DA tables, kotor.Build.TableArchive.
func tables() []kotor.Resource {
	archive := kotor.Build.TableArchive
	if filepath.Ext(archive) == ".bif" {
		return kotor.BifResources(archive, kotor.ReadKey())
	}

	rim, problem := kotor.ReadRim(kotor.ReadFile(filepath.Join(kotor.Game, kotor.Build.Rims, archive)))
	if problem != "" {
		kotor.Fatal(archive + ": " + problem)
	}
	resources := make([]kotor.Resource, len(rim.Entries))
	for i, entry := range rim.Entries {
		resources[i] = kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data}
	}
	return resources
}

// surfaceMaterials reads the rows of surfacemat.2da in the archive of the tables.
func surfaceMaterials() []surfaceMaterial {
	for _, resource := range tables() {
		if resource.Resref != "surfacemat" || resource.Ext != "2da" {
			continue
		}

		table := kotor.ParseTwoDa(resource.Data)
		if len(table.Errors) > 0 {
			kotor.Fatal("surfacemat.2da: " + table.Errors[0])
		}
		label := slices.Index(table.Columns, "label")
		walk := slices.Index(table.Columns, "walk")
		if label < 0 || walk < 0 {
			kotor.Fatal("surfacemat.2da: no label or walk column")
		}

		materials := make([]surfaceMaterial, len(table.Rows))
		for r, row := range table.Rows {
			name := fmt.Sprintf("%02d_%s", r, table.Cells[r][label])
			if row != fmt.Sprint(r) {
				kotor.Fatal("surfacemat.2da: row " + row + " out of order")
			}
			materials[r] = surfaceMaterial{row: r, name: name, walkable: table.Cells[r][walk] == "1"}
		}
		return materials
	}
	kotor.Fatal("surfacemat.2da not in " + kotor.Build.TableArchive)
	return nil
}

// writeWalkmesh converts one walkmesh to <resref>.<ext>.gltf and <resref>.<ext>.bin. Every row of
// surfacemat.2da becomes the material of the same index, and the mesh node's extras hold the values of
// walkmeshJson. It returns its report line and the names of the files it wrote. A walkmesh without faces is
// not written.
func writeWalkmesh(out string, resource kotor.Resource, materials []surfaceMaterial) (string, []string) {
	label := resource.Resref + "." + resource.Ext
	bwm := kotor.ParseBwm(resource.Data)
	line := kotor.ReportBwm(label, bwm)
	if len(bwm.Errors) > 0 || len(bwm.Faces) == 0 {
		return line, nil
	}

	for _, material := range bwm.Materials {
		if int(material) >= len(materials) {
			return line + " (material " + fmt.Sprint(material) + " not in surfacemat.2da)", nil
		}
	}

	colors := make([]kotor.BwmMaterial, len(materials))
	for i, material := range materials {
		red, green, blue := material.color()
		colors[i] = kotor.BwmMaterial{Name: material.name, Color: [3]float32{float32(red), float32(green), float32(blue)}}
	}

	text, bin, err := kotor.BwmGltf(bwm, label, label+".bin", colors, walkmeshJson(bwm))
	if err != nil {
		kotor.Fatal(err.Error())
	}

	kotor.WriteFile(filepath.Join(out, label+".gltf"), text)
	kotor.WriteFile(filepath.Join(out, label+".bin"), bin)
	return line, []string{label + ".gltf", label + ".bin"}
}

// Textures: TexturePacks/<pack>.erf -> extract/TexturePacks/<pack>/*.png (+ *.txi when present)
func convertTextures(summary *kotor.Summary) {
	for _, pack := range kotor.Build.TexturePacks {
		fmt.Printf("textures %s...\n", pack)
		out := filepath.Join(kotor.Extract, "TexturePacks", pack)
		kotor.MakeDir(out)

		var report []string
		written := map[string]bool{}
		for _, entry := range packErf(pack) {
			if entry.Ext != "tpc" {
				report = append(report, entry.Resref+"."+entry.Ext+" not a texture, skipped PASS")
				continue
			}

			resource := kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data()}
			line, files := writeResource(out, entry.Resref, resource, nil, nil)
			report = append(report, line)
			for _, file := range files {
				written[file] = true
			}
		}

		prune(out, written, []string{".png", ".txi"})
		summary.Tally("textures_"+pack, report)
	}
}

// TGA images and their TXI texture options from three groups of BIF archives:
// data/lightmaps*.bif -> extract/data/lightmaps, data/textures.bif -> extract/data/textures,
// data/legacy.bif -> extract/data/legacy. Images become 8-bit PNG with the TGA's channels (gray, RGB or
// RGBA), top row first. The TGA parser must consume each file exactly. The room visibility lists of the
// lightmap archives are converted by writeResource into the same folder. These groups exist only on PC.
//
// The Xbox build keeps one TGA image outside the archives: errortex/*.tga -> extract/errortex/*.png.
func convertTga(summary *kotor.Summary) {
	errortex := filepath.Join(kotor.Game, "errortex")
	var looseReport []string
	looseWritten := map[string]bool{}
	for _, file := range kotor.ListFiles(errortex, ".tga", false) {
		kotor.MakeDir(filepath.Join(kotor.Extract, "errortex"))
		resref := strings.TrimSuffix(file, filepath.Ext(file))
		resource := kotor.Resource{Resref: resref, Type: kotor.TypeTga, Ext: "tga", Data: kotor.ReadFile(filepath.Join(errortex, file))}
		line, files := writeResource(filepath.Join(kotor.Extract, "errortex"), "errortex/"+file, resource, nil, nil)
		looseReport = append(looseReport, line)
		for _, written := range files {
			looseWritten[written] = true
		}
	}
	if len(looseReport) > 0 {
		prune(filepath.Join(kotor.Extract, "errortex"), looseWritten, []string{".png"})
		summary.Tally("tga_errortex", looseReport)
	}

	key := kotor.ReadKey()
	for _, group := range kotor.Build.TgaGroups {
		fmt.Printf("tga %s...\n", group.Name)
		out := filepath.Join(kotor.Extract, kotor.Build.Data, group.Name)
		kotor.MakeDir(out)

		var report []string
		written := map[string]bool{}
		for _, bif := range key.Bifs {
			if !group.Pattern.MatchString(bif) {
				continue
			}

			for _, resource := range kotor.BifResources(bif, key) {
				label := filepath.Base(bif) + ":" + resource.Resref + "." + resource.Ext
				if resource.Ext == "vis" {
					line, files := writeResource(out, label, resource, nil, nil)
					report = append(report, line)
					for _, file := range files {
						written[file] = true
					}
					continue
				}
				if resource.Ext != "tga" && resource.Ext != "txi" {
					report = append(report, label+" not an image, skipped PASS")
					continue
				}

				file := resource.Resref + ".txi"
				if resource.Ext == "tga" {
					file = resource.Resref + ".png"
				}
				if written[file] {
					report = append(report, label+" FAIL: "+file+" already written from another archive")
					continue
				}

				if resource.Ext == "txi" {
					kotor.WriteFile(filepath.Join(out, file), resource.Data)
					written[file] = true
					report = append(report, fmt.Sprintf("%s %d bytes PASS", label, len(resource.Data)))
					continue
				}

				header := kotor.ParseTga(resource.Data)
				image := kotor.DecodeTga(resource.Data, header)
				report = append(report, kotor.ReportTga(label, header))
				if image == nil {
					continue
				}

				kotor.WriteFile(filepath.Join(out, file), kotor.EncodePng(image.Width, image.Height, image.Pix, image.Channels))
				written[file] = true
			}
		}

		prune(out, written, []string{".png", ".txi", ".json"})
		summary.Tally("tga_"+group.Name, report)
	}
}

// Stream audio: streammusic/*.wav, streamsounds/*.wav and streamwaves/**/*.wav -> extract/<folder>/**/*.mp3 | *.wav (PCM).
// All three folders use the layouts that kotor.UnwrapAudio detects. kotor.DecodeAdpcm copies 16-bit PCM
// unchanged.
// streamwaves has one subfolder per module and dialog, which the output keeps.
//
// The Xbox build also stores stream audio as WMA (Windows Media Audio) files. ffmpeg decodes each one to
// <name>.wma.wav, 16-bit PCM. The extension of the source stays in the name, because 50 music tracks exist
// both as WAV and as WMA files. Several ffmpeg processes run at once.
func convertAudio(summary *kotor.Summary, stage, folder string) {
	fmt.Printf("%s...\n", stage)
	src := filepath.Join(kotor.Game, folder)
	out := filepath.Join(kotor.Extract, folder)
	kotor.MakeDir(out)

	wavFiles := kotor.ListFiles(src, ".wav", true)
	wmaFiles := kotor.ListFiles(src, ".wma", true)
	if len(wmaFiles) > 0 && !kotor.Has("ffmpeg") {
		summary.Note(stage + ": ffmpeg missing: WMA files not converted")
		wmaFiles = nil
	}

	report := make([]string, len(wavFiles)+len(wmaFiles))
	outputs := make([]string, len(report))
	for i, file := range wavFiles {
		name := strings.TrimSuffix(file, filepath.Ext(file))
		unwrapped := kotor.UnwrapAudio(kotor.ReadFile(filepath.Join(src, file)))
		data, failure, info := unwrapped.Payload, unwrapped.Err, unwrapped.Info
		if failure == "" && unwrapped.Kind == "wav" {
			decoded := kotor.DecodeAdpcm(data)
			data, failure = decoded.Out, decoded.Err
			info += " " + decoded.Info
		}

		report[i] = fmt.Sprintf("%s %s %s %s", name, unwrapped.Kind, info, kotor.Verdict(failure))
		if failure == "" {
			outputs[i] = name + "." + unwrapped.Kind
			kotor.MakeDir(filepath.Join(out, filepath.Dir(outputs[i])))
			kotor.WriteFile(filepath.Join(out, outputs[i]), data)
		}
	}

	queue := make(chan int, len(wmaFiles))
	for i := range wmaFiles {
		queue <- i
	}
	close(queue)

	var group sync.WaitGroup
	for range runtime.NumCPU() {
		group.Go(func() {
			for i := range queue {
				file := wmaFiles[i]
				outFile := strings.TrimSuffix(file, filepath.Ext(file)) + ".wma.wav"
				kotor.MakeDir(filepath.Join(out, filepath.Dir(outFile)))
				result := kotor.Exec("ffmpeg", []string{
					"-v", "error", "-y", "-i", filepath.Join(src, file),
					"-map", "0:a", "-c:a", "pcm_s16le", "-f", "wav", filepath.Join(out, outFile),
				}, nil)

				slot := len(wavFiles) + i
				if result.Status != 0 || strings.TrimSpace(result.Stderr) != "" {
					report[slot] = file + " wma FAIL: ffmpeg: " + kotor.FirstLine(result.Stderr)
					continue
				}
				report[slot] = file + " wma PASS"
				outputs[slot] = outFile
			}
		})
	}
	group.Wait()

	written := map[string]bool{}
	for _, file := range outputs {
		if file != "" {
			written[file] = true
		}
	}
	prune(out, written, []string{".mp3", ".wav"})
	summary.Tally(stage, report)
}

// Movies: movies/*.bik (PC) or movies/*.xmv (Xbox) -> extract/movies/*.mp4 (H.264 High profile, yuv420p;
// AAC LC audio). Requires ffmpeg, which decodes Bink and XMV. The Bink header is parsed and reported first;
// an XMV file is reported by name only. One ffmpeg process does not keep all CPUs busy at this resolution,
// so movieJobs of them run at once, largest movie first, so that the last ones to finish are short.
// One job per cpusPerMovie CPUs keeps all CPUs busy.
// CRF (constant rate factor) 12 keeps every frame of every movie at 40 dB PSNR or more against the Bink
// frame, which verify requires. At CRF 18, frames in 12 movies fell as low as 36.5 dB. CRF 12 makes
// files about 1.7 times larger.
//
// ffmpeg's XMV demuxer ends every XMV movie of more than one packet with the message xmvEndError, after it has
// read the last packet. The packet chain of every movie ends exactly at the end of its file, and the converted
// movies hold every video frame, so this message alone is not a failure.
const xmvEndError = ": Input/output error"

const (
	cpusPerMovie      = 3
	movieCrf          = "12"
	movieAudioBitrate = "192k"
)

func convertMovies(summary *kotor.Summary) {
	if !kotor.Has("ffmpeg") {
		summary.Note("ffmpeg missing: movies not converted")
		return
	}

	fmt.Println("movies...")
	src := filepath.Join(kotor.Game, "movies")
	out := filepath.Join(kotor.Extract, "movies")
	kotor.MakeDir(out)

	files := kotor.ListFiles(src, kotor.Build.Movie, false)

	// Each movie's report line goes to its own slot, and written records the outputs ffmpeg produced.
	report := make([]string, len(files))
	written := make([]bool, len(files))
	convertMovie := func(i int) {
		file := files[i]
		name := strings.TrimSuffix(file, filepath.Ext(file))
		inFile := filepath.Join(src, file)
		outFile := filepath.Join(out, name+".mp4")
		line := file + " xmv PASS"
		if kotor.Build.Movie == ".bik" {
			movie := kotor.ParseBink(kotor.ReadFile(inFile))
			line = kotor.ReportBink(file, movie)
			if len(movie.Errors) > 0 {
				report[i] = line
				return
			}
		}

		result := kotor.Exec("ffmpeg", []string{
			"-v", "error", "-y", "-i", inFile,
			"-map", "0:v", "-map", "0:a?",
			"-c:v", "libx264", "-preset", "slow", "-crf", movieCrf, "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", movieAudioBitrate,
			"-movflags", "+faststart", outFile,
		}, nil)
		stderr := strings.TrimSpace(result.Stderr)
		if kotor.Build.Movie == ".xmv" {
			stderr = strings.TrimSpace(strings.TrimSuffix(stderr, inFile+xmvEndError))
		}
		if result.Status != 0 || stderr != "" {
			report[i] = name + ".mp4 FAIL: ffmpeg: " + kotor.FirstLine(stderr)
			return
		}

		written[i] = true
		report[i] = line
	}

	sizes := make([]int64, len(files))
	for i, file := range files {
		info, err := os.Stat(filepath.Join(src, file))
		if err != nil {
			kotor.Fatal(err.Error())
		}
		sizes[i] = info.Size()
	}
	order := make([]int, len(files))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return sizes[order[a]] > sizes[order[b]] })

	queue := make(chan int, len(order))
	for _, i := range order {
		queue <- i
	}
	close(queue)

	var group sync.WaitGroup
	for range max(1, runtime.NumCPU()/cpusPerMovie) {
		group.Go(func() {
			for i := range queue {
				convertMovie(i)
			}
		})
	}
	group.Wait()

	keep := map[string]bool{}
	for i, file := range files {
		if written[i] {
			keep[strings.TrimSuffix(file, filepath.Ext(file))+".mp4"] = true
		}
	}
	prune(out, keep, []string{".mp4"})
	summary.Tally("movies", report)
}

// GUI: resource type 2047 in <data>/gui.bif -> extract/<data>/gui/*.json, converted by writeResource. The GUI
// layouts of patch.erf are converted by the erfs stage.
func convertGui(summary *kotor.Summary) {
	fmt.Println("gui...")
	out := filepath.Join(kotor.Extract, kotor.Build.Data, "gui")
	kotor.MakeDir(out)

	var report []string
	written := map[string]bool{}
	for _, resource := range kotor.BifResources(kotor.BifPath("gui"), kotor.ReadKey()) {
		if resource.Type != kotor.TypeGui {
			continue
		}

		line, files := writeResource(out, kotor.Build.Data+"/gui/"+resource.Resref, resource, nil, nil)
		report = append(report, line)
		for _, file := range files {
			written[file] = true
		}
	}

	prune(out, written, []string{".json"})
	summary.Tally("gui", report)
}

// Erfs: <name>.erf -> extract/<name>/ for each archive in kotor.Build.Erfs, converted by writeResource. The
// PC archive patch.erf holds GUI layouts, textures, 2DA tables and one model. The Xbox archive players.erf
// holds the party models, their textures and conversations. An MDX resource is the vertex data of the model of
// the same name and is not converted on its own.
func convertErfs(summary *kotor.Summary) {
	for _, name := range kotor.Build.Erfs {
		fmt.Printf("%s...\n", name)
		out := filepath.Join(kotor.Extract, name)
		kotor.MakeDir(out)

		entries := kotor.ReadErf(filepath.Join(kotor.Game, name+".erf"))
		mdx := map[string][]byte{}
		for _, entry := range entries {
			if entry.Ext == "mdx" {
				mdx[entry.Resref] = entry.Data()
			}
		}

		inputs := newResourceInputs()
		var report []string
		written := map[string]bool{}
		for _, entry := range entries {
			label := entry.Resref + "." + entry.Ext
			if entry.Ext == "mdx" {
				problem := ""
				if !slices.ContainsFunc(entries, func(other kotor.ErfEntry) bool { return other.Resref == entry.Resref && other.Ext == "mdl" }) {
					problem = "no model of the same name"
				}
				report = append(report, label+" vertex data of the model "+kotor.Verdict(problem))
				continue
			}

			resource := kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data()}
			line, files := writeResource(out, label, resource, mdx[entry.Resref], inputs)
			report = append(report, line)
			for _, file := range files {
				written[file] = true
			}
		}

		prune(out, written, []string{".json", ".png", ".txi", ".csv", ".gltf", ".bin"})
		summary.Tally(name, report)
		noteModels(summary, name, inputs.counts)
	}
}

// Lips: <lips>/<module>.mod -> extract/<lips>/<module>/<resref>.csv, where <lips> is kotor.Build.Lips, with one "time_s,shape" line per keyframe.
// Each archive keeps its own folder, so names that occur in several archives are written once per archive.
// Empty archives produce no folder.
func convertLips(summary *kotor.Summary) {
	fmt.Println("lips...")
	src := filepath.Join(kotor.Game, kotor.Build.Lips)
	out := filepath.Join(kotor.Extract, kotor.Build.Lips)
	kotor.MakeDir(out)

	var report []string
	written := map[string]bool{}
	for _, archive := range kotor.ListFiles(src, ".mod", false) {
		module := strings.TrimSuffix(archive, filepath.Ext(archive))
		for _, entry := range kotor.ReadErf(filepath.Join(src, archive)) {
			label := module + "/" + entry.Resref
			if entry.Type != kotor.TypeLip {
				report = append(report, label+"."+entry.Ext+" FAIL: not a LIP resource")
				continue
			}

			lip := kotor.ParseLip(entry.Data())
			report = append(report, kotor.ReportLip(label, lip))
			if len(lip.Errors) > 0 {
				continue
			}

			file := filepath.Join(module, entry.Resref+".csv")
			kotor.MakeDir(filepath.Join(out, module))
			kotor.WriteFile(filepath.Join(out, file), kotor.LipCsv(lip))
			written[file] = true
		}
	}

	prune(out, written, []string{".csv"})
	summary.Tally("lips", report)
}

// dialogTlk reads the string table that localized strings and sound sets refer to.
func dialogTlk() *kotor.Tlk {
	tlk, problem := kotor.ReadTlk(kotor.ReadFile(filepath.Join(kotor.Game, "dialog.tlk")))
	if problem != "" {
		kotor.Fatal("dialog.tlk: " + problem)
	}
	return tlk
}

// gffFile converts a GFF resource to the JSON form of gffJson and returns the text, notes for the report
// line and an error text. The file type must match the resource extension, except for the extension "res",
// which saved games use for GFF files of several file types. The version must be V3.2. Unreferenced bytes
// are noted, not rejected, because 29 module files and 5 template files have them.
func gffFile(data []byte, ext string, tlk *kotor.Tlk) ([]byte, string, string) {
	gff, problem := kotor.ReadGff(data)
	typeMatches := ext == "res" || gff != nil && gff.FileType == fmt.Sprintf("%-4s", strings.ToUpper(ext))
	if problem == "" && (!typeMatches || gff.Version != "V3.2") {
		problem = "unexpected " + gff.FileType + gff.Version
	}
	if problem != "" {
		return nil, "", problem
	}

	unresolved := 0
	text, err := json.MarshalIndent(gffJson(gff.Root, tlk, &unresolved), "", "\t")
	if err != nil {
		return nil, "", err.Error()
	}

	notes := fmt.Sprintf(" %d structs %d fields", gff.Counts.Structs, gff.Counts.Fields)
	if ext == "res" {
		notes = " type=" + strings.TrimSpace(gff.FileType) + notes
	}
	for _, region := range []string{"idx", "list"} {
		if gff.Counts.UnusedBytes[region] > 0 {
			notes += fmt.Sprintf(" %d unreferenced %s bytes", gff.Counts.UnusedBytes[region], region)
		}
	}
	if unresolved > 0 {
		notes += fmt.Sprintf(" %d string numbers past dialog.tlk", unresolved)
	}
	return append(text, '\n'), notes, ""
}

// Modules: modules/<archive>.rim -> extract/modules/<archive>/, one folder per archive. A GFF resource
// becomes <resref>.<ext>.json in the form of gffJson, with localized strings resolved through dialog.tlk.
// A compiled script becomes <resref>.ncs.txt, its disassembly with the routine names of nwscript.nss.
// The extension stays in the name because an area's ARE and GIT resources share a resref. A GFF resource
// must parse, and its file type must match its resource type (see gffFile).
func convertModules(summary *kotor.Summary) {
	fmt.Println("modules...")
	tlk := dialogTlk()
	routines := kotor.ReadNwscript(kotor.ReadKey())
	src := filepath.Join(kotor.Game, "modules")
	out := filepath.Join(kotor.Extract, "modules")
	kotor.MakeDir(out)

	var report []string
	written := map[string]bool{}
	for _, archive := range kotor.ListFiles(src, ".rim", false) {
		name := strings.TrimSuffix(archive, filepath.Ext(archive))
		rim, problem := kotor.ReadRim(kotor.ReadFile(filepath.Join(src, archive)))
		if problem != "" {
			report = append(report, archive+" FAIL: "+problem)
			continue
		}
		kotor.MakeDir(filepath.Join(out, name))

		for _, entry := range rim.Entries {
			label := name + "/" + entry.Resref + "." + entry.Ext
			if entry.Ext == "ncs" {
				ncs := kotor.ParseNcs(entry.Data, routines)
				report = append(report, kotor.ReportNcs(label, ncs))
				if len(ncs.Errors) == 0 {
					file := filepath.Join(name, entry.Resref+".ncs.txt")
					kotor.WriteFile(filepath.Join(out, file), kotor.NcsListing(ncs, routines))
					written[file] = true
				}
				continue
			}

			text, notes, problem := gffFile(entry.Data, entry.Ext, tlk)
			if problem != "" {
				report = append(report, label+" FAIL: "+problem)
				continue
			}

			file := filepath.Join(name, entry.Resref+"."+entry.Ext+".json")
			kotor.WriteFile(filepath.Join(out, file), text)
			written[file] = true
			report = append(report, label+notes+" PASS")
		}
	}

	prune(out, written, []string{".json", ".txt"})
	summary.Tally("modules", report)
}

// resourceInputs holds what writeResource needs besides the resource: the string table for localized
// strings and sound sets, the engine routines for script listings, the texture paths for model materials,
// the surface materials of walkmeshes, and the counts of what the models leave out.
type resourceInputs struct {
	tlk         *kotor.Tlk
	routines    []kotor.NcsRoutine
	texturePath map[string]string
	materials   []surfaceMaterial
	counts      *modelCounts
}

func newResourceInputs() *resourceInputs {
	return &resourceInputs{
		tlk:         dialogTlk(),
		routines:    kotor.ReadNwscript(kotor.ReadKey()),
		texturePath: texturePaths(),
		materials:   surfaceMaterials(),
		counts:      &modelCounts{missing: map[string]bool{}},
	}
}

// writeResource converts one resource of a RIM or BIF archive into dir and returns its report line and the
// names of the files it wrote. mdx is the MDX data of a model and is ignored for other types.
//
//   - A model becomes <resref>.gltf and <resref>.bin (see writeModel).
//   - A walkmesh becomes <resref>.<ext>.gltf and .bin (see writeWalkmesh). A walkmesh without faces is not
//     written.
//   - A sound becomes <resref>.wav as 16-bit PCM: 16-bit PCM files are copied, and IMA ADPCM and 8-bit PCM
//     files are converted (see kotor.DecodeAdpcm).
//   - A 2DA table becomes <resref>.2da.csv (see kotor.TwoDaCsv).
//   - A sound set becomes <resref>.ssf.json (see ssfJson).
//   - A compiled script becomes <resref>.ncs.txt, its disassembly listing.
//   - A script source becomes <resref>.nss, its text decoded as Windows-1252 and written as UTF-8. The line
//     breaks are kept.
//   - A TXI file is copied to <resref>.txi.
//   - A texture (TPC, or TXB on Xbox) becomes <resref>.png, its top mipmap level as 8-bit RGBA, and
//     <resref>.txi, its TXI trailer without trailing NUL bytes, when the trailer holds text.
//   - A TGA image becomes <resref>.png, 8-bit with the TGA's channels.
//   - A GUI layout becomes <resref>.json in the form of gffJson. It must use every byte of its regions.
//   - A letter table becomes <resref>.ltr.json (see ltrJson).
//   - A room visibility list becomes <resref>.vis.json (see visJson). It needs no inputs.
//   - A room layout becomes <resref>.lyt.json (see lytJson). It needs no inputs.
//   - Every other type is a GFF file and becomes <resref>.<ext>.json (see gffFile).
func writeResource(dir, label string, resource kotor.Resource, mdx []byte, inputs *resourceInputs) (string, []string) {
	var line string
	var output []byte
	file := resource.Resref + "." + resource.Ext
	switch resource.Ext {
	case "mdl":
		return writeModel(dir, label, resource.Resref, resource.Data, mdx, inputs.texturePath, inputs.counts)

	case "wok", "dwk", "pwk":
		return writeWalkmesh(dir, resource, inputs.materials)

	case "wav":
		problem := kotor.RiffWalk(resource.Data)
		decoded := kotor.DecodeAdpcm(resource.Data)
		line = label + " " + decoded.Info + " " + kotor.Verdict(problem, decoded.Err)
		if problem == "" && decoded.Err == "" {
			output = decoded.Out
		}

	case "2da":
		table := kotor.ParseTwoDa(resource.Data)
		line = kotor.ReportTwoDa(label, table)
		file += ".csv"
		if len(table.Errors) == 0 {
			output = kotor.TwoDaCsv(table)
		}

	case "ssf":
		ssf := kotor.ParseSsf(resource.Data)
		line = kotor.ReportSsf(label, ssf)
		file += ".json"
		unresolved := 0
		text, err := json.MarshalIndent(ssfJson(ssf, inputs.tlk, &unresolved), "", "\t")
		if err == nil && unresolved > 0 {
			err = fmt.Errorf("%d string numbers past dialog.tlk", unresolved)
		}
		if err != nil {
			line = label + " FAIL: " + err.Error()
		}
		if len(ssf.Errors) == 0 && err == nil {
			output = append(text, '\n')
		}

	case "ncs":
		ncs := kotor.ParseNcs(resource.Data, inputs.routines)
		line = kotor.ReportNcs(label, ncs)
		file += ".txt"
		if len(ncs.Errors) == 0 {
			output = kotor.NcsListing(ncs, inputs.routines)
		}

	case "nss":
		problem := ""
		if bytes.IndexByte(resource.Data, 0) >= 0 {
			problem = "NUL byte in source text"
		}
		line = fmt.Sprintf("%s bytes=%d %s", label, len(resource.Data), kotor.Verdict(problem))
		if problem == "" {
			output = []byte(kotor.Windows1252(resource.Data))
		}

	case "txi":
		line = fmt.Sprintf("%s bytes=%d PASS", label, len(resource.Data))
		output = resource.Data

	case "tpc", "txb":
		texture := kotor.ParseTpc(resource.Data, resource.Ext == "txb")
		image := kotor.DecodeTpc(resource.Data, texture)
		line = kotor.ReportTpc(label, texture)
		if image == nil {
			return line, nil
		}

		files := []string{resource.Resref + ".png"}
		kotor.WriteFile(filepath.Join(dir, files[0]), kotor.EncodePng(image.Width, image.Height, image.Rgba, 4))

		txi := string(texture.Txi)
		if strings.TrimSpace(strings.ReplaceAll(txi, "\x00", "")) != "" {
			files = append(files, resource.Resref+".txi")
			kotor.WriteFile(filepath.Join(dir, files[1]), []byte(strings.TrimRight(txi, "\x00")))
		}
		return line, files

	case "tga":
		header := kotor.ParseTga(resource.Data)
		image := kotor.DecodeTga(resource.Data, header)
		line = kotor.ReportTga(label, header)
		file = resource.Resref + ".png"
		if image != nil {
			output = kotor.EncodePng(image.Width, image.Height, image.Pix, image.Channels)
		}

	case "gui":
		file = resource.Resref + ".json"
		gff, problem := kotor.ReadGff(resource.Data)
		if problem == "" && (gff.FileType != "GUI " || gff.Version != "V3.2") {
			problem = "unexpected " + gff.FileType + gff.Version
		}
		if problem == "" {
			for region, count := range gff.Counts.UnusedBytes {
				if count > 0 {
					problem = fmt.Sprintf("%d unused bytes in %s region", count, region)
				}
			}
		}

		// GUI files have no localized strings, so they need no string table.
		var text []byte
		if problem == "" {
			var err error
			text, err = json.MarshalIndent(gffJson(gff.Root, nil, nil), "", "\t")
			if err != nil {
				problem = err.Error()
			}
		}

		line = label + " " + kotor.Verdict(problem)
		if problem == "" {
			output = append(text, '\n')
		}

	case "vis":
		vis := kotor.ParseVis(resource.Data)
		line = kotor.ReportVis(label, vis)
		file += ".json"
		if len(vis.Errors) == 0 {
			text, err := json.MarshalIndent(visJson(vis), "", "\t")
			if err != nil {
				line = label + " FAIL: " + err.Error()
			} else {
				output = append(text, '\n')
			}
		}

	case "lyt":
		lyt := kotor.ParseLyt(resource.Data)
		line = kotor.ReportLyt(label, lyt)
		file += ".json"
		if len(lyt.Errors) == 0 {
			text, err := json.MarshalIndent(lytJson(lyt), "", "\t")
			if err != nil {
				line = label + " FAIL: " + err.Error()
			} else {
				output = append(text, '\n')
			}
		}

	case "ltr":
		ltr := kotor.ParseLtr(resource.Data)
		line = kotor.ReportLtr(label, ltr)
		file += ".json"
		if len(ltr.Errors) == 0 {
			text, err := json.MarshalIndent(ltrJson(ltr), "", "\t")
			if err != nil {
				line = label + " FAIL: " + err.Error()
			} else {
				output = append(text, '\n')
			}
		}

	default:
		text, notes, problem := gffFile(resource.Data, resource.Ext, inputs.tlk)
		line = label + notes + " " + kotor.Verdict(problem)
		file += ".json"
		output = text
	}

	if output == nil {
		return line, nil
	}
	kotor.WriteFile(filepath.Join(dir, file), output)
	return line, []string{file}
}

// Rims: <rims>/<archive>.rim -> extract/<rims>/<archive>/, one folder per archive, where <rims> is
// kotor.Build.Rims. Every archive whose name ends in "dx" is the partner of the archive of the same name
// without "dx". It holds the MDX data of the partner's models and, on Xbox, the TXB textures of the
// partner's models, and no other resources. Its textures are converted into its own folder, and the
// partner's models refer to them before the textures of texturePaths. Every other resource is converted by
// writeResource. Each archive also gets a report line with its resource count, so that empty archives are
// reported too. A model whose partner archive lacks its MDX data takes the MDX data of the same name from
// globaldx.rim, the partner of the archive that the game always keeps loaded. In the Xbox build, 27 effect
// models of the area archives need this. MDX data that no model uses is counted in the summary and not
// converted.
func convertRims(summary *kotor.Summary) {
	fmt.Println("rims...")
	inputs := newResourceInputs()
	src := filepath.Join(kotor.Game, kotor.Build.Rims)
	out := filepath.Join(kotor.Extract, kotor.Build.Rims)
	kotor.MakeDir(out)

	var report []string
	archives := map[string]*kotor.Rim{}
	for _, archive := range kotor.ListFiles(src, ".rim", false) {
		name := strings.TrimSuffix(archive, filepath.Ext(archive))
		rim, problem := kotor.ReadRim(kotor.ReadFile(filepath.Join(src, archive)))
		if problem != "" {
			report = append(report, archive+" FAIL: "+problem)
			continue
		}

		report = append(report, fmt.Sprintf("%s resources=%d flag=%d PASS", archive, len(rim.Entries), rim.Flag))
		archives[name] = rim
	}

	globalMdx := map[string][]byte{}
	if global, ok := archives["globaldx"]; ok {
		for _, entry := range global.Entries {
			if entry.Ext == "mdx" {
				globalMdx[entry.Resref] = entry.Data
			}
		}
	}

	written := map[string]bool{}
	used := map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(archives)) {
		if strings.HasSuffix(name, "dx") {
			continue
		}

		dir := filepath.Join(out, name)
		kotor.MakeDir(dir)
		mdx := map[string][]byte{}
		texturePath := inputs.texturePath
		partner, hasPartner := archives[name+"dx"]
		if hasPartner {
			partnerDir := filepath.Join(out, name+"dx")
			texturePath = maps.Clone(inputs.texturePath)
			for _, entry := range partner.Entries {
				label := name + "dx/" + entry.Resref + "." + entry.Ext
				switch entry.Ext {
				case "mdx":
					mdx[entry.Resref] = entry.Data

				case "txb":
					kotor.MakeDir(partnerDir)
					resource := kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data}
					line, files := writeResource(partnerDir, label, resource, nil, inputs)
					report = append(report, line)
					for _, file := range files {
						written[filepath.Join(name+"dx", file)] = true
					}
					texturePath[entry.Resref] = filepath.Join(kotor.Build.Rims, name+"dx", entry.Resref+".png")

				default:
					report = append(report, label+" FAIL: neither MDX data nor a texture")
				}
			}
		}

		archiveInputs := *inputs
		archiveInputs.texturePath = texturePath
		for _, entry := range archives[name].Entries {
			label := name + "/" + entry.Resref + "." + entry.Ext
			resource := kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data}
			vertices, own := mdx[entry.Resref]
			partnerName := name + "dx"
			if !own && entry.Ext == "mdl" {
				vertices, partnerName = globalMdx[entry.Resref], "globaldx"
			}
			line, files := writeResource(dir, label, resource, vertices, &archiveInputs)
			if entry.Ext == "mdl" && vertices != nil {
				used[partnerName+"/"+entry.Resref] = true
			}

			report = append(report, line)
			for _, file := range files {
				if written[filepath.Join(name, file)] {
					report = append(report, label+" FAIL: "+file+" already written from another resource")
				}
				written[filepath.Join(name, file)] = true
			}
		}

	}

	orphans := []string{}
	for name, archive := range archives {
		for _, entry := range archive.Entries {
			if strings.HasSuffix(name, "dx") && entry.Ext == "mdx" && !used[name+"/"+entry.Resref] {
				orphans = append(orphans, name+"/"+entry.Resref)
			}
		}
	}

	prune(out, written, []string{".gltf", ".bin", ".wav", ".csv", ".json", ".txt", ".png", ".txi"})
	summary.Tally("rims", report)
	noteModels(summary, "rims", inputs.counts)
	slices.Sort(orphans)
	summary.Note(fmt.Sprintf("rims: MDX data without a model, not converted: %d (%s)", len(orphans), strings.Join(orphans, " ")))
}

// Bifs: <data>/<archive>.bif -> extract/<data>/<archive>/, one folder per archive in kotor.Build.ResourceBifs.
// Each resource is converted by writeResource.
func convertBifs(summary *kotor.Summary) {
	fmt.Println("bifs...")
	inputs := newResourceInputs()
	key := kotor.ReadKey()

	var report []string
	for _, name := range kotor.Build.ResourceBifs {
		out := filepath.Join(kotor.Extract, kotor.Build.Data, name)
		kotor.MakeDir(out)

		written := map[string]bool{}
		for _, resource := range kotor.BifResources(kotor.BifPath(name), key) {
			label := name + "/" + resource.Resref + "." + resource.Ext
			line, files := writeResource(out, label, resource, nil, inputs)
			report = append(report, line)
			for _, file := range files {
				written[file] = true
			}
		}
		prune(out, written, []string{".wav", ".csv", ".json", ".txt", ".nss", ".txi", ".png"})
	}
	summary.Tally("bifs", report)
}

// Tlk: dialog.tlk -> extract/dialog.tlk.json, the string table as an object with "language", the u32 at
// header offset 8, and "entries", one object per entry in file order. Each entry has "strref", its
// position in the table; "flags"; "sound", the sound resref; and "text". The sound and text are empty
// strings when the entry has none. ReadTlk rejects every value this form leaves out.
func convertTlk(summary *kotor.Summary) {
	fmt.Println("tlk...")
	tlk := dialogTlk()

	entries := make([]any, len(tlk.Entries))
	withText, withSound := 0, 0
	for i, entry := range tlk.Entries {
		object := &orderedObject{}
		object.set("strref", number(float64(i), 64))
		object.set("flags", number(float64(entry.Flags), 64))
		object.set("sound", entry.Sound)
		object.set("text", entry.Text)
		entries[i] = object

		if entry.Text != "" {
			withText++
		}
		if entry.Sound != "" {
			withSound++
		}
	}

	object := &orderedObject{}
	object.set("language", number(float64(tlk.Language), 64))
	object.set("entries", entries)
	text, err := json.MarshalIndent(object, "", "\t")
	if err != nil {
		summary.Tally("tlk", []string{"dialog.tlk FAIL: " + err.Error()})
		return
	}

	kotor.WriteFile(filepath.Join(kotor.Extract, "dialog.tlk.json"), append(text, '\n'))
	line := fmt.Sprintf("dialog.tlk entries=%d text=%d sound=%d PASS", len(tlk.Entries), withText, withSound)
	summary.Tally("tlk", []string{line})
}

// Saves: saves/<save>/ -> extract/saves/<save>/, one folder per saved game. Files outside the save folders
// are not converted.
//
//   - A GFF file (*.res) becomes <name>.res.json in the form of gffJson. GLOBALVARS.res also becomes
//     GLOBALVARS.variables.json, its decoded variables (see globalsJson).
//   - The screenshot Screen.tga becomes Screen.png, 8-bit with the same channels.
//   - The archive SAVEGAME.sav becomes the folder SAVEGAME/. Each resource is converted by writeResource,
//     and a nested archive (type 2057, a saved module) becomes a subfolder named after its resref.
//     Every archive must have the layout that kotor.ErfLayoutProblem checks.
//
// The Xbox build has no saves folder. Its folder localvault holds a saved player character, a BIC file,
// which becomes extract/localvault/<name>.bic.json in the form of gffJson.
func convertSaves(summary *kotor.Summary) {
	fmt.Println("saves...")
	inputs := &resourceInputs{tlk: dialogTlk()}
	var report []string
	vault := filepath.Join(kotor.Game, "localvault")
	vaultWritten := map[string]bool{}
	for _, file := range kotor.ListFiles(vault, ".bic", false) {
		text, notes, problem := gffFile(kotor.ReadFile(filepath.Join(vault, file)), "bic", inputs.tlk)
		report = append(report, "localvault/"+file+notes+" "+kotor.Verdict(problem))
		if problem == "" {
			kotor.MakeDir(filepath.Join(kotor.Extract, "localvault"))
			kotor.WriteFile(filepath.Join(kotor.Extract, "localvault", file+".json"), text)
			vaultWritten[file+".json"] = true
		}
	}
	if len(vaultWritten) > 0 {
		prune(filepath.Join(kotor.Extract, "localvault"), vaultWritten, []string{".json"})
	}

	src := filepath.Join(kotor.Game, "saves")
	if _, err := os.Stat(src); err != nil {
		summary.Tally("saves", report)
		summary.Note("saves: no saves folder in this build")
		return
	}

	out := filepath.Join(kotor.Extract, "saves")
	kotor.MakeDir(out)

	written := map[string]bool{}
	write := func(file string, data []byte) {
		kotor.WriteFile(filepath.Join(out, file), data)
		written[file] = true
	}

	// convertArchive converts the resources of an ERF archive into the folder dir, relative to out.
	var convertArchive func(dir string, data []byte)
	convertArchive = func(dir string, data []byte) {
		entries := kotor.ErfEntries(bytes.NewReader(data), int64(len(data)), dir)
		problem := kotor.ErfLayoutProblem(entries, int64(len(data)))
		report = append(report, fmt.Sprintf("%s resources=%d %s", dir, len(entries), kotor.Verdict(problem)))
		if problem != "" {
			return
		}

		kotor.MakeDir(filepath.Join(out, dir))
		for _, entry := range entries {
			if entry.Ext == "sav" {
				convertArchive(filepath.Join(dir, entry.Resref), entry.Data())
				continue
			}

			label := filepath.Join(dir, entry.Resref+"."+entry.Ext)
			resource := kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data()}
			line, files := writeResource(filepath.Join(out, dir), label, resource, nil, inputs)
			report = append(report, line)
			for _, file := range files {
				written[filepath.Join(dir, file)] = true
			}
		}
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		kotor.Fatal(err.Error())
	}
	for _, save := range entries {
		if !save.IsDir() {
			continue
		}

		files, err := os.ReadDir(filepath.Join(src, save.Name()))
		if err != nil {
			kotor.Fatal(err.Error())
		}

		kotor.MakeDir(filepath.Join(out, save.Name()))
		for _, file := range files {
			name := file.Name()
			label := filepath.Join(save.Name(), name)
			data := kotor.ReadFile(filepath.Join(src, label))
			base := strings.TrimSuffix(name, filepath.Ext(name))
			switch strings.ToLower(filepath.Ext(name)) {
			case ".res":
				text, notes, problem := gffFile(data, "res", inputs.tlk)
				report = append(report, label+notes+" "+kotor.Verdict(problem))
				if problem != "" {
					continue
				}
				write(label+".json", text)
				if base != "GLOBALVARS" {
					continue
				}

				gff, _ := kotor.ReadGff(data)
				globals, problem := kotor.ReadGlobals(gff)
				if problem == "" {
					text, err = json.MarshalIndent(globalsJson(globals), "", "\t")
					if err != nil {
						problem = err.Error()
					}
				}
				if problem != "" {
					report = append(report, label+" variables FAIL: "+problem)
					continue
				}

				report = append(report, fmt.Sprintf("%s variables booleans=%d numbers=%d strings=%d locations=%d PASS",
					label, len(globals.Booleans), len(globals.Numbers), len(globals.Strings), len(globals.Locations)))
				write(filepath.Join(save.Name(), base+".variables.json"), append(text, '\n'))

			case ".tga":
				header := kotor.ParseTga(data)
				image := kotor.DecodeTga(data, header)
				report = append(report, kotor.ReportTga(label, header))
				if image != nil {
					write(filepath.Join(save.Name(), base+".png"), kotor.EncodePng(image.Width, image.Height, image.Pix, image.Channels))
				}

			case ".sav":
				convertArchive(filepath.Join(save.Name(), base), data)

			default:
				report = append(report, label+" FAIL: unknown file")
			}
		}
	}

	prune(out, written, []string{".json", ".png"})
	summary.Tally("saves", report)
}

// Movies start first because they take longest.
var stages = []kotor.Stage{
	{Name: "movies", Run: convertMovies},
	{Name: "models", Run: convertModels},
	{Name: "textures", Run: convertTextures},
	{Name: "tga", Run: convertTga},
	{Name: "music", Run: func(summary *kotor.Summary) { convertAudio(summary, "music", "streammusic") }},
	{Name: "sounds", Run: func(summary *kotor.Summary) { convertAudio(summary, "sounds", "streamsounds") }},
	{Name: "waves", Run: func(summary *kotor.Summary) { convertAudio(summary, "waves", "streamwaves") }},
	{Name: "gui", Run: convertGui},
	{Name: "erfs", Run: convertErfs},
	{Name: "lips", Run: convertLips},
	{Name: "modules", Run: convertModules},
	{Name: "rims", Run: convertRims},
	{Name: "bifs", Run: convertBifs},
	{Name: "tlk", Run: convertTlk},
	{Name: "saves", Run: convertSaves},
}

func main() {
	kotor.RunStages(stages, os.Args[1:])
}
