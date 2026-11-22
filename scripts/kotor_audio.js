// usage: node kotor_audio.js [outdir] file.wav...
// KotOR stream audio:
//   A: 58-byte fake WAV header (data size 0) + MP3   -> <name>.mp3
//   B: MP3 stub frames + real RIFF WAV               -> <name>.wav (as stored; may be ADPCM)
// Validates MP3 frame walk / RIFF chunk walk consume payload exactly.
const fs = require("fs")
const path = require("path")

const BR1 = [0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320]
const BR2 = [0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160]
const SR = {
	3: [44100, 48000, 32000],
	2: [22050, 24000, 16000],
	0: [11025, 12000, 8000],
}

// Walk MP3 frames from p. Returns {end, frames, err}; err 'riff' = stopped at embedded RIFF.
function mp3walk(m, p = 0) {
	let n = 0
	if (m.toString("latin1", p, p + 3) === "ID3")
		p += 10 + ((m[p + 6] << 21) | (m[p + 7] << 14) | (m[p + 8] << 7) | m[p + 9])
	while (p < m.length) {
		if (m.length - p === 128 && m.toString("latin1", p, p + 3) === "TAG")
			return { end: p + 128, frames: n, err: "" }
		if (m.toString("latin1", p, p + 4) === "RIFF")
			return { end: p, frames: n, err: "riff" }
		if (p + 4 > m.length)
			return { end: p, frames: n, err: `trailing ${m.length - p}` }
		const h = m.readUInt32BE(p)
		if (h >>> 21 !== 0x7ff) return { end: p, frames: n, err: `no sync at ${p}` }
		const ver = (h >>> 19) & 3,
			lay = (h >>> 17) & 3,
			bri = (h >>> 12) & 15,
			sri = (h >>> 10) & 3,
			pad = (h >>> 9) & 1
		if (lay !== 1 || ver === 1 || bri === 0 || bri === 15 || sri === 3)
			return { end: p, frames: n, err: `bad header at ${p}` }
		p +=
			Math.floor(
				((ver === 3 ? 144 : 72) * (ver === 3 ? BR1 : BR2)[bri] * 1000) /
					SR[ver][sri],
			) + pad
		n++
	}
	return { end: p, frames: n, err: p > m.length ? "overrun" : "" }
}

function riffwalk(r) {
	if (r.readUInt32LE(4) !== r.length - 8) return "bad RIFF size"
	let p = 12
	const ids = []
	while (p + 8 <= r.length) {
		const sz = r.readUInt32LE(p + 4)
		ids.push(r.toString("latin1", p, p + 4))
		p += 8 + sz + (sz & 1)
	}
	if (p !== r.length) return `chunk end ${p}/${r.length}`
	return ids.includes("data") ? "" : "no data chunk"
}

// -> {kind: 'mp3'|'wav', payload, info, err}
function unwrap(b) {
	if (
		b.toString("latin1", 0, 4) === "RIFF" &&
		b.readUInt32LE(4) === 50 &&
		b.readUInt32LE(54) === 0
	) {
		const payload = b.subarray(58),
			w = mp3walk(payload)
		return { kind: "mp3", payload, info: `frames=${w.frames}`, err: w.err }
	}
	const w = mp3walk(b)
	if (w.err === "riff") {
		const payload = b.subarray(w.end)
		const fmt = payload.readUInt16LE(20),
			ch = payload.readUInt16LE(22),
			sr = payload.readUInt32LE(24),
			bits = payload.readUInt16LE(34)
		return {
			kind: "wav",
			payload,
			info: `stub=${w.end} fmt=${fmt} ch=${ch} sr=${sr} bits=${bits}`,
			err: riffwalk(payload),
		}
	}
	return { kind: "mp3", payload: b, info: `raw frames=${w.frames}`, err: w.err }
}

if (require.main === module) {
	const args = process.argv.slice(2)
	const out =
		args.length && fs.existsSync(args[0]) && fs.statSync(args[0]).isDirectory()
			? args.shift()
			: null
	for (const f of args) {
		const name = path.basename(f).replace(/\.wav$/i, ""),
			r = unwrap(fs.readFileSync(f))
		console.log(
			`${name.padEnd(24)} ${r.kind} ${r.info.padEnd(40)} ${r.err ? "FAIL: " + r.err : "PASS"}`,
		)
		if (out && !r.err)
			fs.writeFileSync(path.join(out, `${name}.${r.kind}`), r.payload)
	}
}
module.exports = { unwrap, mp3walk, riffwalk }
