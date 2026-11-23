// Command convert converts all supported game assets into ~/re/assets and writes each parser's report
// to ~/re/temp/reports. It does not check the outputs with independent tools; the verify command does that.
// Movies require ffmpeg. All stages run at the same time. Each stage writes its own outputs and reports
// and keeps its own summary lines, which are printed in stage order at the end.
//
// usage: go run ./cmd/convert [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui]   (default: all)
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"re/kotor"
)

// summary holds the summary lines of one stage.
type summary []string

// tally writes a stage's report and adds its pass and fail counts to the summary.
func (lines *summary) tally(name string, report []string) {
	writeFile(filepath.Join(kotor.Reports, name+".txt"), []byte(strings.Join(report, "\n")+"\n"))
	pass := 0
	for _, line := range report {
		if strings.HasSuffix(line, "PASS") {
			pass++
		}
	}
	*lines = append(*lines, fmt.Sprintf("%s: %d pass, %d fail", name, pass, len(report)-pass))
}

func writeFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		kotor.Fatal(err.Error())
	}
}

func makeDir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		kotor.Fatal(err.Error())
	}
}

func packErf(pack string) *kotor.Erf {
	return kotor.ReadErf(filepath.Join(kotor.Game, "TexturePacks", pack+".erf"))
}

// prune removes files with the given extensions from dir and its subfolders that were not written in this
// run. keep holds paths relative to dir.
func prune(dir string, keep map[string]bool, exts []string) {
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		relative, _ := filepath.Rel(dir, path)
		if slices.Contains(exts, filepath.Ext(path)) && !keep[relative] {
			os.Remove(path)
		}
		return nil
	})
}

// Models: data/models.bif -> assets/data/models/*.obj + *.mtl (passing, with geometry)
func convertModels(lines *summary) {
	fmt.Println("models...")
	out := filepath.Join(kotor.Assets, "data", "models")
	makeDir(out)

	// The first pack in priority order wins, so the packs are read in reverse.
	texturePath := map[string]string{}
	for i := len(kotor.TexturePacks) - 1; i >= 0; i-- {
		pack := kotor.TexturePacks[i]
		for _, entry := range packErf(pack).Entries {
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

		writeFile(filepath.Join(out, resource.Resref+".obj"), []byte(model.Obj))
		writeFile(filepath.Join(out, resource.Resref+".mtl"), []byte(strings.Join(mtl, "\n")+"\n"))
		written[resource.Resref+".obj"] = true
		written[resource.Resref+".mtl"] = true
	}

	prune(out, written, []string{".obj", ".mtl"})
	lines.tally("models", report)

	missingNames := make([]string, 0, len(missing))
	for texture := range missing {
		missingNames = append(missingNames, texture)
	}
	slices.Sort(missingNames)
	*lines = append(*lines,
		fmt.Sprintf("models without rendered geometry skipped: %d", skipped),
		fmt.Sprintf("non-rendered meshes omitted: %d", hidden),
		fmt.Sprintf("model textures not found: %d (%s)", len(missingNames), strings.Join(missingNames, " ")))
}

// Textures: TexturePacks/<pack>.erf -> assets/TexturePacks/<pack>/*.png (+ *.txi when present)
func convertTextures(lines *summary) {
	for _, pack := range kotor.TexturePacks {
		fmt.Printf("textures %s...\n", pack)
		out := filepath.Join(kotor.Assets, "TexturePacks", pack)
		makeDir(out)

		var report []string
		written := map[string]bool{}
		for _, entry := range packErf(pack).Entries {
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

			writeFile(filepath.Join(out, entry.Resref+".png"), kotor.EncodePng(image.Width, image.Height, image.Rgba, 4))
			written[entry.Resref+".png"] = true

			txi := string(texture.Txi)
			if strings.TrimSpace(strings.ReplaceAll(txi, "\x00", "")) != "" {
				writeFile(filepath.Join(out, entry.Resref+".txi"), []byte(strings.TrimRight(txi, "\x00")))
				written[entry.Resref+".txi"] = true
			}
		}

		prune(out, written, []string{".png", ".txi"})
		lines.tally("textures_"+pack, report)
	}
}

// TGA images and their TXI texture options from three groups of BIF archives:
// data/lightmaps*.bif -> assets/data/lightmaps, data/textures.bif -> assets/data/textures,
// data/legacy.bif -> assets/data/legacy. Images become 8-bit PNG with the TGA's channels (gray, RGB or
// RGBA), top row first. The TGA parser must consume each file exactly.
func convertTga(lines *summary) {
	key := kotor.ReadKey()
	for _, group := range kotor.TgaGroups {
		fmt.Printf("tga %s...\n", group.Name)
		out := filepath.Join(kotor.Assets, "data", group.Name)
		makeDir(out)

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
					writeFile(filepath.Join(out, file), resource.Data)
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

				writeFile(filepath.Join(out, file), kotor.EncodePng(image.Width, image.Height, image.Pix, image.Channels))
				written[file] = true
			}
		}

		prune(out, written, []string{".png", ".txi"})
		lines.tally("tga_"+group.Name, report)
	}
}

// listFiles returns the paths of the files under dir whose names match pattern, relative to dir and sorted.
func listFiles(dir string, pattern *regexp.Regexp) []string {
	var names []string
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && path != dir && pattern.MatchString(entry.Name()) {
			relative, _ := filepath.Rel(dir, path)
			names = append(names, relative)
		}
		return err
	})
	slices.Sort(names)
	return names
}

var wavPattern = regexp.MustCompile(`(?i)\.wav$`)

// Stream audio: streammusic/*.wav, streamsounds/*.wav and streamwaves/**/*.wav -> assets/<folder>/**/*.mp3 | *.wav (PCM).
// All three folders use the layouts that kotor.UnwrapAudio detects. kotor.DecodeAdpcm copies PCM unchanged.
// streamwaves has one subfolder per module and dialog, which the output keeps.
func convertAudio(lines *summary, stage, folder string) {
	fmt.Printf("%s...\n", stage)
	src := filepath.Join(kotor.Game, folder)
	out := filepath.Join(kotor.Assets, folder)
	makeDir(out)

	var report []string
	written := map[string]bool{}
	for _, file := range listFiles(src, wavPattern) {
		name := wavPattern.ReplaceAllString(file, "")
		source, err := os.ReadFile(filepath.Join(src, file))
		if err != nil {
			kotor.Fatal(err.Error())
		}

		unwrapped := kotor.UnwrapAudio(source)
		data, failure, info := unwrapped.Payload, unwrapped.Err, unwrapped.Info
		if failure == "" && unwrapped.Kind == "wav" {
			decoded := kotor.DecodeAdpcm(data)
			data, failure = decoded.Out, decoded.Err
			info += " " + decoded.Info
		}

		verdict := "PASS"
		if failure != "" {
			verdict = "FAIL: " + failure
		}
		report = append(report, fmt.Sprintf("%s %s %s %s", name, unwrapped.Kind, info, verdict))
		if failure == "" {
			outFile := name + "." + unwrapped.Kind
			makeDir(filepath.Join(out, filepath.Dir(outFile)))
			writeFile(filepath.Join(out, outFile), data)
			written[outFile] = true
		}
	}

	prune(out, written, []string{".mp3", ".wav"})
	lines.tally(stage, report)
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

var bikPattern = regexp.MustCompile(`(?i)\.bik$`)

func convertMovies(lines *summary) {
	if !kotor.Has("ffmpeg") {
		*lines = append(*lines, "ffmpeg missing: movies not converted")
		return
	}

	fmt.Println("movies...")
	src := filepath.Join(kotor.Game, "movies")
	out := filepath.Join(kotor.Assets, "movies")
	makeDir(out)

	entries, err := os.ReadDir(src)
	if err != nil {
		kotor.Fatal(err.Error())
	}
	var files []string
	for _, entry := range entries {
		if bikPattern.MatchString(entry.Name()) {
			files = append(files, entry.Name())
		}
	}
	slices.Sort(files)

	// Each movie's report line goes to its own slot, and written records the outputs ffmpeg produced.
	report := make([]string, len(files))
	written := make([]bool, len(files))
	convertMovie := func(i int) {
		file := files[i]
		name := bikPattern.ReplaceAllString(file, "")
		inFile := filepath.Join(src, file)
		outFile := filepath.Join(out, name+".mp4")
		source, err := os.ReadFile(inFile)
		if err != nil {
			kotor.Fatal(err.Error())
		}

		movie := kotor.ParseBink(source)
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
		failure := strings.TrimSpace(result.Stderr)
		if failure == "" && result.StartError != nil {
			failure = result.StartError.Error()
		}
		if result.Status != 0 || failure != "" {
			report[i] = name + ".mp4 FAIL: ffmpeg: " + strings.SplitN(failure, "\n", 2)[0]
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
			keep[bikPattern.ReplaceAllString(file, "")+".mp4"] = true
		}
	}
	prune(out, keep, []string{".mp4"})
	lines.tally("movies", report)
}

// GUI: resource type 2047 in data/gui.bif -> assets/data/gui/*.json, and in patch.erf -> assets/patch/*.json.
// Five patch.erf resources replace gui.bif resources of the same name.
func convertGui(lines *summary) {
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
	for _, entry := range kotor.ReadErf(filepath.Join(kotor.Game, "patch.erf")).Entries {
		if entry.Type == kotor.TypeGui {
			patchGuis = append(patchGuis, kotor.Resource{Resref: entry.Resref, Data: entry.Data()})
		}
	}

	var report []string
	for _, src := range []source{{"data/gui", bifGuis}, {"patch", patchGuis}} {
		out := filepath.Join(kotor.Assets, src.dir)
		makeDir(out)

		written := map[string]bool{}
		for _, resource := range src.resources {
			gff := kotor.ReadGff(resource.Data, resource.Resref)
			if gff.FileType != "GUI " || gff.Version != "V3.2" {
				report = append(report, src.dir+"/"+resource.Resref+" FAIL: unexpected "+gff.FileType+gff.Version)
				continue
			}

			var text strings.Builder
			writeJson(&text, guiJson(gff.Root), 0)
			text.WriteString("\n")
			writeFile(filepath.Join(out, resource.Resref+".json"), []byte(text.String()))
			written[resource.Resref+".json"] = true
			report = append(report, src.dir+"/"+resource.Resref+" PASS")
		}

		prune(out, written, []string{".json"})
	}

	lines.tally("gui", report)
}

// Movies start first because they take longest.
var stages = []struct {
	name    string
	convert func(*summary)
}{
	{"movies", convertMovies},
	{"models", convertModels},
	{"textures", convertTextures},
	{"tga", convertTga},
	{"music", func(lines *summary) { convertAudio(lines, "music", "streammusic") }},
	{"sounds", func(lines *summary) { convertAudio(lines, "sounds", "streamsounds") }},
	{"waves", func(lines *summary) { convertAudio(lines, "waves", "streamwaves") }},
	{"gui", convertGui},
}

func main() {
	makeDir(kotor.Reports)
	wanted := os.Args[1:]

	results := map[string]*summary{}
	var group sync.WaitGroup
	for _, stage := range stages {
		if len(wanted) > 0 && !slices.Contains(wanted, stage.name) {
			continue
		}

		lines := &summary{}
		results[stage.name] = lines
		group.Go(func() { stage.convert(lines) })
	}
	group.Wait()

	var output []string
	for _, stage := range kotor.ReportOrder {
		if lines, ok := results[stage]; ok {
			output = append(output, *lines...)
		}
	}
	fmt.Println(strings.Join(output, "\n"))
}
