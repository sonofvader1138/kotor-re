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
// lips: Go's encoding/csv reader must read each CSV file, and its keyframes, packed again as f32 and u8, must
// equal the source LIP keyframe bytes.
// modules: jq must parse each JSON file. No independent tool reads NCS, so each script listing is
// assembled again from its text and must equal the source script byte for byte. The assembler shares only
// the opcode, type and operand tables with the disassembler.
// rims: assimp must load each OBJ file. Every other resource is checked as addResourceChecks describes:
// WAV files with ffprobe and ffmpeg, 2DA CSV files with encoding/csv, SSF and LTR JSON files rebuilt from
// jq's output, other JSON files with jq, NSS files with iconv, and script listings by assembling them.
// bifs: the resources of the BIF archives in kotor.ResourceBifs, checked as addResourceChecks describes.
// tlk: the string table JSON, read with jq, must build again into dialog.tlk byte for byte.
//
// usage: go run ./cmd/verify [models] [textures] [tga] [music] [sounds] [waves] [movies] [gui] [lips] [modules] [rims] [bifs] [tlk]   (default: all)
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
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
			for text := range strings.SplitSeq(string(result.Stdout), "\n") {
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

// assimpObj loads one OBJ file with assimp and returns its report line. The label is the file name
// relative to dir.
func assimpObj(dir, file string) []string {
	objPath := filepath.Join(dir, file)
	faces := 0
	for text := range bytes.SplitSeq(kotor.ReadFile(objPath), []byte("\n")) {
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
}

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
		jobs = append(jobs, func() []string { return assimpObj(dir, file) })
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

// Lips: assets/lips/<module>/*.csv. Go's encoding/csv reader must read each file, and its keyframes, packed
// again as f32 and u8, must equal the keyframe bytes of the source LIP resource. This reader shares no
// code with the converter's writer, but it is not an external tool.
func lips(summary *kotor.Summary) {
	log("lips...")
	src := filepath.Join(kotor.Game, "lips")
	var report []string
	for _, archive := range kotor.ListFiles(src, ".mod", false) {
		module := strings.TrimSuffix(archive, filepath.Ext(archive))
		for _, entry := range kotor.ReadErf(filepath.Join(src, archive)) {
			label := filepath.Join("lips", module, entry.Resref+".csv")
			report = append(report, label+" "+kotor.Verdict(checkLipCsv(filepath.Join(kotor.Assets, label), entry.Data())))
		}
	}
	summary.Tally("csv_lips", report)
}

// checkLipCsv reads a converted LIP file and returns an error message when its keyframes differ from the
// source resource, or an empty string when they match.
func checkLipCsv(path string, source []byte) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}

	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return err.Error()
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != kotor.LipCsvHeader {
		return "bad header"
	}

	packed := make([]byte, 0, len(source))
	for _, row := range rows[1:] {
		time, timeErr := strconv.ParseFloat(row[0], 32)
		shape, shapeErr := strconv.ParseUint(row[1], 10, 8)
		if timeErr != nil || shapeErr != nil {
			return "bad value in " + strings.Join(row, ",")
		}

		packed = binary.LittleEndian.AppendUint32(packed, math.Float32bits(float32(time)))
		packed = append(packed, byte(shape))
	}

	if !bytes.Equal(packed, source[kotor.LipHeaderSize:]) {
		return "keyframes differ from the source"
	}
	return ""
}

// Modules: assets/modules/<archive>/*.json must parse with jq, and every <resref>.ncs.txt listing must
// assemble to the bytes of the script in the source archive.
func modules(summary *kotor.Summary) {
	log("modules...")
	src := filepath.Join(kotor.Game, "modules")
	var jsonJobs []job
	var report []string
	for _, archive := range kotor.ListFiles(src, ".rim", false) {
		name := strings.TrimSuffix(archive, filepath.Ext(archive))
		rim, problem := kotor.ReadRim(kotor.ReadFile(filepath.Join(src, archive)))
		if problem != "" {
			report = append(report, archive+" FAIL: "+problem)
			continue
		}

		for _, entry := range rim.Entries {
			if entry.Ext == "ncs" {
				label := filepath.Join("modules", name, entry.Resref+".ncs.txt")
				report = append(report, label+" "+kotor.Verdict(checkNcsListing(filepath.Join(kotor.Assets, label), entry.Data)))
				continue
			}

			label := filepath.Join("modules", name, entry.Resref+"."+entry.Ext+".json")
			jsonJobs = append(jsonJobs, func() []string {
				result := kotor.Exec("jq", []string{"empty", filepath.Join(kotor.Assets, label)}, nil)
				if result.Status == 0 {
					return line(label + " PASS")
				}
				return line(label + " FAIL: jq: " + kotor.FirstLine(result.Stderr))
			})
		}
	}
	summary.Tally("ncs_modules", report)

	if !kotor.Has("jq") {
		summary.Note("jq missing: module JSON UNVERIFIED")
		return
	}
	summary.Tally("jq_modules", pool(jsonJobs))
}

// checkTools names the tool that each kind of resource check runs, in report order. A check without a
// tool runs in Go and builds the source resource again from the converted file.
var checkTools = []struct{ kind, program string }{
	{"assimp", "assimp"}, {"ffprobe", "ffprobe"}, {"csv", ""}, {"ncs", ""}, {"jq", "jq"}, {"iconv", "iconv"},
	{"copy", ""},
}

// addResourceChecks adds the check of one converted resource to jobs, keyed by the kind of check. path is
// the converted file without the extensions that writeResource adds, and label is the same path relative
// to the folder of the report. Models are not added here, because their OBJ files are checked by listing.
//
//   - A WAV file must be read by ffprobe as 16-bit PCM.
//   - A 2DA CSV file, read with Go's encoding/csv reader, must build again into the source table.
//   - A script listing must assemble to the source script.
//   - The string numbers of an SSF JSON file, read with jq, must build again into the source sound set.
//   - A letter table JSON file, read with jq, must build again into the source letter table.
//   - A script source, converted back to Windows-1252 by iconv, must equal the source bytes.
//   - A TXI file must equal the source bytes.
//   - Every other JSON file must parse with jq.
func addResourceChecks(jobs map[string][]job, label, path, ext string, source []byte) {
	add := func(kind, file string, check func(path string) string) {
		jobs[kind] = append(jobs[kind], func() []string {
			return line(label + file + " " + kotor.Verdict(check(path+file)))
		})
	}

	switch ext {
	case "mdl", "mdx":

	case "wav":
		add("ffprobe", ".wav", func(path string) string {
			result := kotor.Exec("ffprobe", []string{"-v", "error", "-show_entries", "stream=codec_name",
				"-of", "csv=p=0", path}, nil)
			codec := strings.TrimSpace(string(result.Stdout))
			if result.Status != 0 || strings.TrimSpace(result.Stderr) != "" {
				return kotor.FirstLine(result.Stderr)
			}
			if codec != pcmCodec {
				return "codec " + codec
			}

			converted, err := os.ReadFile(path)
			if err != nil {
				return err.Error()
			}
			want, problem := ffmpegSamples(source)
			got, otherProblem := ffmpegSamples(converted)
			if problem != "" || otherProblem != "" {
				return problem + otherProblem
			}
			return compareSamples(want, got, isImaAdpcm(source))
		})

	case "2da":
		add("csv", ".2da.csv", func(path string) string { return checkTwoDaCsv(path, source) })

	case "ncs":
		add("ncs", ".ncs.txt", func(path string) string { return checkNcsListing(path, source) })

	case "ssf":
		add("jq", ".ssf.json", func(path string) string { return checkSsfJson(path, source) })

	case "ltr":
		add("jq", ".ltr.json", func(path string) string { return checkLtrJson(path, source) })

	case "nss":
		add("iconv", ".nss", func(path string) string {
			result := kotor.Exec("iconv", []string{"-f", "UTF-8", "-t", "WINDOWS-1252", path}, nil)
			if result.Status != 0 {
				return "iconv: " + kotor.FirstLine(result.Stderr)
			}
			if !bytes.Equal(result.Stdout, source) {
				return fmt.Sprintf("%d bytes differ from the %d source bytes", len(result.Stdout), len(source))
			}
			return ""
		})

	case "txi":
		add("copy", ".txi", func(path string) string {
			data, err := os.ReadFile(path)
			if err != nil {
				return err.Error()
			}
			if !bytes.Equal(data, source) {
				return "differs from the source"
			}
			return ""
		})

	default:
		add("jq", "."+ext+".json", func(path string) string {
			result := kotor.Exec("jq", []string{"empty", path}, nil)
			if result.Status != 0 {
				return "jq: " + kotor.FirstLine(result.Stderr)
			}
			return ""
		})
	}
}

// ffmpegSamples decodes a WAV file with ffmpeg into raw 16-bit little-endian samples.
func ffmpegSamples(wav []byte) ([]byte, string) {
	result := kotor.Exec("ffmpeg", []string{"-v", "error", "-i", "-", "-f", "s16le", "-acodec", pcmCodec, "-"}, wav)
	if result.Status != 0 || strings.TrimSpace(result.Stderr) != "" {
		return nil, "ffmpeg: " + kotor.FirstLine(result.Stderr)
	}
	return result.Stdout, ""
}

// WAV header offsets of the first chunk id and of the format tag, when the fmt chunk comes first, and the
// IMA ADPCM format tag.
const (
	wavFirstChunkOffset = 12
	wavFormatTagOffset  = 20
	wavFormatImaAdpcm   = 0x11
)

func isImaAdpcm(wav []byte) bool {
	return len(wav) > wavFormatTagOffset+2 &&
		string(wav[wavFirstChunkOffset:wavFirstChunkOffset+4]) == "fmt " &&
		binary.LittleEndian.Uint16(wav[wavFormatTagOffset:]) == wavFormatImaAdpcm
}

// adpcmToleranceSamples is the largest difference allowed between a sample decoded by kotor.DecodeAdpcm
// and the same sample decoded by ffmpeg. kotor.DecodeAdpcm adds shifted step sizes, as the IMA reference
// decoder does, and ffmpeg multiplies exactly, so the decoded values drift apart within a block by up to
// 86 in the game files. The first sample of every block is stored in the block header and agrees exactly.
const adpcmToleranceSamples = 128

// compareSamples compares the 16-bit samples of a converted WAV file with those of its source, both decoded
// by ffmpeg. PCM sources must match exactly. For an IMA ADPCM source, the converted file holds the sample
// count of the fact chunk, which can be fewer than ffmpeg decodes from the whole last block, so the
// converted samples must match the start of the source samples within adpcmToleranceSamples.
func compareSamples(want, got []byte, adpcm bool) string {
	if !adpcm {
		if !bytes.Equal(want, got) {
			return fmt.Sprintf("%d sample bytes differ from the %d of the source", len(got), len(want))
		}
		return ""
	}

	if len(got) > len(want) {
		return fmt.Sprintf("%d sample bytes, more than the %d of the source", len(got), len(want))
	}
	largest := 0
	for i := 0; i+1 < len(got); i += 2 {
		difference := int(int16(binary.LittleEndian.Uint16(got[i:]))) - int(int16(binary.LittleEndian.Uint16(want[i:])))
		largest = max(largest, difference, -difference)
	}
	if largest > adpcmToleranceSamples {
		return fmt.Sprintf("sample difference %d above %d", largest, adpcmToleranceSamples)
	}
	return ""
}

// runResourceChecks runs the jobs of each kind of check and tallies them in the report <kind>_<stage>. A
// kind whose tool is missing is reported as UNVERIFIED. Kinds without jobs write no report.
func runResourceChecks(summary *kotor.Summary, stage string, jobs map[string][]job) {
	for _, tool := range checkTools {
		if len(jobs[tool.kind]) == 0 {
			continue
		}

		report := tool.kind + "_" + stage
		if tool.program != "" && !kotor.Has(tool.program) {
			summary.Note(tool.program + " missing: " + report + " UNVERIFIED")
			continue
		}
		summary.Tally(report, pool(jobs[tool.kind]))
	}
}

// Rims: assets/rims/<archive>/ against rims/<archive>.rim. Each OBJ file must load with assimp, and every
// other resource is checked as addResourceChecks describes.
func rims(summary *kotor.Summary) {
	log("rims...")
	src := filepath.Join(kotor.Game, "rims")
	out := filepath.Join(kotor.Assets, "rims")
	jobs := map[string][]job{}
	var archiveReport []string
	for _, archive := range kotor.ListFiles(src, ".rim", false) {
		name := strings.TrimSuffix(archive, filepath.Ext(archive))
		rim, problem := kotor.ReadRim(kotor.ReadFile(filepath.Join(src, archive)))
		if problem != "" {
			archiveReport = append(archiveReport, archive+" FAIL: "+problem)
			continue
		}

		dir := filepath.Join(out, name)
		for _, file := range kotor.ListFiles(dir, ".obj", false) {
			jobs["assimp"] = append(jobs["assimp"], func() []string { return assimpObj(out, filepath.Join(name, file)) })
		}

		for _, entry := range rim.Entries {
			label := filepath.Join(name, entry.Resref)
			addResourceChecks(jobs, label, filepath.Join(out, label), entry.Ext, entry.Data)
		}
	}

	if len(archiveReport) > 0 {
		summary.Tally("archives_rims", archiveReport)
	}
	runResourceChecks(summary, "rims", jobs)
}

// Bifs: assets/data/<archive>/ against data/<archive>.bif for each archive in kotor.ResourceBifs. Each resource is
// checked as addResourceChecks describes.
func bifs(summary *kotor.Summary) {
	log("bifs...")
	key := kotor.ReadKey()
	jobs := map[string][]job{}
	for _, name := range kotor.ResourceBifs {
		for _, resource := range kotor.BifResources("data/"+name+".bif", key) {
			label := filepath.Join(name, resource.Resref)
			addResourceChecks(jobs, label, filepath.Join(kotor.Assets, "data", label), resource.Ext, resource.Data)
		}
	}
	runResourceChecks(summary, "bifs", jobs)
}

// checkLtrJson reads the values of a converted letter table with jq, in the order letters, single, each
// double set and each triple set, and builds the letter table again: the signature, the letter count and
// one f32 per value. It returns an error message when the result differs from the source, or an empty
// string when it matches.
func checkLtrJson(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", `.letters, (.single, .double[], .triple[] | .start[], .middle[], .end[])`, path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}

	fields := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
	built := append([]byte("LTR V1.0"), byte(len(fields[0])))
	for _, text := range fields[1:] {
		value, err := strconv.ParseFloat(text, 32)
		if err != nil {
			return "bad value " + text
		}
		built = binary.LittleEndian.AppendUint32(built, math.Float32bits(float32(value)))
	}

	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
}

// pcmCodec is ffprobe's name for 16-bit little-endian PCM.
const pcmCodec = "pcm_s16le"

// Binary 2DA layout, rebuilt by checkTwoDaCsv independently of the parser in kotor.
const (
	twoDaSignature    = "2DA V2.b\n"
	twoDaSeparatorTab = '\t'
	twoDaSeparatorNul = 0
)

// checkTwoDaCsv reads a converted 2DA table and builds the binary table again: the column labels, the row
// count, the row labels, one u16 offset per cell, the u16 size of the string data, and the string data,
// where each distinct string is stored once in the order of its first cell. The separator after each label
// is the one the source uses after its first column label. It returns an error message when the result
// differs from the source, or an empty string when it matches.
func checkTwoDaCsv(path string, source []byte) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}

	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return err.Error()
	}
	if len(rows) == 0 || len(rows[0]) < 2 || rows[0][0] != "" {
		return "bad header"
	}

	columns := rows[0][1:]
	separatorAt := len(twoDaSignature) + len(columns[0])
	if separatorAt >= len(source) {
		return "source too short"
	}
	separator := source[separatorAt]
	if separator != twoDaSeparatorTab && separator != twoDaSeparatorNul {
		return fmt.Sprintf("source separator %d", separator)
	}

	latin1 := func(text string) ([]byte, bool) {
		encoded := []byte{}
		for _, char := range text {
			if char > 0xff {
				return nil, false
			}
			encoded = append(encoded, byte(char))
		}
		return encoded, true
	}

	built := []byte(twoDaSignature)
	for _, label := range columns {
		encoded, ok := latin1(label)
		if !ok {
			return "column label outside Latin-1: " + label
		}
		built = append(append(built, encoded...), separator)
	}
	built = append(built, twoDaSeparatorNul)
	built = binary.LittleEndian.AppendUint32(built, uint32(len(rows)-1))

	var cells []string
	for _, row := range rows[1:] {
		encoded, ok := latin1(row[0])
		if !ok {
			return "row label outside Latin-1: " + row[0]
		}
		built = append(append(built, encoded...), separator)
		cells = append(cells, row[1:]...)
	}

	offsets := map[string]int{}
	var stringData []byte
	for _, cell := range cells {
		if _, seen := offsets[cell]; seen {
			continue
		}
		encoded, ok := latin1(cell)
		if !ok {
			return "cell outside Latin-1: " + cell
		}

		offsets[cell] = len(stringData)
		stringData = append(append(stringData, encoded...), twoDaSeparatorNul)
	}

	for _, cell := range cells {
		built = binary.LittleEndian.AppendUint16(built, uint16(offsets[cell]))
	}
	built = binary.LittleEndian.AppendUint16(built, uint16(len(stringData)))
	built = append(built, stringData...)

	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
}

// checkSsfJson reads the string numbers of a converted sound set with jq and builds the sound set again: the
// signature, the slot table offset 12, and one u32 per slot, 0xFFFFFFFF for null. It returns an error
// message when the result differs from the source, or an empty string when it matches.
func checkSsfJson(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", ".slots[].strref", path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}

	built := []byte("SSF V1.1")
	built = binary.LittleEndian.AppendUint32(built, kotor.SsfHeaderSize)
	for _, text := range strings.Fields(string(result.Stdout)) {
		if text == "null" {
			built = binary.LittleEndian.AppendUint32(built, kotor.SsfEmpty)
			continue
		}

		number, err := strconv.ParseUint(text, 10, 32)
		if err != nil {
			return "bad string number " + text
		}
		built = binary.LittleEndian.AppendUint32(built, uint32(number))
	}

	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
}

// ncsMnemonics maps each listing mnemonic to its opcode and type.
var ncsMnemonics = func() map[string][2]byte {
	mnemonics := map[string][2]byte{}
	for opcode := range kotor.NcsOpcodes {
		for valueType := range 256 {
			if _, valid := kotor.NcsOperands(opcode, byte(valueType)); !valid {
				continue
			}

			mnemonic := kotor.NcsMnemonic(opcode, byte(valueType))
			if _, taken := mnemonics[mnemonic]; taken {
				kotor.Fatal("mnemonic " + mnemonic + " is ambiguous")
			}
			mnemonics[mnemonic] = [2]byte{opcode, byte(valueType)}
		}
	}
	return mnemonics
}()

// sizeOpcode is the instruction that carries the program size and has no type byte.
const sizeOpcode = 0x42

// checkNcsListing assembles a script listing and returns an error message when the result differs from the
// source script, or an empty string when it matches. Comment lines start with ";", and an instruction's
// comment starts at the first ";" after its operands.
func checkNcsListing(path string, source []byte) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}

	assembled := []byte("NCS V1.0")
	for _, text := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(text, ";") {
			continue
		}

		fields := strings.Fields(text)
		if len(fields) < 2 {
			return "short line: " + text
		}
		offset, err := strconv.ParseUint(fields[0], 16, 32)
		code, known := ncsMnemonics[fields[1]]
		if err != nil || int(offset) != len(assembled) || !known {
			return "bad line: " + text
		}

		assembled = append(assembled, code[0])
		if code[0] != sizeOpcode {
			assembled = append(assembled, code[1])
		}

		rest := strings.TrimSpace(text[strings.Index(text, fields[1])+len(fields[1]):])
		layout, _ := kotor.NcsOperands(code[0], code[1])
		for _, kind := range layout {
			if kind == 's' {
				quoted, err := strconv.QuotedPrefix(rest)
				value, unquoteErr := strconv.Unquote(quoted)
				if err != nil || unquoteErr != nil {
					return "bad string: " + text
				}

				assembled = binary.BigEndian.AppendUint16(assembled, uint16(len([]rune(value))))
				for _, char := range value {
					assembled = append(assembled, byte(char))
				}
				rest = strings.TrimSpace(rest[len(quoted):])
				continue
			}

			token, remainder, _ := strings.Cut(rest, " ")
			rest = strings.TrimSpace(remainder)
			var bits uint64
			if kind == 'f' {
				value, err := strconv.ParseFloat(token, 32)
				if err != nil {
					return "bad float: " + text
				}
				bits = uint64(math.Float32bits(float32(value)))
			} else {
				value, err := strconv.ParseInt(token, 10, 64)
				if err != nil {
					return "bad operand: " + text
				}
				bits = uint64(value)
			}

			switch kotor.NcsOperandSizes[kind] {
			case 4:
				assembled = binary.BigEndian.AppendUint32(assembled, uint32(bits))
			case 2:
				assembled = binary.BigEndian.AppendUint16(assembled, uint16(bits))
			case 1:
				assembled = append(assembled, byte(bits))
			}
		}
		if rest != "" && !strings.HasPrefix(rest, ";") {
			return "extra operands: " + text
		}
	}

	if !bytes.Equal(assembled, source) {
		return fmt.Sprintf("assembled %d bytes differ from the %d source bytes", len(assembled), len(source))
	}
	return ""
}

// tlkLine is the jq program that prints the language and then one line per entry: the string number, the
// flags, and the sound and text in base64, so that line feeds in the text cannot split a line.
const tlkLine = `.language, (.entries[] | "\(.strref) \(.flags) \(.sound | @base64) \(.text | @base64)")`

// Tlk: assets/dialog.tlk.json against dialog.tlk.
func tlk(summary *kotor.Summary) {
	if !kotor.Has("jq") {
		summary.Note("jq missing: tlk UNVERIFIED")
		return
	}

	log("tlk...")
	source := kotor.ReadFile(filepath.Join(kotor.Game, "dialog.tlk"))
	problem := checkTlkJson(filepath.Join(kotor.Assets, "dialog.tlk.json"), source)
	summary.Tally("jq_tlk", line("dialog.tlk.json "+kotor.Verdict(problem)))
}

// latin1Bytes encodes text as Latin-1 and reports whether every character fits in one byte.
func latin1Bytes(text string) ([]byte, bool) {
	out := []byte{}
	for _, character := range text {
		if character > 0xff {
			return nil, false
		}
		out = append(out, byte(character))
	}
	return out, true
}

// checkTlkJson builds a TLK file from jq's reading of the JSON form and returns an error message when the
// result differs from the source, or an empty string when it matches. Texts are laid out in entry order,
// an entry without text has offset 0, and the three unknown fields are 0.
func checkTlkJson(path string, source []byte) string {
	result := kotor.Exec("jq", []string{"-r", tlkLine, path}, nil)
	if result.Status != 0 {
		return "jq: " + kotor.FirstLine(result.Stderr)
	}

	lines := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\n")
	language, err := strconv.ParseUint(lines[0], 10, 32)
	if err != nil {
		return "bad language " + lines[0]
	}

	count := len(lines) - 1
	table := []byte(kotor.TlkSignature)
	table = binary.LittleEndian.AppendUint32(table, uint32(language))
	table = binary.LittleEndian.AppendUint32(table, uint32(count))
	table = binary.LittleEndian.AppendUint32(table, uint32(kotor.TlkHeaderSize+kotor.TlkEntrySize*count))

	var texts []byte
	for i, text := range lines[1:] {
		fields := strings.Split(text, " ")
		if len(fields) != 4 || fields[0] != strconv.Itoa(i) {
			return fmt.Sprintf("entry %d is malformed: %s", i, text)
		}

		flags, flagsErr := strconv.ParseUint(fields[1], 10, 32)
		soundText, soundErr := base64.StdEncoding.DecodeString(fields[2])
		entryText, textErr := base64.StdEncoding.DecodeString(fields[3])
		sound, soundFits := latin1Bytes(string(soundText))
		body, bodyFits := latin1Bytes(string(entryText))
		if flagsErr != nil || soundErr != nil || textErr != nil || !soundFits || !bodyFits || len(sound) > kotor.ResrefLength {
			return fmt.Sprintf("entry %d has a bad value", i)
		}

		offset := len(texts)
		if len(body) == 0 {
			offset = 0
		}

		table = binary.LittleEndian.AppendUint32(table, uint32(flags))
		table = append(table, sound...)
		table = append(table, make([]byte, kotor.ResrefLength-len(sound))...)
		table = binary.LittleEndian.AppendUint32(table, 0)
		table = binary.LittleEndian.AppendUint32(table, 0)
		table = binary.LittleEndian.AppendUint32(table, uint32(offset))
		table = binary.LittleEndian.AppendUint32(table, uint32(len(body)))
		table = binary.LittleEndian.AppendUint32(table, 0)
		texts = append(texts, body...)
	}

	built := append(table, texts...)
	if !bytes.Equal(built, source) {
		return fmt.Sprintf("built %d bytes differ from the %d source bytes", len(built), len(source))
	}
	return ""
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
	{Name: "lips", Run: lips},
	{Name: "modules", Run: modules},
	{Name: "rims", Run: rims},
	{Name: "bifs", Run: bifs},
	{Name: "tlk", Run: tlk},
}

func main() {
	kotor.RunStages(stages, os.Args[1:])
}
