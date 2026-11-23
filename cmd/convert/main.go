// Command convert converts all supported game assets into ~/re/assets and writes each parser's report
// to ~/re/temp/reports. It does not check the outputs with independent tools; the verify command does that.
// Movies require ffmpeg. All stages run at the same time. Each stage writes its own outputs and reports
// and keeps its own summary lines, which are printed in stage order at the end.
//
// usage: go run ./cmd/convert [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui] [lips]   (default: all)
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
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

// Models: data/models.bif -> assets/data/models/*.obj + *.mtl (passing, with geometry)
func convertModels(summary *kotor.Summary) {
	fmt.Println("models...")
	out := filepath.Join(kotor.Assets, "data", "models")
	kotor.MakeDir(out)

	// The first pack in priority order wins, so the packs are read in reverse.
	texturePath := map[string]string{}
	for _, pack := range slices.Backward(kotor.TexturePacks) {

		for _, entry := range packErf(pack) {
			if entry.Ext == "tpc" {
				texturePath[entry.Resref] = "../../TexturePacks/" + pack + "/" + entry.Resref + ".png"
			}
		}
	}

	resources := kotor.BifResources("data/models.bif", kotor.ReadKey())
	mdx := map[string][]byte{}
	for _, resource := range resources {
		if resource.Ext == "mdx" {
			mdx[resource.Resref] = resource.Data
		}
	}

	var report []string
	written := map[string]bool{}
	missing := map[string]bool{}
	skipped, hidden := 0, 0
	for _, resource := range resources {
		if resource.Ext != "mdl" {
			continue
		}

		model := kotor.ConvertMdl(resource.Data, mdx[resource.Resref], resource.Resref+".mdl", resource.Resref+".mtl")
		report = append(report, kotor.ReportMdl(resource.Resref, model))
		hidden += model.Stats.Hidden
		if len(model.Errors) > 0 {
			continue
		}
		if model.Stats.Meshes == 0 {
			skipped++
			continue
		}

		mtl := []string{"# materials for " + resource.Resref}
		for _, texture := range model.Textures {
			mtl = append(mtl, "", "newmtl "+texture, "Kd 1.000000 1.000000 1.000000")
			if path, ok := texturePath[texture]; ok {
				mtl = append(mtl, "map_Kd "+path)
			} else {
				mtl = append(mtl, "# texture not found in the texture packs")
				missing[texture] = true
			}
		}

		kotor.WriteFile(filepath.Join(out, resource.Resref+".obj"), []byte(model.Obj))
		kotor.WriteFile(filepath.Join(out, resource.Resref+".mtl"), []byte(strings.Join(mtl, "\n")+"\n"))
		written[resource.Resref+".obj"] = true
		written[resource.Resref+".mtl"] = true
	}

	prune(out, written, []string{".obj", ".mtl"})
	summary.Tally("models", report)

	missingNames := make([]string, 0, len(missing))
	for texture := range missing {
		missingNames = append(missingNames, texture)
	}
	slices.Sort(missingNames)
	*summary = append(*summary,
		fmt.Sprintf("models without rendered geometry skipped: %d", skipped),
		fmt.Sprintf("non-rendered meshes omitted: %d", hidden),
		fmt.Sprintf("model textures not found: %d (%s)", len(missingNames), strings.Join(missingNames, " ")))
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

			text, err := json.MarshalIndent(guiJson(gff.Root), "", " ")
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
}

func main() {
	kotor.RunStages(stages, os.Args[1:])
}
