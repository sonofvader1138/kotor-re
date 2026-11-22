// usage: node scripts/convert.js [models] [textures] [tga] [music] [sounds] [movies] [gui]   (default: all)
// Converts all supported game assets into ~/re/assets and writes each parser's report to ~/re/temp/reports.
// It does not check the outputs with independent tools. scripts/verify.js does that.
// Movies require ffmpeg.
// Each stage runs in its own process, and all stages run at the same time. Each stage writes its own
// outputs and reports and sends its summary lines to the main process, which prints them in stage order.
const fs = require("fs")
const path = require("path")
const childProcess = require("child_process")
const os = require("os")
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

// A stage process holds the summary lines of its own stage only.
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
	for (const file of fs.readdirSync(dir)) {
		if (exts.includes(path.extname(file)) && !keep.has(file)) {
			fs.unlinkSync(path.join(dir, file))
		}
	}
}

// Models: data/models.bif -> assets/data/models/*.obj + *.mtl (passing, with geometry)
const convertModels = () => {
	console.log("models...")
	const out = path.join(ASSETS, "data", "models")
	fs.mkdirSync(out, { recursive: true })

	const texPath = new Map()
	for (const pack of [...PACKS].reverse()) {
		for (const entry of packErf(pack).entries) {
			if (entry.ext === "tpc") {
				texPath.set(
					entry.resref,
					`../../TexturePacks/${pack}/${entry.resref}.png`,
				)
			}
		}
	}

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
		if (model.errors.length) {
			continue
		}
		if (model.stats.meshes === 0) {
			skipped++
			continue
		}

		const mtl = [`# materials for ${resource.resref}`]
		for (const texture of [...model.textures].sort()) {
			mtl.push("", `newmtl ${texture}`, "Kd 1.000000 1.000000 1.000000")
			if (texPath.has(texture)) {
				mtl.push(`map_Kd ${texPath.get(texture)}`)
			} else {
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
const convertTextures = () => {
	for (const pack of PACKS) {
		console.log(`textures ${pack}...`)
		const out = path.join(ASSETS, "TexturePacks", pack)
		fs.mkdirSync(out, { recursive: true })

		const lines = [],
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
			if (!image) {
				continue
			}

			fs.writeFileSync(
				path.join(out, `${entry.resref}.png`),
				png.encode(image.width, image.height, image.rgba),
			)
			written.add(`${entry.resref}.png`)

			if (texture.txi.replace(/\0/g, "").trim()) {
				fs.writeFileSync(
					path.join(out, `${entry.resref}.txi`),
					texture.txi.replace(/\0+$/, ""),
				)
				written.add(`${entry.resref}.txi`)
			}
		}

		prune(out, written, [".png", ".txi"])
		tally(`textures_${pack}`, lines)
	}
}

// TGA images and their TXI texture options from three groups of BIF archives:
// data/lightmaps*.bif -> assets/data/lightmaps, data/textures.bif -> assets/data/textures,
// data/legacy.bif -> assets/data/legacy. Images become 8-bit PNG with the TGA's channels (gray, RGB or
// RGBA), top row first. The TGA parser must consume each file exactly.
const TGA_GROUPS = [
	["lightmaps", /^data\/lightmaps\d*\.bif$/],
	["textures", /^data\/textures\.bif$/],
	["legacy", /^data\/legacy\.bif$/],
]
const convertTga = () => {
	const key = kotor.readKey()
	for (const [group, pattern] of TGA_GROUPS) {
		console.log(`tga ${group}...`)
		const out = path.join(ASSETS, "data", group)
		fs.mkdirSync(out, { recursive: true })

		const lines = [],
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
				if (!image) {
					continue
				}

				fs.writeFileSync(
					path.join(out, file),
					png.encode(image.width, image.height, image.pix, image.channels),
				)
				written.add(file)
			}
		}

		prune(out, written, [".png", ".txi"])
		tally(`tga_${group}`, lines)
	}
}

// Stream audio: streammusic/*.wav and streamsounds/*.wav -> assets/<folder>/*.mp3 | *.wav (PCM).
// Both folders use the layouts that audio.unwrap detects. adpcm2pcm.decode copies PCM unchanged.
const convertAudio = (stage, folder) => {
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
		if (!err) {
			fs.writeFileSync(path.join(out, `${name}.${unwrapped.kind}`), data)
		}
	}

	tally(stage, lines)
}

// Movies: movies/*.bik -> assets/movies/*.mp4 (H.264 High profile, yuv420p; AAC LC audio).
// Requires ffmpeg, which decodes Bink. One ffmpeg process does not keep all CPUs busy at this resolution,
// so MOVIE_JOBS of them run at once, largest movie first, so that the last ones to finish are short.
// CRF 12 keeps every frame of every movie at 40 dB PSNR or more against the Bink frame, which verify.js
// requires. At CRF 18, frames in 12 movies fell as low as 36.5 dB. CRF 12 makes files about 1.7 times larger.
const MOVIE_JOBS = Math.max(1, Math.floor(os.availableParallelism() / 3))
const convertMovies = async () => {
	if (
		childProcess.spawnSync("ffmpeg", ["-version"], { stdio: "ignore" }).error
	) {
		summary.push("ffmpeg missing: movies not converted")
		return
	}

	console.log("movies...")
	const src = path.join(kotor.GAME, "movies"),
		out = path.join(ASSETS, "movies")
	fs.mkdirSync(out, { recursive: true })
	const files = fs
		.readdirSync(src)
		.filter((file) => /\.bik$/i.test(file))
		.sort()

	// Resolves to the movie's report line, and adds the output to written when ffmpeg succeeds.
	const written = new Set()
	const convertMovie = async (file) => {
		const name = file.replace(/\.bik$/i, ""),
			inFile = path.join(src, file),
			outFile = path.join(out, `${name}.mp4`)
		const movie = bink.parse(fs.readFileSync(inFile))
		if (movie.errors.length) {
			return bink.report(file, movie)
		}

		const result = await kotor.exec("ffmpeg", [
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
			"12",
			"-pix_fmt",
			"yuv420p",
			"-c:a",
			"aac",
			"-b:a",
			"192k",
			"-movflags",
			"+faststart",
			outFile,
		])
		const err = String(result.stderr || result.error || "").trim()
		if (result.status !== 0 || err) {
			return `${name}.mp4 FAIL: ffmpeg: ${err.split("\n")[0]}`
		}

		written.add(`${name}.mp4`)
		return bink.report(file, movie)
	}

	const sizes = files.map((file) => fs.statSync(path.join(src, file)).size)
	const order = files.map((_, i) => i).sort((a, b) => sizes[b] - sizes[a])
	const lines = new Array(files.length)
	let next = 0
	await Promise.all(
		Array.from({ length: MOVIE_JOBS }, async () => {
			while (next < order.length) {
				const i = order[next++]
				lines[i] = await convertMovie(files[i])
			}
		}),
	)

	prune(out, written, [".mp4"])
	tally("movies", lines)
}

// GUI: resource type 2047 in data/gui.bif -> assets/data/gui/*.json, and in patch.erf -> assets/patch/*.json.
// Five patch.erf resources replace gui.bif resources of the same name.
// A struct becomes an object keyed by field label, in file order. A struct type other than 0 is kept as
// "__struct_type". Lists become arrays of objects, vectors become 3-element arrays, and f32 values use the
// shortest decimal that round-trips.
const f32 = (value) => {
	for (let digits = 1; digits <= 9; digits++) {
		const shortest = +value.toPrecision(digits)
		if (Math.fround(shortest) === value) {
			return shortest
		}
	}
	return value
}

const guiJson = (struct) => {
	const object = {}
	if (struct.structType !== 0) {
		object.__struct_type = struct.structType
	}

	for (const field of struct.fields) {
		object[field.label] =
			field.type === 14
				? guiJson(field.value)
				: field.type === 15
					? field.value.map(guiJson)
					: field.type === 8
						? f32(field.value)
						: field.type === 17
							? field.value.map(f32)
							: field.value
	}
	return object
}

const convertGui = () => {
	console.log("gui...")
	const sources = [
		[
			"data/gui",
			kotor
				.bifResources("data/gui.bif")
				.filter((resource) => resource.type === 2047),
		],
		[
			"patch",
			kotor
				.readErf(path.join(kotor.GAME, "patch.erf"))
				.entries.filter((entry) => entry.type === 2047)
				.map((entry) => ({ resref: entry.resref, data: entry.data() })),
		],
	]

	const lines = []
	for (const [dir, list] of sources) {
		const out = path.join(ASSETS, dir)
		fs.mkdirSync(out, { recursive: true })

		const written = new Set()
		for (const resource of list) {
			const gff = kotor.readGff(resource.data, resource.resref)
			if (gff.fileType !== "GUI " || gff.version !== "V3.2") {
				lines.push(
					`${dir}/${resource.resref} FAIL: unexpected ${gff.fileType}${gff.version}`,
				)
				continue
			}

			fs.writeFileSync(
				path.join(out, `${resource.resref}.json`),
				JSON.stringify(guiJson(gff.root), null, 1) + "\n",
			)
			written.add(`${resource.resref}.json`)
			lines.push(`${dir}/${resource.resref} PASS`)
		}

		prune(out, written, [".json"])
	}

	tally("gui", lines)
}

// Movies start first because they take longest.
const STAGES = [
	["movies", convertMovies],
	["models", convertModels],
	["textures", convertTextures],
	["tga", convertTga],
	["music", () => convertAudio("music", "streammusic")],
	["sounds", () => convertAudio("sounds", "streamsounds")],
	["gui", convertGui],
]
const REPORT_ORDER = [
	"models",
	"textures",
	"tga",
	"music",
	"sounds",
	"movies",
	"gui",
]

// Starts one process per selected stage and prints their summary lines in report order.
const main = async () => {
	const results = {}
	await Promise.all(
		STAGES.filter(([stage]) => run(stage)).map(
			([stage]) =>
				new Promise((resolve) => {
					const child = childProcess.fork(__filename, [], {
						env: { ...process.env, CONVERT_STAGE: stage },
					})
					child.on("message", (lines) => {
						results[stage] = lines
					})
					child.on("exit", (status) => {
						results[stage] ??= [`${stage}: stage exited with status ${status}`]
						resolve()
					})
				}),
		),
	)

	const lines = REPORT_ORDER.filter((stage) => results[stage]).flatMap(
		(stage) => results[stage],
	)
	console.log(lines.join("\n"))
}

const stageName = process.env.CONVERT_STAGE
if (stageName === undefined) {
	main()
} else {
	const convertStage = STAGES.find(([stage]) => stage === stageName)[1]
	Promise.resolve(convertStage()).then(() =>
		process.send(summary, () => process.disconnect()),
	)
}
