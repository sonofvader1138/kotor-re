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
const IDX = [-1, -1, -1, -1, 2, 4, 6, 8]

function chunks(b) {
	const c = {}
	let p = 12
	while (p + 8 <= b.length) {
		const sz = b.readUInt32LE(p + 4)
		c[b.toString("latin1", p, p + 4)] = b.subarray(p + 8, p + 8 + sz)
		p += 8 + sz + (sz & 1)
	}
	return c
}

function pcmWav(pcm, ch, rate, bytes) {
	const h = Buffer.alloc(44)
	h.write("RIFF", 0)
	h.writeUInt32LE(36 + bytes, 4)
	h.write("WAVE", 8)
	h.write("fmt ", 12)
	h.writeUInt32LE(16, 16)
	h.writeUInt16LE(1, 20)
	h.writeUInt16LE(ch, 22)
	h.writeUInt32LE(rate, 24)
	h.writeUInt32LE(rate * ch * 2, 28)
	h.writeUInt16LE(ch * 2, 32)
	h.writeUInt16LE(16, 34)
	h.write("data", 36)
	h.writeUInt32LE(bytes, 40)
	return Buffer.concat([h, Buffer.from(pcm.buffer, 0, bytes)])
}

// -> {out, info, err}
function decode(b) {
	const c = chunks(b),
		fmt = c["fmt "],
		data = c.data,
		fact = c.fact?.readUInt32LE(0)
	const tag = fmt.readUInt16LE(0),
		ch = fmt.readUInt16LE(2),
		rate = fmt.readUInt32LE(4),
		ba = fmt.readUInt16LE(12)
	if (tag !== 0x11) return { out: b, info: `copied (fmt=${tag})`, err: "" }
	const spb = fmt.readUInt16LE(18)
	if (spb !== ((ba - 4 * ch) * 2) / ch + 1)
		return {
			out: null,
			info: "",
			err: `samples/block ${spb} vs blockalign ${ba}`,
		}
	const blocks = Math.floor(data.length / ba),
		total = blocks * spb
	const n = fact && fact <= total ? fact : total
	const pcm = new Int16Array(total * ch)
	let errSum = 0,
		errN = 0,
		maxErr = 0,
		clip = 0
	const last = new Array(ch).fill(null)

	for (let k = 0; k < blocks; k++) {
		const o = k * ba,
			base = k * spb,
			pred = [],
			idx = []
		for (let c = 0; c < ch; c++) {
			pred[c] = data.readInt16LE(o + 4 * c)
			idx[c] = data[o + 4 * c + 2]
			if (idx[c] > 88)
				return {
					out: null,
					info: "",
					err: `bad step index ${idx[c]} block ${k}`,
				}
			if (last[c] !== null) {
				const d = Math.abs(pred[c] - last[c])
				errSum += d
				errN++
				if (d > maxErr) maxErr = d
			}
			pcm[base * ch + c] = pred[c]
		}
		// after headers: 4-byte groups per channel, 8 samples each, low nibble first
		let q = o + 4 * ch
		for (let s = 1; q < o + ba; s += 8) {
			for (let c = 0; c < ch; c++) {
				for (let j = 0; j < 8; j++) {
					const byte = data[q + (j >> 1)],
						nib = j & 1 ? byte >> 4 : byte & 15,
						st = STEP[idx[c]]
					let diff = st >> 3
					if (nib & 1) diff += st >> 2
					if (nib & 2) diff += st >> 1
					if (nib & 4) diff += st
					if (nib & 8) diff = -diff
					let v = pred[c] + diff
					if (v > 32767) {
						v = 32767
						clip++
					} else if (v < -32768) {
						v = -32768
						clip++
					}
					pred[c] = v
					idx[c] = Math.min(88, Math.max(0, idx[c] + IDX[nib & 7]))
					pcm[(base + s + j) * ch + c] = v
				}
				q += 4
			}
		}
		for (let c = 0; c < ch; c++) last[c] = pred[c]
	}
	const partial = data.length - blocks * ba
	return {
		out: pcmWav(pcm, ch, rate, n * ch * 2),
		info: `blocks=${blocks} partial=${partial} samples=${n} boundary_err avg=${(errSum / errN).toFixed(2)} max=${maxErr} clips=${clip}`,
		err: partial ? "partial block" : "",
	}
}

if (require.main === module) {
	const [inp, out] = process.argv.slice(2)
	const r = decode(fs.readFileSync(inp))
	if (r.out) fs.writeFileSync(out, r.out)
	console.log(
		`${inp.replace(/.*\//, "")} ${r.info} ${r.err ? "FAIL: " + r.err : "PASS"}`,
	)
}
module.exports = { decode }
