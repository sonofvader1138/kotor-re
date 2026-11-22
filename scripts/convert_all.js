// usage: node scripts/convert_all.js [models] [textures] [music]   (default: all)
// Converts all supported game assets into ~/re/assets. Reports: ~/re/temp/reports.
// Audio verified with ffprobe if installed. Textures verified with pngjs (temp/verify) if installed.
// Models UNVERIFIED (no independent loader yet).
const fs = require("fs")
const path = require("path")
const { execFileSync } = require("child_process")
const { ROOT, GAME, bifResources, readErf } = require("./kotor")
const tpc = require("./tpc2png")
const PNG = require("./png")
const { convert, report } = require("./mdl2obj")
const { unwrap } = require("./kotor_audio")
const { decode } = require("./adpcm2pcm")

const A = path.join(ROOT, "assets"),
	R = path.join(ROOT, "temp", "reports")
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
const packErf = (p) => readErf(path.join(GAME, "TexturePacks", `${p}.erf`))
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
	const res = bifResources("data/models.bif")
	const mdx = new Map(
		res.filter((r) => r.ext === "mdx").map((r) => [r.resref, r.data]),
	)
	const lines = [],
		written = new Set(),
		missing = new Set()
	let skipped = 0,
		hidden = 0
	for (const r of res.filter((r) => r.ext === "mdl")) {
		const c = convert(
			r.data,
			mdx.get(r.resref) ?? Buffer.alloc(0),
			`${r.resref}.mdl`,
			{ mtllib: `${r.resref}.mtl` },
		)
		lines.push(report(r.resref, c))
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
// Each PNG is decoded again with pngjs, if installed in temp/verify, and compared pixel for pixel.
let pngjs = null
try {
	pngjs = require(
		path.join(ROOT, "temp", "verify", "node_modules", "pngjs"),
	).PNG
} catch {}
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
			if (pngjs) {
				try {
					const v = pngjs.sync.read(file)
					const ok =
						v.width === d.w &&
						v.height === d.h &&
						Buffer.compare(v.data, d.rgba) === 0
					checks.push(
						`${e.resref}.png ${v.width}x${v.height} ${ok ? "PASS" : "FAIL: pixels differ"}`,
					)
				} catch (err) {
					checks.push(`${e.resref}.png FAIL: ${err.message}`)
				}
			}
		}
		prune(out, written, [".png", ".txi"])
		tally(`textures_${p}`, lines)
		if (pngjs) tally(`pngjs_${p}`, checks)
		else summary.push(`pngjs missing: ${p} UNVERIFIED`)
	}
}

// Music: streammusic/*.wav -> assets/streammusic/*.mp3 | *.wav (PCM)
if (run("music")) {
	console.log("music...")
	const src = path.join(GAME, "streammusic"),
		out = path.join(A, "streammusic")
	fs.mkdirSync(out, { recursive: true })
	const lines = []
	for (const f of fs
		.readdirSync(src)
		.filter((f) => /\.wav$/i.test(f))
		.sort()) {
		const name = f.replace(/\.wav$/i, ""),
			u = unwrap(fs.readFileSync(path.join(src, f)))
		let data = u.payload,
			err = u.err,
			info = u.info
		if (!err && u.kind === "wav") {
			const d = decode(data)
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
let ffprobe = true
try {
	execFileSync("ffprobe", ["-version"], { stdio: "ignore" })
} catch {
	ffprobe = false
}
if (ffprobe && run("music")) {
	const dir = path.join(A, "streammusic")
	const lines = fs.readdirSync(dir).map((f) => {
		try {
			execFileSync("ffprobe", ["-v", "error", path.join(dir, f)], {
				stdio: "pipe",
			})
			return `${f} PASS`
		} catch (e) {
			return `${f} FAIL: ${String(e.stderr).trim().split("\n")[0]}`
		}
	})
	tally("ffprobe", lines)
} else if (run("music")) summary.push("ffprobe missing: audio UNVERIFIED")
if (run("models")) summary.push("models UNVERIFIED (no independent loader)")

console.log(summary.join("\n"))
