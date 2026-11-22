// usage: node adpcm2pcm.js in.wav out.wav
// IMA ADPCM WAV (fmt 0x11) -> 16-bit PCM WAV. Other formats: copied unchanged.
// Check: block header predictor vs last decoded sample of previous block.
const fs = require("fs")
const audio = require("./audio")

// WAVE format tags from the fmt chunk.
const WAVE_FORMAT_PCM = 1
const WAVE_FORMAT_IMA_ADPCM = 0x11

const PCM_HEADER_SIZE = 44
const PCM_FMT_CHUNK_SIZE = 16
const PCM_BITS_PER_SAMPLE = 16
const PCM_BYTES_PER_SAMPLE = PCM_BITS_PER_SAMPLE / 8
const INT16_MAX = 32767
const INT16_MIN = -32768

const STEP = [
	7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45,
	50, 55, 60, 66, 73, 80, 88, 97, 107, 118, 130, 143, 157, 173, 190, 209, 230,
	253, 279, 307, 337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963,
	1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024, 3327,
	3660, 4026, 4428, 4871, 5358, 5894, 6484, 7132, 7845, 8630, 9493, 10442,
	11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794,
	32767,
]
const STEP_INDEX_CHANGE = [-1, -1, -1, -1, 2, 4, 6, 8]
const MAX_STEP_INDEX = STEP.length - 1

// Each block starts with one 4-byte header per channel: i16 first sample, u8 step index, u8 reserved.
// Then each channel in turn stores a 4-byte group of 8 samples, 4 bits each.
const BLOCK_HEADER_SIZE = 4
const GROUP_SIZE = 4
const GROUP_SAMPLES = 8

function chunks(file) {
	const found = {}
	let pos = audio.RIFF_HEADER_SIZE
	while (pos + audio.CHUNK_HEADER_SIZE <= file.length) {
		const chunkSize = file.readUInt32LE(pos + 4),
			start = pos + audio.CHUNK_HEADER_SIZE
		found[file.toString("latin1", pos, pos + 4)] = file.subarray(
			start,
			start + chunkSize,
		)
		pos = start + chunkSize + (chunkSize & 1)
	}
	return found
}

function pcmWav(pcm, channels, rate, bytes) {
	const header = Buffer.alloc(PCM_HEADER_SIZE)
	header.write("RIFF", 0)
	header.writeUInt32LE(PCM_HEADER_SIZE - audio.CHUNK_HEADER_SIZE + bytes, 4)
	header.write("WAVE", 8)

	header.write("fmt ", 12)
	header.writeUInt32LE(PCM_FMT_CHUNK_SIZE, 16)
	header.writeUInt16LE(WAVE_FORMAT_PCM, 20)
	header.writeUInt16LE(channels, 22)
	header.writeUInt32LE(rate, 24)
	header.writeUInt32LE(rate * channels * PCM_BYTES_PER_SAMPLE, 28)
	header.writeUInt16LE(channels * PCM_BYTES_PER_SAMPLE, 32)
	header.writeUInt16LE(PCM_BITS_PER_SAMPLE, 34)

	header.write("data", 36)
	header.writeUInt32LE(bytes, 40)

	return Buffer.concat([header, Buffer.from(pcm.buffer, 0, bytes)])
}

// -> {out, info, err}
function decode(file) {
	const found = chunks(file),
		fmt = found["fmt "],
		data = found.data,
		fact = found.fact?.readUInt32LE(0)
	const tag = fmt.readUInt16LE(0),
		channels = fmt.readUInt16LE(2),
		rate = fmt.readUInt32LE(4),
		blockAlign = fmt.readUInt16LE(12)
	if (tag !== WAVE_FORMAT_IMA_ADPCM) {
		return { out: file, info: `copied (fmt=${tag})`, err: "" }
	}

	const samplesPerBlock = fmt.readUInt16LE(18)
	const groupBytes = blockAlign - BLOCK_HEADER_SIZE * channels
	if (
		samplesPerBlock !==
		(groupBytes * (GROUP_SAMPLES / GROUP_SIZE)) / channels + 1
	) {
		return {
			out: null,
			info: "",
			err: `samples/block ${samplesPerBlock} vs blockalign ${blockAlign}`,
		}
	}

	const blocks = Math.floor(data.length / blockAlign),
		total = blocks * samplesPerBlock
	const sampleCount = fact && fact <= total ? fact : total
	const pcm = new Int16Array(total * channels)
	let errSum = 0,
		errN = 0,
		maxErr = 0,
		clip = 0
	const last = new Array(channels).fill(null)

	for (let block = 0; block < blocks; block++) {
		const blockStart = block * blockAlign,
			firstSample = block * samplesPerBlock,
			pred = [],
			idx = []

		for (let channel = 0; channel < channels; channel++) {
			const channelHeader = blockStart + BLOCK_HEADER_SIZE * channel
			pred[channel] = data.readInt16LE(channelHeader)
			idx[channel] = data[channelHeader + 2]
			if (idx[channel] > MAX_STEP_INDEX) {
				return {
					out: null,
					info: "",
					err: `bad step index ${idx[channel]} block ${block}`,
				}
			}

			if (last[channel] !== null) {
				const gap = Math.abs(pred[channel] - last[channel])
				errSum += gap
				errN++
				if (gap > maxErr) {
					maxErr = gap
				}
			}

			pcm[firstSample * channels + channel] = pred[channel]
		}

		// Within a group, the low nibble of each byte comes first.
		let pos = blockStart + BLOCK_HEADER_SIZE * channels
		for (
			let sample = 1;
			pos < blockStart + blockAlign;
			sample += GROUP_SAMPLES
		) {
			for (let channel = 0; channel < channels; channel++) {
				for (let j = 0; j < GROUP_SAMPLES; j++) {
					const byte = data[pos + (j >> 1)],
						nib = j & 1 ? byte >> 4 : byte & 15,
						step = STEP[idx[channel]]

					let diff = step >> 3
					if (nib & 1) {
						diff += step >> 2
					}
					if (nib & 2) {
						diff += step >> 1
					}
					if (nib & 4) {
						diff += step
					}
					if (nib & 8) {
						diff = -diff
					}

					let value = pred[channel] + diff
					if (value > INT16_MAX) {
						value = INT16_MAX
						clip++
					} else if (value < INT16_MIN) {
						value = INT16_MIN
						clip++
					}

					pred[channel] = value
					idx[channel] = Math.min(
						MAX_STEP_INDEX,
						Math.max(0, idx[channel] + STEP_INDEX_CHANGE[nib & 7]),
					)
					pcm[(firstSample + sample + j) * channels + channel] = value
				}
				pos += GROUP_SIZE
			}
		}

		for (let channel = 0; channel < channels; channel++) {
			last[channel] = pred[channel]
		}
	}

	const partial = data.length - blocks * blockAlign
	return {
		out: pcmWav(
			pcm,
			channels,
			rate,
			sampleCount * channels * PCM_BYTES_PER_SAMPLE,
		),
		info: `blocks=${blocks} partial=${partial} samples=${sampleCount} boundary_err avg=${(errSum / errN).toFixed(2)} max=${maxErr} clips=${clip}`,
		err: partial ? "partial block" : "",
	}
}

if (require.main === module) {
	const [inp, out] = process.argv.slice(2)
	const result = decode(fs.readFileSync(inp))
	if (result.out) {
		fs.writeFileSync(out, result.out)
	}

	console.log(
		`${inp.replace(/.*\//, "")} ${result.info} ${result.err ? "FAIL: " + result.err : "PASS"}`,
	)
}

module.exports = { decode }
