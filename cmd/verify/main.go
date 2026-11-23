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
	"fmt"
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
		return decodedImage{err: kotor.FirstLine(result.Stderr)}
	}

	lines := strings.Split(strings.TrimSpace(result.Stderr), "\n")
	return decodedImage{size: lines[len(lines)-1], rgba: result.Stdout}
}

// pngCheck returns a report line that ends in PASS when the PNG decodes to the expected size and pixels.
func pngCheck(name, file, width, height string, rgba []byte) []string {
	if !exists(file) {
		return line(name + " FAIL: output missing")
	}

	decoded := magickDecode("png", kotor.ReadFile(file))
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
	names := kotor.ListFiles(dir, ".png", false)
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
func imageChecks(summary *kotor.Summary, name, dir string, jobs []job) {
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
		summary.Tally("imagemagick_"+name, pixelLines)
	} else {
		summary.Note("ImageMagick missing: " + name + " UNVERIFIED")
	}

	if pngcheck {
		summary.Tally("pngcheck_"+name, structureLines)
	} else {
		summary.Note("pngcheck missing: " + name + " PNG structure unchecked")
	}
}

var (
	meshesPattern = regexp.MustCompile(`(?m)^Meshes:\s+(\d+)`)
	facesPattern  = regexp.MustCompile(`(?m)^Faces:\s+(\d+)`)
)

// Models: assets/data/models/*.obj
func models(summary *kotor.Summary) {
	if !kotor.Has("assimp") {
		summary.Note("assimp missing: models UNVERIFIED")
		return
	}

	log("models...")
	dir := filepath.Join(kotor.Assets, "data", "models")
	var jobs []job
	for _, file := range kotor.ListFiles(dir, ".obj", false) {
		jobs = append(jobs, func() []string {
			objPath := filepath.Join(dir, file)
			faces := 0
			for _, text := range bytes.Split(kotor.ReadFile(objPath), []byte("\n")) {
				if bytes.HasPrefix(text, []byte("f ")) {
					faces++
				}
			}

			result := kotor.Exec("assimp", []string{"info", objPath, "-r"}, nil)
			meshes := meshesPattern.FindSubmatch(result.Stdout)
			loaded := facesPattern.FindSubmatch(result.Stdout)
			if result.Status != 0 || meshes == nil || loaded == nil {
				return line(file + " FAIL: assimp: " + kotor.FirstLine(result.Stderr))
			}

			var errs []string
			if count, _ := strconv.Atoi(string(meshes[1])); count == 0 {
				errs = append(errs, "no meshes")
			}
			if count, _ := strconv.Atoi(string(loaded[1])); count != faces {
				errs = append(errs, fmt.Sprintf("assimp loaded %s faces, file has %d", loaded[1], faces))
			}

			return line(fmt.Sprintf("%s meshes=%s faces=%s %s", file, meshes[1], loaded[1], kotor.Verdict(errs...)))
		})
	}
	summary.Tally("assimp_models", pool(jobs))
}

// Textures: assets/TexturePacks/<pack>/*.png against the TPC textures in TexturePacks/<pack>.erf.
// Each job decodes its TPC texture when it starts, so only one texture per job slot is held in memory.
func textures(summary *kotor.Summary) {
	for _, pack := range kotor.TexturePacks {
		log("textures " + pack + "...")
		dir := filepath.Join(kotor.Assets, "TexturePacks", pack)
		entries := kotor.ReadErf(filepath.Join(kotor.Game, "TexturePacks", pack+".erf"))

		var jobs []job
		for _, entry := range entries {
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
		imageChecks(summary, pack, dir, jobs)
	}
}

// TGA images: assets/data/<group>/*.png and *.txi against the BIF archives they came from
func tgaImages(summary *kotor.Summary) {
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
						if !bytes.Equal(kotor.ReadFile(file), resource.Data) {
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

		summary.Tally("txi_"+group.Name, pool(txiJobs))
		imageChecks(summary, group.Name, dir, jobs)
	}
}

// Stream audio: assets/streammusic, assets/streamsounds and assets/streamwaves
func streamAudio(summary *kotor.Summary, stage, folder string) {
	if !ffprobe {
		summary.Note("ffprobe missing: " + stage + " UNVERIFIED")
		return
	}

	log(stage + "...")
	dir := filepath.Join(kotor.Assets, folder)
	audioFiles := append(kotor.ListFiles(dir, ".mp3", true), kotor.ListFiles(dir, ".wav", true)...)
	slices.Sort(audioFiles)

	jobs := make([]job, len(audioFiles))
	for i, file := range audioFiles {
		jobs[i] = func() []string {
			result := kotor.Exec("ffprobe", []string{"-v", "error", filepath.Join(dir, file)}, nil)
			if result.Status == 0 && strings.TrimSpace(result.Stderr) == "" {
				return line(file + " PASS")
			}
			return line(file + " FAIL: " + kotor.FirstLine(result.Stderr))
		}
	}
	summary.Tally("ffprobe_"+stage, pool(jobs))
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

// optional formats a missing ffprobe field as "?".
func optional[T any](value *T) string {
	if value == nil {
		return "?"
	}
	return fmt.Sprint(*value)
}

func optionalNumber(value *string) float64 {
	if value == nil {
		return math.NaN()
	}
	number, err := strconv.ParseFloat(*value, 64)
	if err != nil {
		return math.NaN()
	}
	return number
}

var (
	psnrPattern = regexp.MustCompile(`PSNR [^\n\r]*average:(\S+) min:(\S+)`)
)

func movies(summary *kotor.Summary) {
	if !ffprobe || !kotor.Has("ffmpeg") {
		summary.Note("ffmpeg or ffprobe missing: movies UNVERIFIED")
		return
	}

	log("movies...")
	src := filepath.Join(kotor.Game, "movies")
	dir := filepath.Join(kotor.Assets, "movies")
	var jobs []job
	for _, file := range kotor.ListFiles(src, ".bik", false) {
		jobs = append(jobs, func() []string {
			name := strings.TrimSuffix(file, filepath.Ext(file))
			inFile := filepath.Join(src, file)
			outFile := filepath.Join(dir, name+".mp4")
			movie := kotor.ParseBink(kotor.ReadFile(inFile))
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
						errs = append(errs, fmt.Sprintf("audio %s s, Bink %.3f s", optional(sound.Duration), durationS))
					}
					audioInfo = fmt.Sprintf("%dHz %dch %.3fs", track.SampleRate, channels, durationS)
				}
			}

			average, minimum := "?", "?"
			measured := "not measured"
			psnrMin := math.NaN()
			if psnr := psnrPattern.FindStringSubmatch(compare.Stderr); psnr != nil {
				average, minimum, measured = psnr[1], psnr[2], psnr[2]
				if value, err := strconv.ParseFloat(psnr[2], 64); err == nil {
					psnrMin = value
				}
			}
			if !(psnrMin >= minPsnr) {
				errs = append(errs, "minimum PSNR "+measured)
			}

			return line(fmt.Sprintf("%s.mp4 %dx%d %d frames %s PSNR avg %s min %s %s",
				name, header.Width, header.Height, header.Frames, audioInfo, average, minimum, kotor.Verdict(errs...)))
		})
	}
	summary.Tally("ffprobe_movies", pool(jobs))
}

// GUI: assets/data/gui/*.json and assets/patch/*.json
func gui(summary *kotor.Summary) {
	if !kotor.Has("jq") {
		summary.Note("jq missing: gui UNVERIFIED")
		return
	}

	log("gui...")
	var jobs []job
	for _, dir := range []string{"data/gui", "patch"} {
		for _, file := range kotor.ListFiles(filepath.Join(kotor.Assets, dir), ".json", false) {
			jobs = append(jobs, func() []string {
				result := kotor.Exec("jq", []string{"empty", filepath.Join(kotor.Assets, dir, file)}, nil)
				if result.Status == 0 {
					return line(dir + "/" + file + " PASS")
				}
				return line(dir + "/" + file + " FAIL: jq: " + kotor.FirstLine(result.Stderr))
			})
		}
	}
	summary.Tally("jq_gui", pool(jobs))
}

// Movies queue their jobs first because each one takes longest, so short jobs fill the other slots.
var stages = []kotor.Stage{
	{Name: "movies", Run: movies},
	{Name: "models", Run: models},
	{Name: "textures", Run: textures},
	{Name: "tga", Run: tgaImages},
	{Name: "music", Run: func(summary *kotor.Summary) { streamAudio(summary, "music", "streammusic") }},
	{Name: "sounds", Run: func(summary *kotor.Summary) { streamAudio(summary, "sounds", "streamsounds") }},
	{Name: "waves", Run: func(summary *kotor.Summary) { streamAudio(summary, "waves", "streamwaves") }},
	{Name: "gui", Run: gui},
}

func main() {
	kotor.RunStages(stages, os.Args[1:])
}
