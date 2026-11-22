// usage: node scripts/convert_all.js [models] [textures] [music]   (default: all)
// Converts all supported game assets into ~/re/assets. Reports: ~/re/temp/reports.
// Audio verified with ffprobe if installed. Textures verified with ImageMagick if installed.
// Models UNVERIFIED (no independent loader yet).
const fs = require("fs")
const path = require("path")
const childProcess = require("child_process")
const kotor = require("./kotor")
const tpc = require("./tpc2png")
const PNG = require("./png")
const mdl2obj = require("./mdl2obj")
const kotorAudio = require("./kotor_audio")
const adpcm2pcm = require("./adpcm2pcm")

const A = path.join(kotor.ROOT, "assets"),
	R = path.join(kotor.ROOT, "temp", "reports")
fs.mkdirSync(R, { recursive: true })
const want = new Set(process.argv.slice(2))
const run = (s) => want.size === 0 || want.has(s)
const summary = []
const tally = (name, lines) => {
	fs.writeFileSync(path.join(R, `${name}.txt`), lines.join("\n") + "\n")
	const pass = lines.filter((l) => l.endsWith("PASS")).length
	summary.push(`${name}: ${pass} pass, ${lines.length - pass} fail`)
}

// Texture packs used for model materials, in priority order. tpb and tpc hold the same textures at
// lower resolution and are not converted.
const PACKS = ["swpc_tex_tpa", "swpc_tex_gui"]
const packErf = (p) =>
	kotor.readErf(path.join(kotor.GAME, "TexturePacks", `${p}.erf`))
// Removes files with the given extensions from dir that were not written in this run.
const prune = (dir, keep, exts) => {
	for (const f of fs.readdirSync(dir))
		if (exts.includes(path.extname(f)) && !keep.has(f))
			fs.unlinkSync(path.join(dir, f))
}

// Models: data/models.bif -> assets/data/models/*.obj + *.mtl (passing, with geometry)
if (run("models")) {
	console.log("models...")
	const out = path.join(A, "data", "models")
	fs.mkdirSync(out, { recursive: true })
	const texPath = new Map()
	for (const p of [...PACKS].reverse())
		for (const e of packErf(p).entries)
			if (e.ext === "tpc")
				texPath.set(e.resref, `../../TexturePacks/${p}/${e.resref}.png`)
	const res = kotor.bifResources("data/models.bif")
	const mdx = new Map(
		res.filter((r) => r.ext === "mdx").map((r) => [r.resref, r.data]),
	)
	const lines = [],
		written = new Set(),
		missing = new Set()
	let skipped = 0,
		hidden = 0
	for (const r of res.filter((r) => r.ext === "mdl")) {
		const c = mdl2obj.convert(
			r.data,
			mdx.get(r.resref) ?? Buffer.alloc(0),
			`${r.resref}.mdl`,
			{ mtllib: `${r.resref}.mtl` },
		)
		lines.push(mdl2obj.report(r.resref, c))
		hidden += c.st.hidden
		if (c.errors.length) continue
		if (c.st.meshes === 0) {
			skipped++
			continue
		}
		const mtl = [`# materials for ${r.resref}`]
		for (const t of [...c.textures].sort()) {
			mtl.push("", `newmtl ${t}`, "Kd 1.000000 1.000000 1.000000")
			if (texPath.has(t)) mtl.push(`map_Kd ${texPath.get(t)}`)
			else {
				mtl.push("# texture not found in the texture packs")
				missing.add(t)
			}
		}
		fs.writeFileSync(path.join(out, `${r.resref}.obj`), c.obj)
		fs.writeFileSync(path.join(out, `${r.resref}.mtl`), mtl.join("\n") + "\n")
		written.add(`${r.resref}.obj`).add(`${r.resref}.mtl`)
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
const magickCheck = (name, file, w, h, rgba) => {
	const r = childProcess.spawnSync(
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
	const err = String(r.stderr || r.error).trim()
	if (r.status !== 0) return `${name} FAIL: ${err.split("\n")[0]}`
	const size = err.split("\n").pop()
	if (size !== `${w} ${h}`)
		return `${name} FAIL: size ${size}, expected ${w} ${h}`
	if (Buffer.compare(r.stdout, rgba) !== 0)
		return `${name} ${size} FAIL: pixels differ`
	return `${name} ${size} PASS`
}
if (run("textures")) {
	for (const p of PACKS) {
		console.log(`textures ${p}...`)
		const out = path.join(A, "TexturePacks", p)
		fs.mkdirSync(out, { recursive: true })
		const lines = [],
			checks = [],
			written = new Set()
		for (const e of packErf(p).entries) {
			if (e.ext !== "tpc") {
				lines.push(`${e.resref}.${e.ext} not a texture, skipped PASS`)
				continue
			}
			const b = e.data(),
				t = tpc.parse(b),
				d = tpc.decode(b, t)
			lines.push(tpc.report(e.resref, t))
			if (!d) continue
			const file = PNG.encode(d.w, d.h, d.rgba)
			fs.writeFileSync(path.join(out, `${e.resref}.png`), file)
			written.add(`${e.resref}.png`)
			if (t.txi.replace(/\0/g, "").trim()) {
				fs.writeFileSync(
					path.join(out, `${e.resref}.txi`),
					t.txi.replace(/\0+$/, ""),
				)
				written.add(`${e.resref}.txi`)
			}
			if (magick)
				checks.push(magickCheck(`${e.resref}.png`, file, d.w, d.h, d.rgba))
		}
		prune(out, written, [".png", ".txi"])
		tally(`textures_${p}`, lines)
		if (magick) tally(`imagemagick_${p}`, checks)
		else summary.push(`ImageMagick missing: ${p} UNVERIFIED`)
	}
}

// Music: streammusic/*.wav -> assets/streammusic/*.mp3 | *.wav (PCM)
if (run("music")) {
	console.log("music...")
	const src = path.join(kotor.GAME, "streammusic"),
		out = path.join(A, "streammusic")
	fs.mkdirSync(out, { recursive: true })
	const lines = []
	for (const f of fs
		.readdirSync(src)
		.filter((f) => /\.wav$/i.test(f))
		.sort()) {
		const name = f.replace(/\.wav$/i, ""),
			u = kotorAudio.unwrap(fs.readFileSync(path.join(src, f)))
		let data = u.payload,
			err = u.err,
			info = u.info
		if (!err && u.kind === "wav") {
			const d = adpcm2pcm.decode(data)
			data = d.out
			err = d.err
			info += " " + d.info
		}
		lines.push(`${name} ${u.kind} ${info} ${err ? "FAIL: " + err : "PASS"}`)
		if (!err) fs.writeFileSync(path.join(out, `${name}.${u.kind}`), data)
	}
	tally("music", lines)
}

// Independent verification (audio)
// childProcess.spawnSync reports a missing program in its error field instead of throwing.
const ffprobe = !childProcess.spawnSync("ffprobe", ["-version"], {
	stdio: "ignore",
}).error
if (ffprobe && run("music")) {
	const dir = path.join(A, "streammusic")
	const lines = fs.readdirSync(dir).map((f) => {
		const r = childProcess.spawnSync(
			"ffprobe",
			["-v", "error", path.join(dir, f)],
			{
				encoding: "utf8",
			},
		)
		if (r.status === 0) return `${f} PASS`
		return `${f} FAIL: ${
			String(r.stderr || r.error)
				.trim()
				.split("\n")[0]
		}`
	})
	tally("ffprobe", lines)
} else if (run("music")) summary.push("ffprobe missing: audio UNVERIFIED")
if (run("models")) summary.push("models UNVERIFIED (no independent loader)")

console.log(summary.join("\n"))
