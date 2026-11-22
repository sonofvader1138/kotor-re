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

// MPEG audio version codes from the frame header. Code 1 is reserved.
const MPEG_VERSION = { mpeg25: 0, reserved: 1, mpeg2: 2, mpeg1: 3 }
const SAMPLE_RATES = {
	[MPEG_VERSION.mpeg1]: [44100, 48000, 32000],
	[MPEG_VERSION.mpeg2]: [22050, 24000, 16000],
	[MPEG_VERSION.mpeg25]: [11025, 12000, 8000],
}

// Layer code 1 is Layer III. Bitrate index 0 means free format and 15 is invalid. Sample rate index 3 is reserved.
const LAYER_III = 1
const BITRATE_FREE = 0
const BITRATE_INVALID = 15
const SAMPLE_RATE_RESERVED = 3

// Every frame header starts with 11 set bits.
const FRAME_SYNC = 0x7ff

// A Layer III frame holds 1152 samples in MPEG-1 and 576 in MPEG-2 and 2.5. Its byte length is
// samples / 8 * bitrate / sample rate, which gives these coefficients.
const FRAME_COEFFICIENT_MPEG1 = 144
const FRAME_COEFFICIENT_MPEG2 = 72

const ID3V2_HEADER_SIZE = 10
const ID3V1_TAG_SIZE = 128

// RIFF: 4-byte id, u32 size of the rest of the file, 4-byte form type, then chunks of
// 4-byte id, u32 size, data padded to an even length.
const RIFF_HEADER_SIZE = 12
const CHUNK_HEADER_SIZE = 8

// Layout A: a 58-byte WAV header whose RIFF size field is 50 and whose data chunk size, at offset 54, is 0.
const FAKE_HEADER_SIZE = 58
const FAKE_RIFF_SIZE = 50
const FAKE_DATA_SIZE_OFFSET = 54

// Walk MP3 frames from pos. Returns {end, frames, err}; err 'riff' = stopped at embedded RIFF.
function mp3walk(mp3, pos = 0) {
	let frames = 0
	if (mp3.toString("latin1", pos, pos + 3) === "ID3") {
		pos +=
			ID3V2_HEADER_SIZE +
			((mp3[pos + 6] << 21) |
				(mp3[pos + 7] << 14) |
				(mp3[pos + 8] << 7) |
				mp3[pos + 9])
	}

	while (pos < mp3.length) {
		if (
			mp3.length - pos === ID3V1_TAG_SIZE &&
			mp3.toString("latin1", pos, pos + 3) === "TAG"
		) {
			return { end: pos + ID3V1_TAG_SIZE, frames, err: "" }
		}
		if (mp3.toString("latin1", pos, pos + 4) === "RIFF") {
			return { end: pos, frames, err: "riff" }
		}
		if (pos + 4 > mp3.length) {
			return { end: pos, frames, err: `trailing ${mp3.length - pos}` }
		}

		const header = mp3.readUInt32BE(pos)
		if (header >>> 21 !== FRAME_SYNC) {
			return { end: pos, frames, err: `no sync at ${pos}` }
		}

		const ver = (header >>> 19) & 3,
			lay = (header >>> 17) & 3,
			bitrateIndex = (header >>> 12) & 15,
			sampleRateIndex = (header >>> 10) & 3,
			pad = (header >>> 9) & 1
		if (
			lay !== LAYER_III ||
			ver === MPEG_VERSION.reserved ||
			bitrateIndex === BITRATE_FREE ||
			bitrateIndex === BITRATE_INVALID ||
			sampleRateIndex === SAMPLE_RATE_RESERVED
		) {
			return { end: pos, frames, err: `bad header at ${pos}` }
		}

		const mpeg1 = ver === MPEG_VERSION.mpeg1,
			bitrateKbps = (mpeg1 ? BITRATES_MPEG1 : BITRATES_MPEG2)[bitrateIndex]
		pos +=
			Math.floor(
				((mpeg1 ? FRAME_COEFFICIENT_MPEG1 : FRAME_COEFFICIENT_MPEG2) *
					bitrateKbps *
					1000) /
					SAMPLE_RATES[ver][sampleRateIndex],
			) + pad
		frames++
	}

	return { end: pos, frames, err: pos > mp3.length ? "overrun" : "" }
}

function riffwalk(riff) {
	if (riff.readUInt32LE(4) !== riff.length - CHUNK_HEADER_SIZE) {
		return "bad RIFF size"
	}

	let pos = RIFF_HEADER_SIZE
	const ids = []
	while (pos + CHUNK_HEADER_SIZE <= riff.length) {
		const chunkSize = riff.readUInt32LE(pos + 4)
		ids.push(riff.toString("latin1", pos, pos + 4))
		pos += CHUNK_HEADER_SIZE + chunkSize + (chunkSize & 1)
	}

	if (pos !== riff.length) {
		return `chunk end ${pos}/${riff.length}`
	}
	return ids.includes("data") ? "" : "no data chunk"
}

// -> {kind: 'mp3'|'wav', payload, info, err}
function unwrap(file) {
	if (
		file.toString("latin1", 0, 4) === "RIFF" &&
		file.readUInt32LE(4) === FAKE_RIFF_SIZE &&
		file.readUInt32LE(FAKE_DATA_SIZE_OFFSET) === 0
	) {
		const payload = file.subarray(FAKE_HEADER_SIZE),
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

		if (out && !result.err) {
			fs.writeFileSync(path.join(out, `${name}.${result.kind}`), result.payload)
		}
	}
}

module.exports = {
	RIFF_HEADER_SIZE,
	CHUNK_HEADER_SIZE,
	unwrap,
	mp3walk,
	riffwalk,
}
