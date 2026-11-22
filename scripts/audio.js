// usage: node audio.js [outdir] file.wav...
// KotOR stream audio:
//   A: 58-byte fake WAV header (data size 0) + MP3   -> <name>.mp3
//   B: MP3 stub frames + real RIFF WAV               -> <name>.wav (as stored; may be ADPCM)
// Validates MP3 frame walk / RIFF chunk walk consume payload exactly.
const fs = require("fs")
const path = require("path")

const BITRATES_MPEG1 = [
	0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320,
]
const BITRATES_MPEG2 = [
	0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160,
]
const SAMPLE_RATES = {
	3: [44100, 48000, 32000],
	2: [22050, 24000, 16000],
	0: [11025, 12000, 8000],
}

// Walk MP3 frames from pos. Returns {end, frames, err}; err 'riff' = stopped at embedded RIFF.
function mp3walk(mp3, pos = 0) {
	let frames = 0
	if (mp3.toString("latin1", pos, pos + 3) === "ID3")
		pos +=
			10 +
			((mp3[pos + 6] << 21) |
				(mp3[pos + 7] << 14) |
				(mp3[pos + 8] << 7) |
				mp3[pos + 9])
	while (pos < mp3.length) {
		if (
			mp3.length - pos === 128 &&
			mp3.toString("latin1", pos, pos + 3) === "TAG"
		)
			return { end: pos + 128, frames, err: "" }
		if (mp3.toString("latin1", pos, pos + 4) === "RIFF")
			return { end: pos, frames, err: "riff" }
		if (pos + 4 > mp3.length)
			return { end: pos, frames, err: `trailing ${mp3.length - pos}` }
		const header = mp3.readUInt32BE(pos)
		if (header >>> 21 !== 0x7ff)
			return { end: pos, frames, err: `no sync at ${pos}` }
		const ver = (header >>> 19) & 3,
			lay = (header >>> 17) & 3,
			bitrateIndex = (header >>> 12) & 15,
			sampleRateIndex = (header >>> 10) & 3,
			pad = (header >>> 9) & 1
		if (
			lay !== 1 ||
			ver === 1 ||
			bitrateIndex === 0 ||
			bitrateIndex === 15 ||
			sampleRateIndex === 3
		)
			return { end: pos, frames, err: `bad header at ${pos}` }
		pos +=
			Math.floor(
				((ver === 3 ? 144 : 72) *
					(ver === 3 ? BITRATES_MPEG1 : BITRATES_MPEG2)[bitrateIndex] *
					1000) /
					SAMPLE_RATES[ver][sampleRateIndex],
			) + pad
		frames++
	}
	return { end: pos, frames, err: pos > mp3.length ? "overrun" : "" }
}

function riffwalk(riff) {
	if (riff.readUInt32LE(4) !== riff.length - 8) return "bad RIFF size"
	let pos = 12
	const ids = []
	while (pos + 8 <= riff.length) {
		const chunkSize = riff.readUInt32LE(pos + 4)
		ids.push(riff.toString("latin1", pos, pos + 4))
		pos += 8 + chunkSize + (chunkSize & 1)
	}
	if (pos !== riff.length) return `chunk end ${pos}/${riff.length}`
	return ids.includes("data") ? "" : "no data chunk"
}

// -> {kind: 'mp3'|'wav', payload, info, err}
function unwrap(file) {
	if (
		file.toString("latin1", 0, 4) === "RIFF" &&
		file.readUInt32LE(4) === 50 &&
		file.readUInt32LE(54) === 0
	) {
		const payload = file.subarray(58),
			walk = mp3walk(payload)
		return {
			kind: "mp3",
			payload,
			info: `frames=${walk.frames}`,
			err: walk.err,
		}
	}
	const walk = mp3walk(file)
	if (walk.err === "riff") {
		const payload = file.subarray(walk.end)
		const fmt = payload.readUInt16LE(20),
			channels = payload.readUInt16LE(22),
			sr = payload.readUInt32LE(24),
			bits = payload.readUInt16LE(34)
		return {
			kind: "wav",
			payload,
			info: `stub=${walk.end} fmt=${fmt} ch=${channels} sr=${sr} bits=${bits}`,
			err: riffwalk(payload),
		}
	}
	return {
		kind: "mp3",
		payload: file,
		info: `raw frames=${walk.frames}`,
		err: walk.err,
	}
}

if (require.main === module) {
	const args = process.argv.slice(2)
	const out =
		args.length && fs.existsSync(args[0]) && fs.statSync(args[0]).isDirectory()
			? args.shift()
			: null
	for (const file of args) {
		const name = path.basename(file).replace(/\.wav$/i, ""),
			result = unwrap(fs.readFileSync(file))
		console.log(
			`${name.padEnd(24)} ${result.kind} ${result.info.padEnd(40)} ${result.err ? "FAIL: " + result.err : "PASS"}`,
		)
		if (out && !result.err)
			fs.writeFileSync(path.join(out, `${name}.${result.kind}`), result.payload)
	}
}
module.exports = { unwrap, mp3walk, riffwalk }
