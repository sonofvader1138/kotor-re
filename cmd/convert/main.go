// Command convert converts all supported game assets into ~/re/assets and writes each parser's report
// to ~/re/temp/reports. It does not check the outputs with independent tools; the verify command does that.
// Movies require ffmpeg. All stages run at the same time. Each stage writes its own outputs and reports
// and keeps its own summary lines, which are printed in stage order at the end.
//
// usage: go run ./cmd/convert [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui] [lips] [modules] [rims]   (default: all)
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
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

// texturePaths maps each texture resref to its PNG file in the texture packs, relative to a folder two
// levels below assets. The first pack in priority order wins, so the packs are read in reverse.
func texturePaths() map[string]string {
	paths := map[string]string{}
	for _, pack := range slices.Backward(kotor.TexturePacks) {

		for _, entry := range packErf(pack) {
			if entry.Ext == "tpc" {
				paths[entry.Resref] = "../../TexturePacks/" + pack + "/" + entry.Resref + ".png"
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

// writeModel converts one MDL resource and its MDX data to <resref>.obj and <resref>.mtl in out. It
// returns its report line and the names of the files it wrote. A model without rendered geometry is
// counted and not written.
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
			mtl = append(mtl, "map_Kd "+path)
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

// Models: data/models.bif -> assets/data/models/*.obj + *.mtl (passing, with geometry)
func convertModels(summary *kotor.Summary) {
	fmt.Println("models...")
	out := filepath.Join(kotor.Assets, "data", "models")
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
	written := map[string]bool{}
	counts := &modelCounts{missing: map[string]bool{}}
	for _, resource := range resources {
		if resource.Ext == "mdl" {
			line, files := writeModel(out, resource.Resref, resource.Resref, resource.Data, mdx[resource.Resref], texturePath, counts)
			report = append(report, line)
			for _, file := range files {
				written[file] = true
			}
		}
	}

	prune(out, written, []string{".obj", ".mtl"})
	summary.Tally("models", report)
	noteModels(summary, "models", counts)
}

// Textures: TexturePacks/<pack>.erf -> assets/TexturePacks/<pack>/*.png (+ *.txi when present)
func convertTextures(summary *kotor.Summary) {
	for _, pack := range kotor.TexturePacks {
		fmt.Printf("textures %s...\n", pack)
		out := filepath.Join(kotor.Assets, "TexturePacks", pack)
		kotor.MakeDir(out)

		var report []string
		written := map[string]bool{}
		for _, entry := range packErf(pack) {
			if entry.Ext != "tpc" {
				report = append(report, entry.Resref+"."+entry.Ext+" not a texture, skipped PASS")
				continue
			}

			data := entry.Data()
			texture := kotor.ParseTpc(data)
			image := kotor.DecodeTpc(data, texture)
			report = append(report, kotor.ReportTpc(entry.Resref, texture))
			if image == nil {
				continue
			}

			kotor.WriteFile(filepath.Join(out, entry.Resref+".png"), kotor.EncodePng(image.Width, image.Height, image.Rgba, 4))
			written[entry.Resref+".png"] = true

			txi := string(texture.Txi)
			if strings.TrimSpace(strings.ReplaceAll(txi, "\x00", "")) != "" {
				kotor.WriteFile(filepath.Join(out, entry.Resref+".txi"), []byte(strings.TrimRight(txi, "\x00")))
				written[entry.Resref+".txi"] = true
			}
		}

		prune(out, written, []string{".png", ".txi"})
		summary.Tally("textures_"+pack, report)
	}
}

// TGA images and their TXI texture options from three groups of BIF archives:
// data/lightmaps*.bif -> assets/data/lightmaps, data/textures.bif -> assets/data/textures,
// data/legacy.bif -> assets/data/legacy. Images become 8-bit PNG with the TGA's channels (gray, RGB or
// RGBA), top row first. The TGA parser must consume each file exactly.
func convertTga(summary *kotor.Summary) {
	key := kotor.ReadKey()
	for _, group := range kotor.TgaGroups {
		fmt.Printf("tga %s...\n", group.Name)
		out := filepath.Join(kotor.Assets, "data", group.Name)
		kotor.MakeDir(out)

		var report []string
		written := map[string]bool{}
		for _, bif := range key.Bifs {
			if !group.Pattern.MatchString(bif) {
				continue
			}

			for _, resource := range kotor.BifResources(bif, key) {
				label := filepath.Base(bif) + ":" + resource.Resref + "." + resource.Ext
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

		prune(out, written, []string{".png", ".txi"})
		summary.Tally("tga_"+group.Name, report)
	}
}

// Stream audio: streammusic/*.wav, streamsounds/*.wav and streamwaves/**/*.wav -> assets/<folder>/**/*.mp3 | *.wav (PCM).
// All three folders use the layouts that kotor.UnwrapAudio detects. kotor.DecodeAdpcm copies PCM unchanged.
// streamwaves has one subfolder per module and dialog, which the output keeps.
func convertAudio(summary *kotor.Summary, stage, folder string) {
	fmt.Printf("%s...\n", stage)
	src := filepath.Join(kotor.Game, folder)
	out := filepath.Join(kotor.Assets, folder)
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

// Movies: movies/*.bik -> assets/movies/*.mp4 (H.264 High profile, yuv420p; AAC LC audio).
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
	out := filepath.Join(kotor.Assets, "movies")
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

// GUI: resource type 2047 in data/gui.bif -> assets/data/gui/*.json, and in patch.erf -> assets/patch/*.json.
// Five patch.erf resources replace gui.bif resources of the same name.
func convertGui(summary *kotor.Summary) {
	fmt.Println("gui...")
	type source struct {
		dir       string
		resources []kotor.Resource
	}

	var bifGuis, patchGuis []kotor.Resource
	for _, resource := range kotor.BifResources("data/gui.bif", kotor.ReadKey()) {
		if resource.Type == kotor.TypeGui {
			bifGuis = append(bifGuis, resource)
		}
	}
	for _, entry := range kotor.ReadErf(filepath.Join(kotor.Game, "patch.erf")) {
		if entry.Type == kotor.TypeGui {
			patchGuis = append(patchGuis, kotor.Resource{Resref: entry.Resref, Data: entry.Data()})
		}
	}

	var report []string
	for _, src := range []source{{"data/gui", bifGuis}, {"patch", patchGuis}} {
		out := filepath.Join(kotor.Assets, src.dir)
		kotor.MakeDir(out)

		written := map[string]bool{}
		for _, resource := range src.resources {
			gff, problem := kotor.ReadGff(resource.Data)
			if problem == "" {
				for region, count := range gff.Counts.UnusedBytes {
					if count > 0 {
						problem = fmt.Sprintf("%d unused bytes in %s region", count, region)
					}
				}
			}
			if problem != "" {
				report = append(report, src.dir+"/"+resource.Resref+" FAIL: "+problem)
				continue
			}
			if gff.FileType != "GUI " || gff.Version != "V3.2" {
				report = append(report, src.dir+"/"+resource.Resref+" FAIL: unexpected "+gff.FileType+gff.Version)
				continue
			}

			// GUI files have no localized strings, so they need no string table.
			text, err := json.MarshalIndent(gffJson(gff.Root, nil, nil), "", "\t")
			if err != nil {
				report = append(report, src.dir+"/"+resource.Resref+" FAIL: "+err.Error())
				continue
			}

			kotor.WriteFile(filepath.Join(out, resource.Resref+".json"), append(text, '\n'))
			written[resource.Resref+".json"] = true
			report = append(report, src.dir+"/"+resource.Resref+" PASS")
		}

		prune(out, written, []string{".json"})
	}

	summary.Tally("gui", report)
}

// Lips: lips/<module>.mod -> assets/lips/<module>/<resref>.csv, with one "time_s,shape" line per keyframe.
// Each archive keeps its own folder, so names that occur in several archives are written once per archive.
// Empty archives produce no folder.
func convertLips(summary *kotor.Summary) {
	fmt.Println("lips...")
	src := filepath.Join(kotor.Game, "lips")
	out := filepath.Join(kotor.Assets, "lips")
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
// line and an error text. The file type must match the resource extension, and the version must be V3.2.
// Unreferenced bytes are noted, not rejected, because 29 module files have them.
func gffFile(data []byte, ext string, tlk *kotor.Tlk) ([]byte, string, string) {
	gff, problem := kotor.ReadGff(data)
	if problem == "" && (gff.FileType != fmt.Sprintf("%-4s", strings.ToUpper(ext)) || gff.Version != "V3.2") {
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

// Modules: modules/<archive>.rim -> assets/modules/<archive>/, one folder per archive. A GFF resource
// becomes <resref>.<ext>.json in the form of gffJson, with localized strings resolved through dialog.tlk.
// A compiled script becomes <resref>.ncs.txt, its disassembly with the routine names of nwscript.nss.
// The extension stays in the name because an area's ARE and GIT resources share a resref. A GFF resource
// must parse, and its file type must match its resource type (see gffFile).
func convertModules(summary *kotor.Summary) {
	fmt.Println("modules...")
	tlk := dialogTlk()
	routines := kotor.ReadNwscript(kotor.ReadKey())
	src := filepath.Join(kotor.Game, "modules")
	out := filepath.Join(kotor.Assets, "modules")
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

// Rims: rims/<archive>.rim -> assets/rims/<archive>/, one folder per archive. Every archive whose name
// ends in "dx" holds the MDX data of the models in the archive of the same name without "dx", and no other
// resources. A model becomes <resref>.obj and <resref>.mtl. A sound becomes <resref>.wav as 16-bit PCM:
// PCM files are copied, and IMA ADPCM files are decoded. A 2DA table becomes <resref>.2da.csv (see
// kotor.TwoDaCsv), a sound set becomes <resref>.ssf.json (see ssfJson), an item template becomes
// <resref>.uti.json (see gffFile), and a compiled script becomes <resref>.ncs.txt. Each archive also gets
// a report line with its resource count, so that the four empty archives are reported too. MDX data
// without a model in the partner archive is counted in the summary and not converted.
func convertRims(summary *kotor.Summary) {
	fmt.Println("rims...")
	tlk := dialogTlk()
	routines := kotor.ReadNwscript(kotor.ReadKey())
	texturePath := texturePaths()
	src := filepath.Join(kotor.Game, "rims")
	out := filepath.Join(kotor.Assets, "rims")
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
	counts := &modelCounts{missing: map[string]bool{}}
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
			var files []string
			var line string
			switch entry.Ext {
			case "mdl":
				line, files = writeModel(dir, label, entry.Resref, entry.Data, mdx[entry.Resref], texturePath, counts)
				used[entry.Resref] = true

			case "wav":
				problem := kotor.RiffWalk(entry.Data)
				decoded := kotor.DecodeAdpcm(entry.Data)
				line = label + " " + decoded.Info + " " + kotor.Verdict(problem, decoded.Err)
				if problem == "" && decoded.Err == "" {
					kotor.WriteFile(filepath.Join(dir, entry.Resref+".wav"), decoded.Out)
					files = []string{entry.Resref + ".wav"}
				}

			case "2da":
				table := kotor.ParseTwoDa(entry.Data)
				line = kotor.ReportTwoDa(label, table)
				if len(table.Errors) == 0 {
					kotor.WriteFile(filepath.Join(dir, entry.Resref+".2da.csv"), kotor.TwoDaCsv(table))
					files = []string{entry.Resref + ".2da.csv"}
				}

			case "ssf":
				ssf := kotor.ParseSsf(entry.Data)
				line = kotor.ReportSsf(label, ssf)
				unresolved := 0
				text, err := json.MarshalIndent(ssfJson(ssf, tlk, &unresolved), "", "\t")
				if err == nil && unresolved > 0 {
					err = fmt.Errorf("%d string numbers past dialog.tlk", unresolved)
				}
				if err != nil {
					line = label + " FAIL: " + err.Error()
				}
				if len(ssf.Errors) == 0 && err == nil {
					kotor.WriteFile(filepath.Join(dir, entry.Resref+".ssf.json"), append(text, '\n'))
					files = []string{entry.Resref + ".ssf.json"}
				}

			case "ncs":
				ncs := kotor.ParseNcs(entry.Data, routines)
				line = kotor.ReportNcs(label, ncs)
				if len(ncs.Errors) == 0 {
					kotor.WriteFile(filepath.Join(dir, entry.Resref+".ncs.txt"), kotor.NcsListing(ncs, routines))
					files = []string{entry.Resref + ".ncs.txt"}
				}

			default:
				text, notes, problem := gffFile(entry.Data, entry.Ext, tlk)
				line = label + notes + " " + kotor.Verdict(problem)
				if problem == "" {
					kotor.WriteFile(filepath.Join(dir, entry.Resref+"."+entry.Ext+".json"), text)
					files = []string{entry.Resref + "." + entry.Ext + ".json"}
				}
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
	noteModels(summary, "rims", counts)
	slices.Sort(orphans)
	summary.Note(fmt.Sprintf("rims: MDX data without a model, not converted: %d (%s)", len(orphans), strings.Join(orphans, " ")))
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
	{Name: "lips", Run: convertLips},
	{Name: "modules", Run: convertModules},
	{Name: "rims", Run: convertRims},
}

func main() {
	kotor.RunStages(stages, os.Args[1:])
}
