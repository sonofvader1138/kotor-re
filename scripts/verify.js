// usage: node scripts/verify.js [models] [textures] [tga] [music] [sounds] [movies] [gui]   (default: all)
// Checks the converted assets in ~/re/assets with independent tools and writes one report per check to
// ~/re/temp/reports. Run scripts/convert.js first. A stage whose tool is missing is reported as UNVERIFIED.
// Within a stage, the checks run in parallel, one external program per CPU at a time.
// models: assimp must load each OBJ with at least one mesh and the same face count as the file.
// textures: ImageMagick's decoding of each PNG must equal the decoded TPC texture, and pngcheck must accept it.
// tga: ImageMagick's decoding of each PNG must equal its decoding of the source TGA, and pngcheck must
// accept the PNG. Each TXI must equal the source bytes.
// music, sounds: ffprobe must read each file without error.
// movies: ffprobe must report the Bink file's size, frame count, sample rate, channel count and audio
// duration (within 0.1 s; ffmpeg's Bink audio decoder emits about 0.03 s more than the packets declare),
// and every output frame must have a PSNR of at least 40 dB against the decoded Bink frame.
// gui: jq must parse each JSON file.
const fs = require("fs")
const path = require("path")
const childProcess = require("child_process")
const os = require("os")
const kotor = require("./kotor")
const tpc = require("./tpc2png")
const bink = require("./bink")

const ASSETS = path.join(kotor.ROOT, "assets"),
	REPORTS = path.join(kotor.ROOT, "temp", "reports")
fs.mkdirSync(REPORTS, { recursive: true })
const want = new Set(process.argv.slice(2))
const run = (stage) => want.size === 0 || want.has(stage)
const summary = []
const tally = (name, lines) => {
	fs.writeFileSync(path.join(REPORTS, `${name}.txt`), lines.join("\n") + "\n")
	const pass = lines.filter((line) => line.endsWith("PASS")).length
	summary.push(`${name}: ${pass} pass, ${lines.length - pass} fail`)
}
// childProcess.spawnSync reports a missing program in its error field instead of throwing.
const has = (program, flag = "-version") =>
	!childProcess.spawnSync(program, [flag], { stdio: "ignore" }).error
// Runs a program without blocking and resolves to { status, stdout (Buffer), stderr (string), error }.
// A missing program resolves with error set, like childProcess.spawnSync.
const exec = (program, args, input) =>
	new Promise((resolve) => {
		const child = childProcess.spawn(program, args)
		const stdout = [],
			stderr = []
		child.stdout.on("data", (chunk) => stdout.push(chunk))
		child.stderr.on("data", (chunk) => stderr.push(chunk))
		// A program that exits before reading all of its input closes the pipe. The exit status reports that.
		child.stdin.on("error", () => {})
		child.stdin.end(input)
		const finish = (status, error) =>
			resolve({
				status,
				error,
				stdout: Buffer.concat(stdout),
				stderr: Buffer.concat(stderr).toString(),
			})
		child.on("error", (error) => finish(null, error))
		child.on("close", (status) => finish(status))
	})
// Runs the jobs, each a function returning a promise, with one job per CPU at a time.
// Resolves to their results in job order.
const WORKERS = os.availableParallelism()
const pool = async (jobs) => {
	const results = new Array(jobs.length)
	let next = 0
	const worker = async () => {
		while (next < jobs.length) {
			const i = next++
			results[i] = await jobs[i]()
		}
	}
	await Promise.all(Array.from({ length: WORKERS }, worker))
	return results
}
const firstLine = (result) =>
	String(result.stderr || result.error || "")
		.trim()
		.split("\n")[0]
const files = (dir, ext) =>
	fs.existsSync(dir)
		? fs
				.readdirSync(dir)
				.filter((file) => file.endsWith(ext))
				.sort()
		: []

// Decodes an image with ImageMagick into 8-bit RGBA, top row first. The size goes to stderr.
const magickDecode = async (format, data) => {
	const result = await exec(
		"convert",
		[
			`${format}:-`,
			"-auto-orient",
			"-format",
			"%w %h",
			"-write",
			"info:fd:2",
			"-depth",
			"8",
			"rgba:-",
		],
		data,
	)
	if (result.status !== 0) return { err: firstLine(result) }
	return {
		size: result.stderr.trim().split("\n").pop(),
		rgba: result.stdout,
	}
}
// Returns a report line that ends in PASS when the PNG decodes to the expected size and pixels.
const pngCheck = async (name, file, width, height, rgba) => {
	if (!fs.existsSync(file)) return `${name} FAIL: output missing`
	const decoded = await magickDecode("png", fs.readFileSync(file))
	if (decoded.err) return `${name} FAIL: ${decoded.err}`
	if (decoded.size !== `${width} ${height}`)
		return `${name} FAIL: size ${decoded.size}, expected ${width} ${height}`
	if (Buffer.compare(decoded.rgba, rgba) !== 0)
		return `${name} ${decoded.size} FAIL: pixels differ`
	return `${name} ${decoded.size} PASS`
}
// Runs pngcheck on every PNG in dir, 200 files per run. Resolves to one report line per file.
const pngcheckDir = async (dir) => {
	const names = files(dir, ".png"),
		jobs = []
	for (let i = 0; i < names.length; i += 200) {
		const chunk = names.slice(i, i + 200)
		jobs.push(async () => {
			const result = await exec(
				"pngcheck",
				chunk.map((file) => path.join(dir, file)),
			)
			const ok = new Set()
			for (const line of result.stdout.toString().split("\n")) {
				const match = /^OK: (.*?) \(/.exec(line)
				if (match) ok.add(path.basename(match[1]))
			}
			return chunk.map((file) =>
				ok.has(file) ? `${file} PASS` : `${file} FAIL: pngcheck`,
			)
		})
	}
	return (await pool(jobs)).flat()
}
const magick = has("convert"),
	pngcheck = has("pngcheck", "-h")
const imageChecks = async (name, dir, jobs) => {
	if (magick) tally(`imagemagick_${name}`, await pool(jobs))
	else summary.push(`ImageMagick missing: ${name} UNVERIFIED`)
	if (pngcheck) tally(`pngcheck_${name}`, await pngcheckDir(dir))
	else summary.push(`pngcheck missing: ${name} PNG structure unchecked`)
}

// Models: assets/data/models/*.obj
const models = async () => {
	if (!has("assimp", "version")) {
		summary.push("assimp missing: models UNVERIFIED")
		return
	}
	console.log("models...")
	const dir = path.join(ASSETS, "data", "models")
	const jobs = files(dir, ".obj").map((file) => async () => {
		const objPath = path.join(dir, file)
		const faces = fs
			.readFileSync(objPath, "latin1")
			.split("\n")
			.filter((line) => line.startsWith("f ")).length
		const result = await exec("assimp", ["info", objPath])
		const stdout = result.stdout.toString()
		const meshes = /^Meshes:\s+(\d+)/m.exec(stdout),
			loaded = /^Faces:\s+(\d+)/m.exec(stdout)
		if (result.status !== 0 || !meshes || !loaded)
			return `${file} FAIL: assimp: ${firstLine(result)}`
		const errs = []
		if (Number(meshes[1]) === 0) errs.push("no meshes")
		if (Number(loaded[1]) !== faces)
			errs.push(`assimp loaded ${loaded[1]} faces, file has ${faces}`)
		return (
			`${file} meshes=${meshes[1]} faces=${loaded[1]} ` +
			(errs.length ? "FAIL: " + errs.join("; ") : "PASS")
		)
	})
	tally("assimp_models", await pool(jobs))
}

// Textures: assets/TexturePacks/<pack>/*.png against the TPC textures in TexturePacks/<pack>.erf.
// Each job decodes its TPC texture when it starts, so only one texture per worker is held in memory.
const textures = async () => {
	for (const pack of ["swpc_tex_tpa", "swpc_tex_gui"]) {
		console.log(`textures ${pack}...`)
		const dir = path.join(ASSETS, "TexturePacks", pack)
		const erf = kotor.readErf(
			path.join(kotor.GAME, "TexturePacks", `${pack}.erf`),
		)
		const jobs = erf.entries
			.filter((entry) => entry.ext === "tpc")
			.map((entry) => async () => {
				const data = entry.data(),
					image = tpc.decode(data, tpc.parse(data)),
					name = `${entry.resref}.png`
				if (!image) return `${name} FAIL: source texture does not decode`
				return pngCheck(
					name,
					path.join(dir, name),
					image.width,
					image.height,
					image.rgba,
				)
			})
		await imageChecks(pack, dir, jobs)
	}
}

// TGA images: assets/data/<group>/*.png and *.txi against the BIF archives they came from
const TGA_GROUPS = [
	["lightmaps", /^data\/lightmaps\d*\.bif$/],
	["textures", /^data\/textures\.bif$/],
	["legacy", /^data\/legacy\.bif$/],
]
const tgaImages = async () => {
	const key = kotor.readKey()
	for (const [group, pattern] of TGA_GROUPS) {
		console.log(`tga ${group}...`)
		const dir = path.join(ASSETS, "data", group)
		const txiLines = [],
			jobs = [],
			seen = new Set()
		for (const bif of key.bifs.filter((name) => pattern.test(name))) {
			for (const resource of kotor.bifResources(bif, key)) {
				if (resource.ext !== "tga" && resource.ext !== "txi") continue
				const name = `${resource.resref}.${resource.ext === "tga" ? "png" : "txi"}`,
					file = path.join(dir, name)
				// convert.js keeps the first archive's copy of a name and reports the others.
				if (seen.has(name)) continue
				seen.add(name)
				if (resource.ext === "txi") {
					txiLines.push(
						!fs.existsSync(file)
							? `${name} FAIL: output missing`
							: Buffer.compare(fs.readFileSync(file), resource.data) !== 0
								? `${name} FAIL: differs from source`
								: `${name} PASS`,
					)
					continue
				}
				jobs.push(async () => {
					const source = await magickDecode("tga", resource.data)
					if (source.err) return `${name} FAIL: source TGA: ${source.err}`
					const [width, height] = source.size.split(" ")
					return pngCheck(name, file, width, height, source.rgba)
				})
			}
		}
		tally(`txi_${group}`, txiLines)
		await imageChecks(group, dir, jobs)
	}
}

// Stream audio: assets/streammusic and assets/streamsounds
const ffprobe = has("ffprobe")
const streamAudio = async (stage, folder) => {
	if (!ffprobe) {
		summary.push(`ffprobe missing: ${stage} UNVERIFIED`)
		return
	}
	console.log(`${stage}...`)
	const dir = path.join(ASSETS, folder)
	const jobs = files(dir, "").map((file) => async () => {
		const result = await exec("ffprobe", ["-v", "error", path.join(dir, file)])
		return result.status === 0 && !result.stderr.trim()
			? `${file} PASS`
			: `${file} FAIL: ${firstLine(result)}`
	})
	tally(`ffprobe_${stage}`, await pool(jobs))
}

// Movies: assets/movies/*.mp4 against movies/*.bik. The probe and the PSNR comparison of one movie run
// at the same time.
const MIN_PSNR = 40
const movies = async () => {
	if (!(ffprobe && has("ffmpeg"))) {
		summary.push("ffmpeg or ffprobe missing: movies UNVERIFIED")
		return
	}
	console.log("movies...")
	const src = path.join(kotor.GAME, "movies"),
		dir = path.join(ASSETS, "movies")
	const jobs = files(src, ".bik").map((file) => async () => {
		const name = file.replace(/\.bik$/i, ""),
			inFile = path.join(src, file),
			outFile = path.join(dir, `${name}.mp4`)
		const movie = bink.parse(fs.readFileSync(inFile))
		if (movie.errors.length) return `${name}.mp4 FAIL: source does not parse`
		if (!fs.existsSync(outFile)) return `${name}.mp4 FAIL: output missing`
		const header = movie.header
		const [probe, compare] = await Promise.all([
			exec("ffprobe", [
				"-v",
				"error",
				"-count_frames",
				"-of",
				"json",
				"-show_entries",
				"stream=codec_type,codec_name,width,height,nb_read_frames,sample_rate,channels,duration",
				outFile,
			]),
			exec("ffmpeg", [
				"-hide_banner",
				"-nostats",
				"-i",
				outFile,
				"-i",
				inFile,
				"-lavfi",
				"[0:v][1:v]psnr",
				"-f",
				"null",
				"-",
			]),
		])
		const streams =
			probe.status === 0 ? JSON.parse(probe.stdout.toString()).streams : []
		const video = streams.find((stream) => stream.codec_type === "video"),
			sound = streams.find((stream) => stream.codec_type === "audio")
		const errs = []
		if (!video || video.codec_name !== "h264")
			errs.push("no H.264 video stream")
		else {
			if (video.width !== header.width || video.height !== header.height)
				errs.push(
					`size ${video.width}x${video.height}, Bink ${header.width}x${header.height}`,
				)
			if (Number(video.nb_read_frames) !== header.frames)
				errs.push(`${video.nb_read_frames} frames, Bink ${header.frames}`)
		}
		let audioInfo = "no audio"
		if (header.audioTracks === 0 && sound) errs.push("unexpected audio stream")
		if (header.audioTracks > 0) {
			const track = header.tracks[0],
				channels = track.flags & 0x2000 ? 2 : 1,
				durationS = movie.decodedBytes[0] / (2 * channels * track.sampleRate)
			if (!sound || sound.codec_name !== "aac") errs.push("no AAC audio stream")
			else {
				if (Number(sound.sample_rate) !== track.sampleRate)
					errs.push(`${sound.sample_rate} Hz, Bink ${track.sampleRate} Hz`)
				if (sound.channels !== channels)
					errs.push(`${sound.channels} channels, Bink ${channels}`)
				if (Math.abs(Number(sound.duration) - durationS) > 0.1)
					errs.push(`audio ${sound.duration} s, Bink ${durationS.toFixed(3)} s`)
				audioInfo = `${track.sampleRate}Hz ${channels}ch ${durationS.toFixed(3)}s`
			}
		}
		const psnr = /PSNR .*average:(\S+) min:(\S+)/.exec(compare.stderr)
		const psnrMin = psnr ? Number(psnr[2].replace("inf", "Infinity")) : NaN
		if (!(psnrMin >= MIN_PSNR))
			errs.push(`minimum PSNR ${psnr ? psnr[2] : "not measured"}`)
		return (
			`${name}.mp4 ${header.width}x${header.height} ${header.frames} frames ${audioInfo} ` +
			`PSNR avg ${psnr ? psnr[1] : "?"} min ${psnr ? psnr[2] : "?"} ` +
			(errs.length ? "FAIL: " + errs.join("; ") : "PASS")
		)
	})
	tally("ffprobe_movies", await pool(jobs))
}

// GUI: assets/data/gui/*.json and assets/patch/*.json
const gui = async () => {
	if (!has("jq", "--version")) {
		summary.push("jq missing: gui UNVERIFIED")
		return
	}
	console.log("gui...")
	const jobs = ["data/gui", "patch"].flatMap((dir) =>
		files(path.join(ASSETS, dir), ".json").map((file) => async () => {
			const result = await exec("jq", ["empty", path.join(ASSETS, dir, file)])
			return result.status === 0
				? `${dir}/${file} PASS`
				: `${dir}/${file} FAIL: jq: ${firstLine(result)}`
		}),
	)
	tally("jq_gui", await pool(jobs))
}

const main = async () => {
	if (run("models")) await models()
	if (run("textures")) await textures()
	if (run("tga")) await tgaImages()
	if (run("music")) await streamAudio("music", "streammusic")
	if (run("sounds")) await streamAudio("sounds", "streamsounds")
	if (run("movies")) await movies()
	if (run("gui")) await gui()
	console.log(summary.join("\n"))
}
main()
