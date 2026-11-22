// usage: node scripts/convert-all.js [models] [textures] [tga] [music] [sounds] [movies]   (default: all)
// Converts all supported game assets into ~/re/assets. Reports: ~/re/temp/reports.
// Audio verified with ffprobe if installed. Textures verified with ImageMagick if installed.
// Models UNVERIFIED (no independent loader yet). Movies require ffmpeg and are verified with ffprobe.
const fs = require("fs")
const path = require("path")
const childProcess = require("child_process")
const kotor = require("./kotor")
const tpc = require("./tpc2png")
const tga = require("./tga")
const png = require("./png")
const mdl2obj = require("./mdl2obj")
const audio = require("./audio")
const adpcm2pcm = require("./adpcm2pcm")
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

// Texture packs used for model materials, in priority order. tpb and tpc hold the same textures at
// lower resolution and are not converted.
const PACKS = ["swpc_tex_tpa", "swpc_tex_gui"]
const packErf = (pack) =>
	kotor.readErf(path.join(kotor.GAME, "TexturePacks", `${pack}.erf`))
// Removes files with the given extensions from dir that were not written in this run.
const prune = (dir, keep, exts) => {
	for (const file of fs.readdirSync(dir))
		if (exts.includes(path.extname(file)) && !keep.has(file))
			fs.unlinkSync(path.join(dir, file))
}

// Models: data/models.bif -> assets/data/models/*.obj + *.mtl (passing, with geometry)
if (run("models")) {
	console.log("models...")
	const out = path.join(ASSETS, "data", "models")
	fs.mkdirSync(out, { recursive: true })
	const texPath = new Map()
	for (const pack of [...PACKS].reverse())
		for (const entry of packErf(pack).entries)
			if (entry.ext === "tpc")
				texPath.set(
					entry.resref,
					`../../TexturePacks/${pack}/${entry.resref}.png`,
				)
	const resources = kotor.bifResources("data/models.bif")
	const mdx = new Map(
		resources
			.filter((resource) => resource.ext === "mdx")
			.map((resource) => [resource.resref, resource.data]),
	)
	const lines = [],
		written = new Set(),
		missing = new Set()
	let skipped = 0,
		hidden = 0
	for (const resource of resources.filter(
		(resource) => resource.ext === "mdl",
	)) {
		const model = mdl2obj.convert(
			resource.data,
			mdx.get(resource.resref) ?? Buffer.alloc(0),
			`${resource.resref}.mdl`,
			{ mtllib: `${resource.resref}.mtl` },
		)
		lines.push(mdl2obj.report(resource.resref, model))
		hidden += model.stats.hidden
		if (model.errors.length) continue
		if (model.stats.meshes === 0) {
			skipped++
			continue
		}
		const mtl = [`# materials for ${resource.resref}`]
		for (const texture of [...model.textures].sort()) {
			mtl.push("", `newmtl ${texture}`, "Kd 1.000000 1.000000 1.000000")
			if (texPath.has(texture)) mtl.push(`map_Kd ${texPath.get(texture)}`)
			else {
				mtl.push("# texture not found in the texture packs")
				missing.add(texture)
			}
		}
		fs.writeFileSync(path.join(out, `${resource.resref}.obj`), model.obj)
		fs.writeFileSync(
			path.join(out, `${resource.resref}.mtl`),
			mtl.join("\n") + "\n",
		)
		written.add(`${resource.resref}.obj`).add(`${resource.resref}.mtl`)
	}
	prune(out, written, [".obj", ".mtl"])
	tally("models", lines)
	summary.push(
		`models without rendered geometry skipped: ${skipped}`,
		`non-rendered meshes omitted: ${hidden}`,
		`model textures not found: ${missing.size} (${[...missing].sort().join(" ")})`,
	)
}

// Textures: TexturePacks/<pack>.erf -> assets/TexturePacks/<pack>/*.png (+ *.txi when present)
// Each PNG is decoded again by ImageMagick, if installed, and compared with the encoded pixels.
const magick = !childProcess.spawnSync("convert", ["-version"], {
	stdio: "ignore",
}).error
// Decodes a PNG with ImageMagick. The size goes to stderr and 8-bit RGBA pixels to stdout.
// Returns a report line that ends in PASS when size and pixels match the encoder input.
const magickCheck = (name, file, width, height, rgba) => {
	const result = childProcess.spawnSync(
		"convert",
		[
			"png:-",
			"-format",
			"%w %h",
			"-write",
			"info:fd:2",
			"-depth",
			"8",
			"rgba:-",
		],
		{ input: file, maxBuffer: 1 << 28 },
	)
	const err = String(result.stderr || result.error).trim()
	if (result.status !== 0) return `${name} FAIL: ${err.split("\n")[0]}`
	const size = err.split("\n").pop()
	if (size !== `${width} ${height}`)
		return `${name} FAIL: size ${size}, expected ${width} ${height}`
	if (Buffer.compare(result.stdout, rgba) !== 0)
		return `${name} ${size} FAIL: pixels differ`
	return `${name} ${size} PASS`
}
if (run("textures")) {
	for (const pack of PACKS) {
		console.log(`textures ${pack}...`)
		const out = path.join(ASSETS, "TexturePacks", pack)
		fs.mkdirSync(out, { recursive: true })
		const lines = [],
			checks = [],
			written = new Set()
		for (const entry of packErf(pack).entries) {
			if (entry.ext !== "tpc") {
				lines.push(`${entry.resref}.${entry.ext} not a texture, skipped PASS`)
				continue
			}
			const data = entry.data(),
				texture = tpc.parse(data),
				image = tpc.decode(data, texture)
			lines.push(tpc.report(entry.resref, texture))
			if (!image) continue
			const file = png.encode(image.width, image.height, image.rgba)
			fs.writeFileSync(path.join(out, `${entry.resref}.png`), file)
			written.add(`${entry.resref}.png`)
			if (texture.txi.replace(/\0/g, "").trim()) {
				fs.writeFileSync(
					path.join(out, `${entry.resref}.txi`),
					texture.txi.replace(/\0+$/, ""),
				)
				written.add(`${entry.resref}.txi`)
			}
			if (magick)
				checks.push(
					magickCheck(
						`${entry.resref}.png`,
						file,
						image.width,
						image.height,
						image.rgba,
					),
				)
		}
		prune(out, written, [".png", ".txi"])
		tally(`textures_${pack}`, lines)
		if (magick) tally(`imagemagick_${pack}`, checks)
		else summary.push(`ImageMagick missing: ${pack} UNVERIFIED`)
	}
}

// TGA images and their TXI texture options from three groups of BIF archives:
// data/lightmaps*.bif -> assets/data/lightmaps, data/textures.bif -> assets/data/textures,
// data/legacy.bif -> assets/data/legacy. Images become 8-bit PNG with the TGA's channels (gray, RGB or
// RGBA), top row first. Each image is checked three ways: the TGA parser must consume the file exactly,
// ImageMagick's own decoding of the TGA (with -auto-orient) must equal the PNG's pixels, and
// ImageMagick and pngcheck must both read the PNG.
const pngcheck = !childProcess.spawnSync("pngcheck", ["-h"], {
	stdio: "ignore",
}).error
// Decodes a TGA with ImageMagick into 8-bit RGBA, top row first. Returns a report line.
const magickTgaCheck = (name, data, width, height, rgba) => {
	const result = childProcess.spawnSync(
		"convert",
		["tga:-", "-auto-orient", "-depth", "8", "rgba:-"],
		{ input: data, maxBuffer: 1 << 28 },
	)
	if (result.status !== 0)
		return `${name} FAIL: ${
			String(result.stderr || result.error)
				.trim()
				.split("\n")[0]
		}`
	if (result.stdout.length !== width * height * 4)
		return `${name} FAIL: ImageMagick returned ${result.stdout.length} bytes, expected ${width * height * 4}`
	if (Buffer.compare(result.stdout, rgba) !== 0)
		return `${name} FAIL: pixels differ from ImageMagick's decoding of the TGA`
	return `${name} ${width}x${height} PASS`
}
// Runs pngcheck on every PNG in dir. Returns one report line per file.
const pngcheckDir = (dir) => {
	const files = fs.readdirSync(dir).filter((file) => file.endsWith(".png"))
	const result = childProcess.spawnSync(
		"pngcheck",
		files.map((file) => path.join(dir, file)),
		{ encoding: "utf8", maxBuffer: 1 << 26 },
	)
	const ok = new Set()
	for (const line of String(result.stdout).split("\n")) {
		const match = /^OK: (.*?) \(/.exec(line)
		if (match) ok.add(path.basename(match[1]))
	}
	return files.map((file) =>
		ok.has(file) ? `${file} PASS` : `${file} FAIL: pngcheck`,
	)
}
const TGA_GROUPS = [
	["lightmaps", /^data\/lightmaps\d*\.bif$/],
	["textures", /^data\/textures\.bif$/],
	["legacy", /^data\/legacy\.bif$/],
]
if (run("tga")) {
	const key = kotor.readKey()
	for (const [group, pattern] of TGA_GROUPS) {
		console.log(`tga ${group}...`)
		const out = path.join(ASSETS, "data", group)
		fs.mkdirSync(out, { recursive: true })
		const lines = [],
			checks = [],
			written = new Set()
		for (const bif of key.bifs.filter((name) => pattern.test(name))) {
			for (const resource of kotor.bifResources(bif, key)) {
				const label = `${path.basename(bif)}:${resource.resref}.${resource.ext}`
				if (resource.ext !== "tga" && resource.ext !== "txi") {
					lines.push(`${label} not an image, skipped PASS`)
					continue
				}
				const file = `${resource.resref}.${resource.ext === "tga" ? "png" : "txi"}`
				if (written.has(file)) {
					lines.push(
						`${label} FAIL: ${file} already written from another archive`,
					)
					continue
				}
				if (resource.ext === "txi") {
					fs.writeFileSync(path.join(out, file), resource.data)
					written.add(file)
					lines.push(`${label} ${resource.data.length} bytes PASS`)
					continue
				}
				const header = tga.parse(resource.data),
					image = tga.decode(resource.data, header)
				lines.push(tga.report(label, header))
				if (!image) continue
				const encoded = png.encode(
					image.width,
					image.height,
					image.pix,
					image.channels,
				)
				fs.writeFileSync(path.join(out, file), encoded)
				written.add(file)
				if (magick) {
					checks.push(
						magickTgaCheck(
							`${resource.resref}.tga`,
							resource.data,
							image.width,
							image.height,
							image.rgba,
						),
					)
					checks.push(
						magickCheck(file, encoded, image.width, image.height, image.rgba),
					)
				}
			}
		}
		prune(out, written, [".png", ".txi"])
		tally(`tga_${group}`, lines)
		if (magick) tally(`imagemagick_${group}`, checks)
		else summary.push(`ImageMagick missing: ${group} UNVERIFIED`)
		if (pngcheck) tally(`pngcheck_${group}`, pngcheckDir(out))
		else summary.push(`pngcheck missing: ${group} PNG structure unchecked`)
	}
}

// Stream audio: streammusic/*.wav and streamsounds/*.wav -> assets/<folder>/*.mp3 | *.wav (PCM).
// Both folders use the layouts that audio.unwrap detects. adpcm2pcm.decode copies PCM unchanged.
const AUDIO = [
	["music", "streammusic"],
	["sounds", "streamsounds"],
].filter(([stage]) => run(stage))
for (const [stage, folder] of AUDIO) {
	console.log(`${stage}...`)
	const src = path.join(kotor.GAME, folder),
		out = path.join(ASSETS, folder)
	fs.mkdirSync(out, { recursive: true })
	const lines = []
	for (const file of fs
		.readdirSync(src)
		.filter((file) => /\.wav$/i.test(file))
		.sort()) {
		const name = file.replace(/\.wav$/i, ""),
			unwrapped = audio.unwrap(fs.readFileSync(path.join(src, file)))
		let data = unwrapped.payload,
			err = unwrapped.err,
			info = unwrapped.info
		if (!err && unwrapped.kind === "wav") {
			const decoded = adpcm2pcm.decode(data)
			data = decoded.out
			err = decoded.err
			info += " " + decoded.info
		}
		lines.push(
			`${name} ${unwrapped.kind} ${info} ${err ? "FAIL: " + err : "PASS"}`,
		)
		if (!err)
			fs.writeFileSync(path.join(out, `${name}.${unwrapped.kind}`), data)
	}
	tally(stage, lines)
}

// Movies: movies/*.bik -> assets/movies/*.mp4 (H.264 High profile, yuv420p; AAC LC audio).
// Requires ffmpeg, which decodes Bink. Each output is checked with ffprobe against the Bink parser:
// size, frame count, sample rate, channel count and audio duration (within 0.1 s; ffmpeg's Bink
// audio decoder emits about 0.03 s more than the packets declare). Then ffmpeg compares every
// output frame with the decoded Bink frame and requires a minimum PSNR of 40 dB.
const MIN_PSNR = 40
const has = (program) =>
	!childProcess.spawnSync(program, ["-version"], { stdio: "ignore" }).error
if (run("movies") && !(has("ffmpeg") && has("ffprobe")))
	summary.push("ffmpeg or ffprobe missing: movies not converted")
else if (run("movies")) {
	console.log("movies...")
	const src = path.join(kotor.GAME, "movies"),
		out = path.join(ASSETS, "movies")
	fs.mkdirSync(out, { recursive: true })
	const lines = [],
		written = new Set()
	for (const file of fs
		.readdirSync(src)
		.filter((file) => /\.bik$/i.test(file))
		.sort()) {
		const name = file.replace(/\.bik$/i, ""),
			inFile = path.join(src, file),
			outFile = path.join(out, `${name}.mp4`)
		const movie = bink.parse(fs.readFileSync(inFile))
		if (movie.errors.length) {
			lines.push(bink.report(file, movie))
			continue
		}
		const header = movie.header
		const encodeResult = childProcess.spawnSync(
			"ffmpeg",
			[
				"-v",
				"error",
				"-y",
				"-i",
				inFile,
				"-map",
				"0:v",
				"-map",
				"0:a?",
				"-c:v",
				"libx264",
				"-preset",
				"slow",
				"-crf",
				"18",
				"-pix_fmt",
				"yuv420p",
				"-c:a",
				"aac",
				"-b:a",
				"192k",
				"-movflags",
				"+faststart",
				outFile,
			],
			{ encoding: "utf8" },
		)
		const encodeError = String(
			encodeResult.stderr || encodeResult.error || "",
		).trim()
		if (encodeResult.status !== 0 || encodeError) {
			lines.push(`${name}.mp4 FAIL: ffmpeg: ${encodeError.split("\n")[0]}`)
			continue
		}
		written.add(`${name}.mp4`)
		const probe = childProcess.spawnSync(
			"ffprobe",
			[
				"-v",
				"error",
				"-count_frames",
				"-of",
				"json",
				"-show_entries",
				"stream=codec_type,codec_name,width,height,nb_read_frames,sample_rate,channels,duration",
				outFile,
			],
			{ encoding: "utf8" },
		)
		const streams = probe.status === 0 ? JSON.parse(probe.stdout).streams : []
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
				dur = movie.decodedBytes[0] / (2 * channels * track.sampleRate)
			if (!sound || sound.codec_name !== "aac") errs.push("no AAC audio stream")
			else {
				if (Number(sound.sample_rate) !== track.sampleRate)
					errs.push(`${sound.sample_rate} Hz, Bink ${track.sampleRate} Hz`)
				if (sound.channels !== channels)
					errs.push(`${sound.channels} channels, Bink ${channels}`)
				if (Math.abs(Number(sound.duration) - dur) > 0.1)
					errs.push(`audio ${sound.duration} s, Bink ${dur.toFixed(3)} s`)
				audioInfo = `${track.sampleRate}Hz ${channels}ch ${dur.toFixed(3)}s`
			}
		}
		const compareResult = childProcess.spawnSync(
			"ffmpeg",
			[
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
			],
			{ encoding: "utf8", maxBuffer: 1 << 26 },
		)
		const psnr = /PSNR .*average:(\S+) min:(\S+)/.exec(
			compareResult.stderr || "",
		)
		const psnrMin = psnr ? Number(psnr[2].replace("inf", "Infinity")) : NaN
		if (!(psnrMin >= MIN_PSNR))
			errs.push(`minimum PSNR ${psnr ? psnr[2] : "not measured"}`)
		lines.push(
			`${name}.mp4 ${header.width}x${header.height} ${header.frames} frames ${audioInfo} ` +
				`PSNR avg ${psnr ? psnr[1] : "?"} min ${psnr ? psnr[2] : "?"} ` +
				(errs.length ? "FAIL: " + errs.join("; ") : "PASS"),
		)
	}
	prune(out, written, [".mp4"])
	tally("movies", lines)
}

// Independent verification (audio)
// childProcess.spawnSync reports a missing program in its error field instead of throwing.
const ffprobe = !childProcess.spawnSync("ffprobe", ["-version"], {
	stdio: "ignore",
}).error
for (const [stage, folder] of AUDIO) {
	if (!ffprobe) {
		summary.push(`ffprobe missing: ${stage} UNVERIFIED`)
		continue
	}
	const dir = path.join(ASSETS, folder)
	const lines = fs.readdirSync(dir).map((file) => {
		const result = childProcess.spawnSync(
			"ffprobe",
			["-v", "error", path.join(dir, file)],
			{
				encoding: "utf8",
			},
		)
		if (result.status === 0) return `${file} PASS`
		return `${file} FAIL: ${
			String(result.stderr || result.error)
				.trim()
				.split("\n")[0]
		}`
	})
	tally(`ffprobe_${stage}`, lines)
}
if (run("models")) summary.push("models UNVERIFIED (no independent loader)")

console.log(summary.join("\n"))
