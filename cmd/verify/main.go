// Command verify checks the converted assets in ~/re/assets with independent tools and writes one report
// per check to ~/re/temp/reports. Run the convert command first. A stage whose tool is missing is reported
// as UNVERIFIED. All stages run at the same time, and their checks share jobSlots job slots, so that at
// most that many external programs run at once. Each report lists its lines in job order.
//
// models: assimp must load each OBJ with at least one mesh and the same face count as the file. The import
// is raw (-r): assimp's default post-processing merges meshes with identical geometry into one, and some
// models repeat a mesh at the same place, so the default import reports fewer faces than the file has.
// The raw import also skips assimp's validation of its own data structures. The OBJ parser still runs and
// still rejects faces whose vertex index is out of range.
// textures: ImageMagick's decoding of each PNG must equal the decoded TPC texture, and pngcheck must accept it.
// tga: ImageMagick's decoding of each PNG must equal its decoding of the source TGA, and pngcheck must
// accept the PNG. Each TXI must equal the source bytes.
// music, sounds, waves: ffprobe must read each file without error.
// movies: ffprobe must report the Bink file's size, frame count, sample rate, channel count and audio
// duration (within 0.1 s; ffmpeg's Bink audio decoder emits about 0.03 s more than the packets declare),
// and every output frame must have a PSNR of at least 40 dB against the decoded Bink frame.
// gui: jq must parse each JSON file.
//
// usage: go run ./cmd/verify [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui]   (default: all)
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"re/kotor"
)

// Two jobs per CPU keep every CPU busy while some jobs wait on their programs.
const jobsPerCpu = 2

var jobSlots = make(chan struct{}, runtime.NumCPU()*jobsPerCpu)

// item is a summary entry: a note, or a report with its name and lines.
type item struct {
	note  string
	name  string
	lines []string
}

type summary []item

func (items *summary) note(text string) {
	*items = append(*items, item{note: text})
}

func (items *summary) tally(name string, lines []string) {
	*items = append(*items, item{name: name, lines: lines})
}

// A job returns its report lines, usually one.
type job func() []string

func line(text string) []string {
	return []string{text}
}

// pool runs jobs in the shared slots and returns their lines in job order.
func pool(jobs []job) []string {
	results := make([][]string, len(jobs))
	var group sync.WaitGroup
	for i, run := range jobs {
		group.Go(func() {
			jobSlots <- struct{}{}
			results[i] = run()
			<-jobSlots
		})
	}
	group.Wait()
	return slices.Concat(results...)
}

var logMutex sync.Mutex

func log(message string) {
	logMutex.Lock()
	fmt.Println(message)
	logMutex.Unlock()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		kotor.Fatal(err.Error())
	}
	return data
}

// firstLine returns the first line of a program's error output, or of the start error when there is none.
func firstLine(result kotor.ExecResult) string {
	text := result.Stderr
	if text == "" && result.StartError != nil {
		text = result.StartError.Error()
	}
	return strings.SplitN(strings.TrimSpace(text), "\n", 2)[0]
}

// files lists the files in dir whose names end in ext, as sorted paths relative to dir. recursive
// includes subfolders. A missing dir yields no files, so that the caller reports each output as missing.
func files(dir, ext string, recursive bool) []string {
	var names []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}

		relative, _ := filepath.Rel(dir, path)
		if strings.HasSuffix(relative, ext) {
			names = append(names, relative)
		}
		if entry.IsDir() && !recursive {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		kotor.Fatal(err.Error())
	}

	slices.Sort(names)
	return names
}

// jsNumber converts text to a number like JavaScript's Number(): NaN when it is not a number.
func jsNumber(text string) float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return math.NaN()
	}
	return value
}

type decodedImage struct {
	err  string
	size string
	rgba []byte
}

// magickDecode decodes an image with ImageMagick into 8-bit RGBA, top row first. The size goes to stderr.
func magickDecode(format string, data []byte) decodedImage {
	result := kotor.Exec("convert", []string{
		format + ":-", "-auto-orient", "-format", "%w %h", "-write", "info:fd:2", "-depth", "8", "rgba:-",
	}, data)
	if result.Status != 0 {
		return decodedImage{err: firstLine(result)}
	}

	lines := strings.Split(strings.TrimSpace(result.Stderr), "\n")
	return decodedImage{size: lines[len(lines)-1], rgba: result.Stdout}
}

// pngCheck returns a report line that ends in PASS when the PNG decodes to the expected size and pixels.
func pngCheck(name, file, width, height string, rgba []byte) []string {
	if !exists(file) {
		return line(name + " FAIL: output missing")
	}

	decoded := magickDecode("png", readFile(file))
	if decoded.err != "" {
		return line(name + " FAIL: " + decoded.err)
	}
	if decoded.size != width+" "+height {
		return line(fmt.Sprintf("%s FAIL: size %s, expected %s %s", name, decoded.size, width, height))
	}
	if !bytes.Equal(decoded.rgba, rgba) {
		return line(name + " " + decoded.size + " FAIL: pixels differ")
	}
	return line(name + " " + decoded.size + " PASS")
}

// pngcheckDir runs pngcheck on every PNG in dir, pngcheckBatch files per run.
const pngcheckBatch = 200

var pngcheckOk = regexp.MustCompile(`^OK: (.*?) \(`)

func pngcheckDir(dir string) []string {
	names := files(dir, ".png", false)
	var jobs []job
	for chunk := range slices.Chunk(names, pngcheckBatch) {
		jobs = append(jobs, func() []string {
			paths := make([]string, len(chunk))
			for i, file := range chunk {
				paths[i] = filepath.Join(dir, file)
			}
			result := kotor.Exec("pngcheck", paths, nil)

			ok := map[string]bool{}
			for _, text := range strings.Split(string(result.Stdout), "\n") {
				if match := pngcheckOk.FindStringSubmatch(text); match != nil {
					ok[filepath.Base(match[1])] = true
				}
			}

			lines := make([]string, len(chunk))
			for i, file := range chunk {
				lines[i] = file + " FAIL: pngcheck"
				if ok[file] {
					lines[i] = file + " PASS"
				}
			}
			return lines
		})
	}
	return pool(jobs)
}

var (
	magick   = kotor.Has("convert")
	pngcheck = kotor.Has("pngcheck")
	ffprobe  = kotor.Has("ffprobe")
)

// imageChecks runs the pixel and structure checks of one directory at the same time.
func imageChecks(items *summary, name, dir string, jobs []job) {
	var pixelLines, structureLines []string
	var group sync.WaitGroup
	if magick {
		group.Go(func() { pixelLines = pool(jobs) })
	}
	if pngcheck {
		group.Go(func() { structureLines = pngcheckDir(dir) })
	}
	group.Wait()

	if magick {
		items.tally("imagemagick_"+name, pixelLines)
	} else {
		items.note("ImageMagick missing: " + name + " UNVERIFIED")
	}

	if pngcheck {
		items.tally("pngcheck_"+name, structureLines)
	} else {
		items.note("pngcheck missing: " + name + " PNG structure unchecked")
	}
}

var (
	meshesPattern = regexp.MustCompile(`(?m)^Meshes:\s+(\d+)`)
	facesPattern  = regexp.MustCompile(`(?m)^Faces:\s+(\d+)`)
)

// Models: assets/data/models/*.obj
func models(items *summary) {
	if !kotor.Has("assimp") {
		items.note("assimp missing: models UNVERIFIED")
		return
	}

	log("models...")
	dir := filepath.Join(kotor.Assets, "data", "models")
	var jobs []job
	for _, file := range files(dir, ".obj", false) {
		jobs = append(jobs, func() []string {
			objPath := filepath.Join(dir, file)
			faces := 0
			for _, text := range bytes.Split(readFile(objPath), []byte("\n")) {
				if bytes.HasPrefix(text, []byte("f ")) {
					faces++
				}
			}

			result := kotor.Exec("assimp", []string{"info", objPath, "-r"}, nil)
			meshes := meshesPattern.FindSubmatch(result.Stdout)
			loaded := facesPattern.FindSubmatch(result.Stdout)
			if result.Status != 0 || meshes == nil || loaded == nil {
				return line(file + " FAIL: assimp: " + firstLine(result))
			}

			var errs []string
			if jsNumber(string(meshes[1])) == 0 {
				errs = append(errs, "no meshes")
			}
			if jsNumber(string(loaded[1])) != float64(faces) {
				errs = append(errs, fmt.Sprintf("assimp loaded %s faces, file has %d", loaded[1], faces))
			}

			return line(fmt.Sprintf("%s meshes=%s faces=%s %s", file, meshes[1], loaded[1], kotor.Verdict(errs)))
		})
	}
	items.tally("assimp_models", pool(jobs))
}

// Textures: assets/TexturePacks/<pack>/*.png against the TPC textures in TexturePacks/<pack>.erf.
// Each job decodes its TPC texture when it starts, so only one texture per job slot is held in memory.
func textures(items *summary) {
	for _, pack := range kotor.TexturePacks {
		log("textures " + pack + "...")
		dir := filepath.Join(kotor.Assets, "TexturePacks", pack)
		erf := kotor.ReadErf(filepath.Join(kotor.Game, "TexturePacks", pack+".erf"))

		var jobs []job
		for _, entry := range erf.Entries {
			if entry.Ext != "tpc" {
				continue
			}

			jobs = append(jobs, func() []string {
				data := entry.Data()
				image := kotor.DecodeTpc(data, kotor.ParseTpc(data))
				name := entry.Resref + ".png"
				if image == nil {
					return line(name + " FAIL: source texture does not decode")
				}

				return pngCheck(name, filepath.Join(dir, name), strconv.Itoa(image.Width), strconv.Itoa(image.Height), image.Rgba)
			})
		}
		imageChecks(items, pack, dir, jobs)
	}
}

// TGA images: assets/data/<group>/*.png and *.txi against the BIF archives they came from
func tgaImages(items *summary) {
	key := kotor.ReadKey()
	for _, group := range kotor.TgaGroups {
		log("tga " + group.Name + "...")
		dir := filepath.Join(kotor.Assets, "data", group.Name)

		var txiJobs, jobs []job
		seen := map[string]bool{}
		for _, bif := range key.Bifs {
			if !group.Pattern.MatchString(bif) {
				continue
			}

			for _, resource := range kotor.BifResources(bif, key) {
				if resource.Ext != "tga" && resource.Ext != "txi" {
					continue
				}

				name := resource.Resref + ".txi"
				if resource.Ext == "tga" {
					name = resource.Resref + ".png"
				}
				file := filepath.Join(dir, name)

				// convert keeps the first archive's copy of a name and reports the others.
				if seen[name] {
					continue
				}
				seen[name] = true

				if resource.Ext == "txi" {
					txiJobs = append(txiJobs, func() []string {
						if !exists(file) {
							return line(name + " FAIL: output missing")
						}
						if !bytes.Equal(readFile(file), resource.Data) {
							return line(name + " FAIL: differs from source")
						}
						return line(name + " PASS")
					})
					continue
				}

				jobs = append(jobs, func() []string {
					source := magickDecode("tga", resource.Data)
					if source.err != "" {
						return line(name + " FAIL: source TGA: " + source.err)
					}

					width, height, _ := strings.Cut(source.size, " ")
					return pngCheck(name, file, width, height, source.rgba)
				})
			}
		}

		items.tally("txi_"+group.Name, pool(txiJobs))
		imageChecks(items, group.Name, dir, jobs)
	}
}

// Stream audio: assets/streammusic, assets/streamsounds and assets/streamwaves
func streamAudio(items *summary, stage, folder string) {
	if !ffprobe {
		items.note("ffprobe missing: " + stage + " UNVERIFIED")
		return
	}

	log(stage + "...")
	dir := filepath.Join(kotor.Assets, folder)
	audioFiles := append(files(dir, ".mp3", true), files(dir, ".wav", true)...)
	slices.Sort(audioFiles)

	jobs := make([]job, len(audioFiles))
	for i, file := range audioFiles {
		jobs[i] = func() []string {
			result := kotor.Exec("ffprobe", []string{"-v", "error", filepath.Join(dir, file)}, nil)
			if result.Status == 0 && strings.TrimSpace(result.Stderr) == "" {
				return line(file + " PASS")
			}
			return line(file + " FAIL: " + firstLine(result))
		}
	}
	items.tally("ffprobe_"+stage, pool(jobs))
}

// Movies: assets/movies/*.mp4 against movies/*.bik. The probe and the PSNR comparison of one movie run
// at the same time.
const minPsnr = 40

// ffmpeg's Bink audio decoder emits about 0.03 s more than the packets declare.
const audioDurationToleranceS = 0.1

// Bink audio is 16-bit.
const binkBytesPerSample = 2

// probeStream holds the ffprobe fields that the movie check reads. A missing field stays nil.
type probeStream struct {
	CodecType    string  `json:"codec_type"`
	CodecName    string  `json:"codec_name"`
	Width        *int    `json:"width"`
	Height       *int    `json:"height"`
	NbReadFrames *string `json:"nb_read_frames"`
	SampleRate   *string `json:"sample_rate"`
	Channels     *int    `json:"channels"`
	Duration     *string `json:"duration"`
}

// optional formats a missing ffprobe field as "undefined", as the JS check printed it.
func optional[T any](value *T) string {
	if value == nil {
		return "undefined"
	}
	return fmt.Sprint(*value)
}

func optionalNumber(value *string) float64 {
	if value == nil {
		return math.NaN()
	}
	return jsNumber(*value)
}

var (
	bikPattern  = regexp.MustCompile(`(?i)\.bik$`)
	psnrPattern = regexp.MustCompile(`PSNR [^\n\r]*average:(\S+) min:(\S+)`)
)

func movies(items *summary) {
	if !ffprobe || !kotor.Has("ffmpeg") {
		items.note("ffmpeg or ffprobe missing: movies UNVERIFIED")
		return
	}

	log("movies...")
	src := filepath.Join(kotor.Game, "movies")
	dir := filepath.Join(kotor.Assets, "movies")
	var jobs []job
	for _, file := range files(src, ".bik", false) {
		jobs = append(jobs, func() []string {
			name := bikPattern.ReplaceAllString(file, "")
			inFile := filepath.Join(src, file)
			outFile := filepath.Join(dir, name+".mp4")
			movie := kotor.ParseBink(readFile(inFile))
			if len(movie.Errors) > 0 {
				return line(name + ".mp4 FAIL: source does not parse")
			}
			if !exists(outFile) {
				return line(name + ".mp4 FAIL: output missing")
			}

			header := movie.Header
			var probe, compare kotor.ExecResult
			var group sync.WaitGroup
			group.Go(func() {
				probe = kotor.Exec("ffprobe", []string{
					"-v", "error", "-count_frames", "-of", "json", "-show_entries",
					"stream=codec_type,codec_name,width,height,nb_read_frames,sample_rate,channels,duration",
					outFile,
				}, nil)
			})
			group.Go(func() {
				compare = kotor.Exec("ffmpeg", []string{
					"-hide_banner", "-nostats", "-i", outFile, "-i", inFile,
					"-lavfi", "[0:v][1:v]psnr", "-f", "null", "-",
				}, nil)
			})
			group.Wait()

			var parsed struct {
				Streams []probeStream `json:"streams"`
			}
			if probe.Status == 0 && json.Unmarshal(probe.Stdout, &parsed) != nil {
				return line(name + ".mp4 FAIL: ffprobe output is not JSON")
			}
			var video, sound *probeStream
			for i := range parsed.Streams {
				stream := &parsed.Streams[i]
				if stream.CodecType == "video" && video == nil {
					video = stream
				}
				if stream.CodecType == "audio" && sound == nil {
					sound = stream
				}
			}

			var errs []string
			if video == nil || video.CodecName != "h264" {
				errs = append(errs, "no H.264 video stream")
			} else {
				if video.Width == nil || video.Height == nil || *video.Width != header.Width || *video.Height != header.Height {
					errs = append(errs, fmt.Sprintf("size %sx%s, Bink %dx%d",
						optional(video.Width), optional(video.Height), header.Width, header.Height))
				}
				if optionalNumber(video.NbReadFrames) != float64(header.Frames) {
					errs = append(errs, fmt.Sprintf("%s frames, Bink %d", optional(video.NbReadFrames), header.Frames))
				}
			}

			audioInfo := "no audio"
			if header.AudioTracks == 0 && sound != nil {
				errs = append(errs, "unexpected audio stream")
			}
			if header.AudioTracks > 0 {
				track := header.Tracks[0]
				channels := 1
				if track.Flags&kotor.BinkTrackStereo != 0 {
					channels = 2
				}
				durationS := float64(movie.DecodedBytes[0]) / float64(binkBytesPerSample*channels*track.SampleRate)
				if sound == nil || sound.CodecName != "aac" {
					errs = append(errs, "no AAC audio stream")
				} else {
					if optionalNumber(sound.SampleRate) != float64(track.SampleRate) {
						errs = append(errs, fmt.Sprintf("%s Hz, Bink %d Hz", optional(sound.SampleRate), track.SampleRate))
					}
					if sound.Channels == nil || *sound.Channels != channels {
						errs = append(errs, fmt.Sprintf("%s channels, Bink %d", optional(sound.Channels), channels))
					}
					if math.Abs(optionalNumber(sound.Duration)-durationS) > audioDurationToleranceS {
						errs = append(errs, fmt.Sprintf("audio %s s, Bink %s s", optional(sound.Duration), kotor.ToFixed(durationS, 3)))
					}
					audioInfo = fmt.Sprintf("%dHz %dch %ss", track.SampleRate, channels, kotor.ToFixed(durationS, 3))
				}
			}

			average, minimum := "?", "?"
			measured := "not measured"
			psnrMin := math.NaN()
			if psnr := psnrPattern.FindStringSubmatch(compare.Stderr); psnr != nil {
				average, minimum, measured = psnr[1], psnr[2], psnr[2]
				psnrMin = jsNumber(strings.Replace(psnr[2], "inf", "Infinity", 1))
			}
			if !(psnrMin >= minPsnr) {
				errs = append(errs, "minimum PSNR "+measured)
			}

			return line(fmt.Sprintf("%s.mp4 %dx%d %d frames %s PSNR avg %s min %s %s",
				name, header.Width, header.Height, header.Frames, audioInfo, average, minimum, kotor.Verdict(errs)))
		})
	}
	items.tally("ffprobe_movies", pool(jobs))
}

// GUI: assets/data/gui/*.json and assets/patch/*.json
func gui(items *summary) {
	if !kotor.Has("jq") {
		items.note("jq missing: gui UNVERIFIED")
		return
	}

	log("gui...")
	var jobs []job
	for _, dir := range []string{"data/gui", "patch"} {
		for _, file := range files(filepath.Join(kotor.Assets, dir), ".json", false) {
			jobs = append(jobs, func() []string {
				result := kotor.Exec("jq", []string{"empty", filepath.Join(kotor.Assets, dir, file)}, nil)
				if result.Status == 0 {
					return line(dir + "/" + file + " PASS")
				}
				return line(dir + "/" + file + " FAIL: jq: " + firstLine(result))
			})
		}
	}
	items.tally("jq_gui", pool(jobs))
}

// Movies queue their jobs first because each one takes longest, so short jobs fill the other slots.
var stages = []struct {
	name  string
	check func(*summary)
}{
	{"movies", movies},
	{"models", models},
	{"textures", textures},
	{"tga", tgaImages},
	{"music", func(items *summary) { streamAudio(items, "music", "streammusic") }},
	{"sounds", func(items *summary) { streamAudio(items, "sounds", "streamsounds") }},
	{"waves", func(items *summary) { streamAudio(items, "waves", "streamwaves") }},
	{"gui", gui},
}

func main() {
	wanted := os.Args[1:]

	results := map[string]*summary{}
	var group sync.WaitGroup
	for _, stage := range stages {
		if len(wanted) > 0 && !slices.Contains(wanted, stage.name) {
			continue
		}

		items := &summary{}
		results[stage.name] = items
		group.Go(func() { stage.check(items) })
	}
	group.Wait()

	if err := os.MkdirAll(kotor.Reports, 0o755); err != nil {
		kotor.Fatal(err.Error())
	}

	var output []string
	for _, stage := range kotor.ReportOrder {
		items, ok := results[stage]
		if !ok {
			continue
		}

		for _, entry := range *items {
			if entry.name == "" {
				output = append(output, entry.note)
				continue
			}

			report := []byte(strings.Join(entry.lines, "\n") + "\n")
			if err := os.WriteFile(filepath.Join(kotor.Reports, entry.name+".txt"), report, 0o644); err != nil {
				kotor.Fatal(err.Error())
			}

			pass := 0
			for _, text := range entry.lines {
				if strings.HasSuffix(text, "PASS") {
					pass++
				}
			}
			output = append(output, fmt.Sprintf("%s: %d pass, %d fail", entry.name, pass, len(entry.lines)-pass))
		}
	}
	fmt.Println(strings.Join(output, "\n"))
}
