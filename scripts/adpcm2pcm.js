// usage: node adpcm2pcm.js in.wav out.wav
// IMA ADPCM WAV (fmt 0x11) -> 16-bit PCM WAV. Other formats: copied unchanged.
// Check: block header predictor vs last decoded sample of previous block.
const fs = require("fs")

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

function chunks(file) {
	const found = {}
	let pos = 12
	while (pos + 8 <= file.length) {
		const chunkSize = file.readUInt32LE(pos + 4)
		found[file.toString("latin1", pos, pos + 4)] = file.subarray(
			pos + 8,
			pos + 8 + chunkSize,
		)
		pos += 8 + chunkSize + (chunkSize & 1)
	}
	return found
}

function pcmWav(pcm, channels, rate, bytes) {
	const header = Buffer.alloc(44)
	header.write("RIFF", 0)
	header.writeUInt32LE(36 + bytes, 4)
	header.write("WAVE", 8)
	header.write("fmt ", 12)
	header.writeUInt32LE(16, 16)
	header.writeUInt16LE(1, 20)
	header.writeUInt16LE(channels, 22)
	header.writeUInt32LE(rate, 24)
	header.writeUInt32LE(rate * channels * 2, 28)
	header.writeUInt16LE(channels * 2, 32)
	header.writeUInt16LE(16, 34)
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
	if (tag !== 0x11) return { out: file, info: `copied (fmt=${tag})`, err: "" }
	const samplesPerBlock = fmt.readUInt16LE(18)
	if (samplesPerBlock !== ((blockAlign - 4 * channels) * 2) / channels + 1)
		return {
			out: null,
			info: "",
			err: `samples/block ${samplesPerBlock} vs blockalign ${blockAlign}`,
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
			pred[channel] = data.readInt16LE(blockStart + 4 * channel)
			idx[channel] = data[blockStart + 4 * channel + 2]
			if (idx[channel] > 88)
				return {
					out: null,
					info: "",
					err: `bad step index ${idx[channel]} block ${block}`,
				}
			if (last[channel] !== null) {
				const gap = Math.abs(pred[channel] - last[channel])
				errSum += gap
				errN++
				if (gap > maxErr) maxErr = gap
			}
			pcm[firstSample * channels + channel] = pred[channel]
		}
		// after headers: 4-byte groups per channel, 8 samples each, low nibble first
		let pos = blockStart + 4 * channels
		for (let sample = 1; pos < blockStart + blockAlign; sample += 8) {
			for (let channel = 0; channel < channels; channel++) {
				for (let j = 0; j < 8; j++) {
					const byte = data[pos + (j >> 1)],
						nib = j & 1 ? byte >> 4 : byte & 15,
						step = STEP[idx[channel]]
					let diff = step >> 3
					if (nib & 1) diff += step >> 2
					if (nib & 2) diff += step >> 1
					if (nib & 4) diff += step
					if (nib & 8) diff = -diff
					let value = pred[channel] + diff
					if (value > 32767) {
						value = 32767
						clip++
					} else if (value < -32768) {
						value = -32768
						clip++
					}
					pred[channel] = value
					idx[channel] = Math.min(
						88,
						Math.max(0, idx[channel] + STEP_INDEX_CHANGE[nib & 7]),
					)
					pcm[(firstSample + sample + j) * channels + channel] = value
				}
				pos += 4
			}
		}
		for (let channel = 0; channel < channels; channel++)
			last[channel] = pred[channel]
	}
	const partial = data.length - blocks * blockAlign
	return {
		out: pcmWav(pcm, channels, rate, sampleCount * channels * 2),
		info: `blocks=${blocks} partial=${partial} samples=${sampleCount} boundary_err avg=${(errSum / errN).toFixed(2)} max=${maxErr} clips=${clip}`,
		err: partial ? "partial block" : "",
	}
}

if (require.main === module) {
	const [inp, out] = process.argv.slice(2)
	const result = decode(fs.readFileSync(inp))
	if (result.out) fs.writeFileSync(out, result.out)
	console.log(
		`${inp.replace(/.*\//, "")} ${result.info} ${result.err ? "FAIL: " + result.err : "PASS"}`,
	)
}
module.exports = { decode }
