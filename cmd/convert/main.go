// Command convert converts all supported game assets into ~/re/extracted and writes each parser's report
// to ~/re/temp/reports. It does not check the outputs with independent tools; the verify command does that.
// Movies require ffmpeg. All stages run at the same time. Each stage writes its own outputs and reports
// and keeps its own summary lines, which are printed in stage order at the end.
//
// usage: go run ./cmd/convert [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui] [patch] [lips] [modules] [rims] [bifs] [tlk] [saves]   (default: all)
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

// texturePaths maps each texture resref to its PNG file in the texture packs, relative to extracted. The first
// pack in priority order wins, so the packs are read in reverse.
func texturePaths() map[string]string {
	paths := map[string]string{}
	for _, pack := range slices.Backward(kotor.MaterialPacks) {

		for _, entry := range packErf(pack) {
			if entry.Ext == "tpc" {
				paths[entry.Resref] = filepath.Join("TexturePacks", pack, entry.Resref+".png")
			}
		}
	}
	return paths
}

// modelCounts collects what the models of one stage leave out, for the stage summary.
type modelCounts struct {
	skipped int
	hidden  int
	missing map[string]bool
}

// writeModel converts one MDL resource and its MDX data to <resref>.obj and <resref>.mtl in out. The MTL
// file refers to each texture by its path relative to out. It returns its report line and the names of the
// files it wrote. A model without rendered geometry is counted and not written.
func writeModel(out, label, resref string, mdl, mdx []byte, texturePath map[string]string,
	counts *modelCounts) (string, []string) {
	model := kotor.ConvertMdl(mdl, mdx, resref+".mdl", resref+".mtl")
	counts.hidden += model.Stats.Hidden
	if len(model.Errors) > 0 {
		return kotor.ReportMdl(label, model), nil
	}
	if model.Stats.Meshes == 0 {
		counts.skipped++
		return kotor.ReportMdl(label, model), nil
	}

	mtl := []string{"# materials for " + resref}
	for _, texture := range model.Textures {
		mtl = append(mtl, "", "newmtl "+texture, "Kd 1.000000 1.000000 1.000000")
		if path, ok := texturePath[texture]; ok {
			relative, err := filepath.Rel(out, filepath.Join(kotor.Extracted, path))
			if err != nil {
				kotor.Fatal(err.Error())
			}
			mtl = append(mtl, "map_Kd "+filepath.ToSlash(relative))
		} else {
			mtl = append(mtl, "# texture not found in the texture packs")
			counts.missing[texture] = true
		}
	}

	kotor.WriteFile(filepath.Join(out, resref+".obj"), []byte(model.Obj))
	kotor.WriteFile(filepath.Join(out, resref+".mtl"), []byte(strings.Join(mtl, "\n")+"\n"))
	return kotor.ReportMdl(label, model), []string{resref + ".obj", resref + ".mtl"}
}

func noteModels(summary *kotor.Summary, stage string, counts *modelCounts) {
	missingNames := make([]string, 0, len(counts.missing))
	for texture := range counts.missing {
		missingNames = append(missingNames, texture)
	}
	slices.Sort(missingNames)
	*summary = append(*summary,
		fmt.Sprintf("%s: models without rendered geometry skipped: %d", stage, counts.skipped),
		fmt.Sprintf("%s: non-rendered meshes omitted: %d", stage, counts.hidden),
		fmt.Sprintf("%s: model textures not found: %d (%s)", stage, len(missingNames), strings.Join(missingNames, " ")))
}

// Models: data/models.bif -> extracted/data/models/*.obj + *.mtl (passing, with geometry), and the walkmeshes
// of the same archive -> extracted/data/models/<resref>.<wok|dwk|pwk>.obj + .json and walkmesh.mtl
func convertModels(summary *kotor.Summary) {
	fmt.Println("models...")
	out := filepath.Join(kotor.Extracted, "data", "models")
	kotor.MakeDir(out)

	texturePath := texturePaths()
	resources := kotor.BifResources("data/models.bif", kotor.ReadKey())
	mdx := map[string][]byte{}
	for _, resource := range resources {
		if resource.Ext == "mdx" {
			mdx[resource.Resref] = resource.Data
		}
	}

	var report []string
	var walkmeshReport []string
	written := map[string]bool{walkmeshMtl: true}
	counts := &modelCounts{missing: map[string]bool{}}
	materials := surfaceMaterials()
	walkmeshes := map[string]int{}
	for _, resource := range resources {
		var line string
		var files []string
		switch resource.Ext {
		case "mdl":
			line, files = writeModel(out, resource.Resref, resource.Resref, resource.Data, mdx[resource.Resref], texturePath, counts)
			report = append(report, line)

		case "wok", "dwk", "pwk":
			line, files = writeWalkmesh(out, resource, materials)
			walkmeshReport = append(walkmeshReport, line)
			if files == nil {
				walkmeshes["empty "+resource.Ext]++
			}
		}

		for _, file := range files {
			written[file] = true
		}
	}

	mtl := []string{"# surface materials of the walkmeshes, from surfacemat.2da"}
	for _, material := range materials {
		red, green, blue := material.color()
		mtl = append(mtl, "", "newmtl "+material.name, fmt.Sprintf("Kd %.6f %.6f %.6f", red, green, blue))
	}
	kotor.WriteFile(filepath.Join(out, walkmeshMtl), []byte(strings.Join(mtl, "\n")+"\n"))

	prune(out, written, []string{".obj", ".mtl", ".json"})
	summary.Tally("models", report)
	noteModels(summary, "models", counts)
	summary.Tally("walkmeshes", walkmeshReport)
	summary.Note(fmt.Sprintf("walkmeshes: without faces, not written: %d wok, %d dwk, %d pwk",
		walkmeshes["empty wok"], walkmeshes["empty dwk"], walkmeshes["empty pwk"]))
}

// walkmeshMtl is the material library that all walkmesh OBJ files share.
const walkmeshMtl = "walkmesh.mtl"

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

// surfaceMaterials reads the rows of surfacemat.2da in data/2da.bif.
func surfaceMaterials() []surfaceMaterial {
	for _, resource := range kotor.BifResources("data/2da.bif", kotor.ReadKey()) {
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
	kotor.Fatal("surfacemat.2da not in data/2da.bif")
	return nil
}

// writeWalkmesh converts one walkmesh to <resref>.<ext>.obj, which uses walkmeshMtl, and <resref>.<ext>.json
// (see walkmeshJson). It returns its report line and the names of the files it wrote. A walkmesh without
// faces is not written.
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

	obj := kotor.BwmObj(bwm, label, walkmeshMtl, func(material uint32) string { return materials[material].name })
	text, err := json.MarshalIndent(walkmeshJson(bwm), "", "\t")
	if err != nil {
		kotor.Fatal(err.Error())
	}

	kotor.WriteFile(filepath.Join(out, label+".obj"), []byte(obj))
	kotor.WriteFile(filepath.Join(out, label+".json"), append(text, '\n'))
	return line, []string{label + ".obj", label + ".json"}
}

// Textures: TexturePacks/<pack>.erf -> extracted/TexturePacks/<pack>/*.png (+ *.txi when present)
func convertTextures(summary *kotor.Summary) {
	for _, pack := range kotor.TexturePacks {
		fmt.Printf("textures %s...\n", pack)
		out := filepath.Join(kotor.Extracted, "TexturePacks", pack)
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
// data/lightmaps*.bif -> extracted/data/lightmaps, data/textures.bif -> extracted/data/textures,
// data/legacy.bif -> extracted/data/legacy. Images become 8-bit PNG with the TGA's channels (gray, RGB or
// RGBA), top row first. The TGA parser must consume each file exactly. The room visibility lists of the
// lightmap archives are converted by writeResource into the same folder.
func convertTga(summary *kotor.Summary) {
	key := kotor.ReadKey()
	for _, group := range kotor.TgaGroups {
		fmt.Printf("tga %s...\n", group.Name)
		out := filepath.Join(kotor.Extracted, "data", group.Name)
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

// Stream audio: streammusic/*.wav, streamsounds/*.wav and streamwaves/**/*.wav -> extracted/<folder>/**/*.mp3 | *.wav (PCM).
// All three folders use the layouts that kotor.UnwrapAudio detects. kotor.DecodeAdpcm copies 16-bit PCM
// unchanged.
// streamwaves has one subfolder per module and dialog, which the output keeps.
func convertAudio(summary *kotor.Summary, stage, folder string) {
	fmt.Printf("%s...\n", stage)
	src := filepath.Join(kotor.Game, folder)
	out := filepath.Join(kotor.Extracted, folder)
	kotor.MakeDir(out)

	var report []string
	written := map[string]bool{}
	for _, file := range kotor.ListFiles(src, ".wav", true) {
		name := strings.TrimSuffix(file, filepath.Ext(file))
		unwrapped := kotor.UnwrapAudio(kotor.ReadFile(filepath.Join(src, file)))
		data, failure, info := unwrapped.Payload, unwrapped.Err, unwrapped.Info
		if failure == "" && unwrapped.Kind == "wav" {
			decoded := kotor.DecodeAdpcm(data)
			data, failure = decoded.Out, decoded.Err
			info += " " + decoded.Info
		}

		report = append(report, fmt.Sprintf("%s %s %s %s", name, unwrapped.Kind, info, kotor.Verdict(failure)))
		if failure == "" {
			outFile := name + "." + unwrapped.Kind
			kotor.MakeDir(filepath.Join(out, filepath.Dir(outFile)))
			kotor.WriteFile(filepath.Join(out, outFile), data)
			written[outFile] = true
		}
	}

	prune(out, written, []string{".mp3", ".wav"})
	summary.Tally(stage, report)
}

// Movies: movies/*.bik -> extracted/movies/*.mp4 (H.264 High profile, yuv420p; AAC LC audio).
// Requires ffmpeg, which decodes Bink. One ffmpeg process does not keep all CPUs busy at this resolution,
// so movieJobs of them run at once, largest movie first, so that the last ones to finish are short.
// One job per cpusPerMovie CPUs keeps all CPUs busy.
// CRF (constant rate factor) 12 keeps every frame of every movie at 40 dB PSNR or more against the Bink
// frame, which verify requires. At CRF 18, frames in 12 movies fell as low as 36.5 dB. CRF 12 makes
// files about 1.7 times larger.
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
	out := filepath.Join(kotor.Extracted, "movies")
	kotor.MakeDir(out)

	files := kotor.ListFiles(src, ".bik", false)

	// Each movie's report line goes to its own slot, and written records the outputs ffmpeg produced.
	report := make([]string, len(files))
	written := make([]bool, len(files))
	convertMovie := func(i int) {
		file := files[i]
		name := strings.TrimSuffix(file, filepath.Ext(file))
		inFile := filepath.Join(src, file)
		outFile := filepath.Join(out, name+".mp4")
		movie := kotor.ParseBink(kotor.ReadFile(inFile))
		if len(movie.Errors) > 0 {
			report[i] = kotor.ReportBink(file, movie)
			return
		}

		result := kotor.Exec("ffmpeg", []string{
			"-v", "error", "-y", "-i", inFile,
			"-map", "0:v", "-map", "0:a?",
			"-c:v", "libx264", "-preset", "slow", "-crf", movieCrf, "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", movieAudioBitrate,
			"-movflags", "+faststart", outFile,
		}, nil)
		if result.Status != 0 || strings.TrimSpace(result.Stderr) != "" {
			report[i] = name + ".mp4 FAIL: ffmpeg: " + kotor.FirstLine(result.Stderr)
			return
		}

		written[i] = true
		report[i] = kotor.ReportBink(file, movie)
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

// GUI: resource type 2047 in data/gui.bif -> extracted/data/gui/*.json, converted by writeResource. The GUI
// layouts of patch.erf are converted by the patch stage.
func convertGui(summary *kotor.Summary) {
	fmt.Println("gui...")
	out := filepath.Join(kotor.Extracted, "data", "gui")
	kotor.MakeDir(out)

	var report []string
	written := map[string]bool{}
	for _, resource := range kotor.BifResources("data/gui.bif", kotor.ReadKey()) {
		if resource.Type != kotor.TypeGui {
			continue
		}

		line, files := writeResource(out, "data/gui/"+resource.Resref, resource, nil, nil)
		report = append(report, line)
		for _, file := range files {
			written[file] = true
		}
	}

	prune(out, written, []string{".json"})
	summary.Tally("gui", report)
}

// Patch: patch.erf -> extracted/patch/, converted by writeResource: GUI layouts, textures, 2DA tables and one
// model. The MDX resource is the vertex data of the model of the same name and is not converted on its own.
func convertPatch(summary *kotor.Summary) {
	fmt.Println("patch...")
	out := filepath.Join(kotor.Extracted, "patch")
	kotor.MakeDir(out)

	entries := kotor.ReadErf(filepath.Join(kotor.Game, "patch.erf"))
	mdx := map[string][]byte{}
	for _, entry := range entries {
		if entry.Ext == "mdx" {
			mdx[entry.Resref] = entry.Data()
		}
	}

	inputs := &resourceInputs{texturePath: texturePaths(), counts: &modelCounts{missing: map[string]bool{}}}
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

	prune(out, written, []string{".json", ".png", ".txi", ".csv", ".obj", ".mtl"})
	summary.Tally("patch", report)
	noteModels(summary, "patch", inputs.counts)
}

// Lips: lips/<module>.mod -> extracted/lips/<module>/<resref>.csv, with one "time_s,shape" line per keyframe.
// Each archive keeps its own folder, so names that occur in several archives are written once per archive.
// Empty archives produce no folder.
func convertLips(summary *kotor.Summary) {
	fmt.Println("lips...")
	src := filepath.Join(kotor.Game, "lips")
	out := filepath.Join(kotor.Extracted, "lips")
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

// Modules: modules/<archive>.rim -> extracted/modules/<archive>/, one folder per archive. A GFF resource
// becomes <resref>.<ext>.json in the form of gffJson, with localized strings resolved through dialog.tlk.
// A compiled script becomes <resref>.ncs.txt, its disassembly with the routine names of nwscript.nss.
// The extension stays in the name because an area's ARE and GIT resources share a resref. A GFF resource
// must parse, and its file type must match its resource type (see gffFile).
func convertModules(summary *kotor.Summary) {
	fmt.Println("modules...")
	tlk := dialogTlk()
	routines := kotor.ReadNwscript(kotor.ReadKey())
	src := filepath.Join(kotor.Game, "modules")
	out := filepath.Join(kotor.Extracted, "modules")
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
// and the counts of what the models leave out.
type resourceInputs struct {
	tlk         *kotor.Tlk
	routines    []kotor.NcsRoutine
	texturePath map[string]string
	counts      *modelCounts
}

func newResourceInputs() *resourceInputs {
	return &resourceInputs{
		tlk:         dialogTlk(),
		routines:    kotor.ReadNwscript(kotor.ReadKey()),
		texturePath: texturePaths(),
		counts:      &modelCounts{missing: map[string]bool{}},
	}
}

// writeResource converts one resource of a RIM or BIF archive into dir and returns its report line and the
// names of the files it wrote. mdx is the MDX data of a model and is ignored for other types.
//
//   - A model becomes <resref>.obj and <resref>.mtl (see writeModel).
//   - A sound becomes <resref>.wav as 16-bit PCM: 16-bit PCM files are copied, and IMA ADPCM and 8-bit PCM
//     files are converted (see kotor.DecodeAdpcm).
//   - A 2DA table becomes <resref>.2da.csv (see kotor.TwoDaCsv).
//   - A sound set becomes <resref>.ssf.json (see ssfJson).
//   - A compiled script becomes <resref>.ncs.txt, its disassembly listing.
//   - A script source becomes <resref>.nss, its text decoded as Windows-1252 and written as UTF-8. The line
//     breaks are kept.
//   - A TXI file is copied to <resref>.txi.
//   - A texture becomes <resref>.png, its top mipmap level as 8-bit RGBA, and <resref>.txi, its TXI
//     trailer without trailing NUL bytes, when the trailer holds text.
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

	case "tpc":
		texture := kotor.ParseTpc(resource.Data)
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

// Rims: rims/<archive>.rim -> extracted/rims/<archive>/, one folder per archive. Every archive whose name
// ends in "dx" holds the MDX data of the models in the archive of the same name without "dx", and no other
// resources. Each resource is converted by writeResource. Each archive also gets a report line with its
// resource count, so that the four empty archives are reported too. MDX data without a model in the
// partner archive is counted in the summary and not converted.
func convertRims(summary *kotor.Summary) {
	fmt.Println("rims...")
	inputs := newResourceInputs()
	src := filepath.Join(kotor.Game, "rims")
	out := filepath.Join(kotor.Extracted, "rims")
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

	written := map[string]bool{}
	orphans := []string{}
	for _, name := range slices.Sorted(maps.Keys(archives)) {
		if strings.HasSuffix(name, "dx") {
			continue
		}

		mdx := map[string][]byte{}
		used := map[string]bool{}
		if partner, ok := archives[name+"dx"]; ok {
			for _, entry := range partner.Entries {
				if entry.Ext != "mdx" {
					report = append(report, name+"dx/"+entry.Resref+"."+entry.Ext+" FAIL: not MDX data")
					continue
				}
				mdx[entry.Resref] = entry.Data
			}
		}

		dir := filepath.Join(out, name)
		kotor.MakeDir(dir)
		for _, entry := range archives[name].Entries {
			label := name + "/" + entry.Resref + "." + entry.Ext
			resource := kotor.Resource{Resref: entry.Resref, Type: entry.Type, Ext: entry.Ext, Data: entry.Data}
			line, files := writeResource(dir, label, resource, mdx[entry.Resref], inputs)
			if entry.Ext == "mdl" {
				used[entry.Resref] = true
			}

			report = append(report, line)
			for _, file := range files {
				written[filepath.Join(name, file)] = true
			}
		}

		for resref := range mdx {
			if !used[resref] {
				orphans = append(orphans, name+"dx/"+resref)
			}
		}
	}

	prune(out, written, []string{".obj", ".mtl", ".wav", ".csv", ".json", ".txt"})
	summary.Tally("rims", report)
	noteModels(summary, "rims", inputs.counts)
	slices.Sort(orphans)
	summary.Note(fmt.Sprintf("rims: MDX data without a model, not converted: %d (%s)", len(orphans), strings.Join(orphans, " ")))
}

// Bifs: data/<archive>.bif -> extracted/data/<archive>/, one folder per archive in kotor.ResourceBifs. Each
// resource is converted by writeResource.
func convertBifs(summary *kotor.Summary) {
	fmt.Println("bifs...")
	inputs := newResourceInputs()
	key := kotor.ReadKey()

	var report []string
	for _, name := range kotor.ResourceBifs {
		out := filepath.Join(kotor.Extracted, "data", name)
		kotor.MakeDir(out)

		written := map[string]bool{}
		for _, resource := range kotor.BifResources("data/"+name+".bif", key) {
			label := name + "/" + resource.Resref + "." + resource.Ext
			line, files := writeResource(out, label, resource, nil, inputs)
			report = append(report, line)
			for _, file := range files {
				written[file] = true
			}
		}
		prune(out, written, []string{".wav", ".csv", ".json", ".txt", ".nss", ".txi"})
	}
	summary.Tally("bifs", report)
}

// Tlk: dialog.tlk -> extracted/dialog.tlk.json, the string table as an object with "language", the u32 at
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

	kotor.WriteFile(filepath.Join(kotor.Extracted, "dialog.tlk.json"), append(text, '\n'))
	line := fmt.Sprintf("dialog.tlk entries=%d text=%d sound=%d PASS", len(tlk.Entries), withText, withSound)
	summary.Tally("tlk", []string{line})
}

// Saves: saves/<save>/ -> extracted/saves/<save>/, one folder per saved game. Files outside the save folders
// are not converted.
//
//   - A GFF file (*.res) becomes <name>.res.json in the form of gffJson. GLOBALVARS.res also becomes
//     GLOBALVARS.variables.json, its decoded variables (see globalsJson).
//   - The screenshot Screen.tga becomes Screen.png, 8-bit with the same channels.
//   - The archive SAVEGAME.sav becomes the folder SAVEGAME/. Each resource is converted by writeResource,
//     and a nested archive (type 2057, a saved module) becomes a subfolder named after its resref.
//     Every archive must have the layout that kotor.ErfLayoutProblem checks.
func convertSaves(summary *kotor.Summary) {
	fmt.Println("saves...")
	inputs := &resourceInputs{tlk: dialogTlk()}
	src := filepath.Join(kotor.Game, "saves")
	out := filepath.Join(kotor.Extracted, "saves")
	kotor.MakeDir(out)

	var report []string
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
	{Name: "patch", Run: convertPatch},
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
