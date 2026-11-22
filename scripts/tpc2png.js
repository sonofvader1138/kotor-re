// usage: node tpc2png.js in.tpc out.png
// TPC texture -> PNG (RGBA, 8 bits per channel). Layout: see FILE_FORMATS.md.
// Header 128 bytes: u32 dataSize (0 = uncompressed), f32 alpha threshold, u16 width, u16 height,
// u8 encoding, u8 mip count, zero padding. Pixel data for all mip levels follows. An optional
// ASCII TXI (texture info) trailer fills the rest of the file.
// Only the top mip level is decoded. Cube faces are stacked vertically; flipbook frames keep their grid.
const fs = require("fs")
const PNG = require("./png")

// Byte size of one image of w x h pixels for a given storage kind.
function levelSize(kind, w, h) {
	if (kind === "dxt1")
		return Math.max(1, (w + 3) >> 2) * Math.max(1, (h + 3) >> 2) * 8
	if (kind === "dxt5")
		return Math.max(1, (w + 3) >> 2) * Math.max(1, (h + 3) >> 2) * 16
	return w * h * { gray: 1, rgb: 3, rgba: 4 }[kind]
}

function parse(b) {
	const errors = []
	if (b.length < 128) return { errors: ["shorter than header"] }
	const dataSize = b.readUInt32LE(0),
		alpha = b.readFloatLE(4)
	const width = b.readUInt16LE(8),
		height = b.readUInt16LE(10),
		enc = b[12],
		mips = b[13]
	if (b.subarray(14, 128).some((x) => x)) errors.push("header padding not zero")
	let kind
	if (dataSize === 0) kind = { 1: "gray", 2: "rgb", 4: "rgba" }[enc]
	else kind = { 2: "dxt1", 4: "dxt5" }[enc]
	if (!kind)
		return {
			errors: [...errors, `unknown encoding ${enc} with dataSize ${dataSize}`],
		}
	// The pixel data is a list of sub-images, each with its own mip chain. Each sub-image is
	// placed at (x, y) in the output. Three layouts exist:
	//   plain:     one image with `mips` levels.
	//   cube:      height = 6 * width; six square faces with `mips` levels each, stacked vertically.
	//   flipbook:  dataSize covers numx * numy animation frames (counts from the TXI trailer),
	//              each with a full mip chain down to 1x1. The header mip count is 1 and is ignored.
	const chain = (w, h, n) => {
		let s = 0
		for (let m = 0; m < n; m++)
			s += levelSize(kind, Math.max(1, w >> m), Math.max(1, h >> m))
		return s
	}
	let layout = "plain",
		images = [{ x: 0, y: 0, w: width, h: height, levels: mips }]
	if (width > 0 && height === 6 * width) {
		layout = "cube"
		images = Array.from({ length: 6 }, (_, f) => ({
			x: 0,
			y: f * width,
			w: width,
			h: width,
			levels: mips,
		}))
	} else if (
		dataSize &&
		dataSize !== levelSize(kind, width, height) &&
		128 + dataSize <= b.length
	) {
		const t = b.toString("latin1", 128 + dataSize)
		const nx = +(/^\s*numx\s+(\d+)/im.exec(t) || [])[1],
			ny = +(/^\s*numy\s+(\d+)/im.exec(t) || [])[1]
		if (nx > 0 && ny > 0 && width % nx === 0 && height % ny === 0) {
			const fw = width / nx,
				fh = height / ny,
				levels = Math.log2(Math.max(fw, fh)) + 1
			layout = "flipbook"
			images = []
			for (let j = 0; j < ny; j++)
				for (let i = 0; i < nx; i++)
					images.push({ x: i * fw, y: j * fh, w: fw, h: fh, levels })
		}
	}
	// A recorded size of 16 for a single 8-byte DXT1 block is accepted: all such files end exactly after the mip chain.
	const top = levelSize(kind, images[0].w, images[0].h)
	if (
		dataSize &&
		dataSize !== top &&
		!(dataSize === 16 && top === 8) &&
		layout !== "flipbook"
	)
		errors.push(`dataSize ${dataSize} matches no known layout`)
	let pixels = 0
	for (const im of images) {
		im.offset = 128 + pixels
		pixels += chain(im.w, im.h, im.levels)
	}
	if (layout === "flipbook" && pixels !== dataSize)
		errors.push(`flipbook frames total ${pixels}, dataSize ${dataSize}`)
	const end = 128 + pixels
	if (end > b.length)
		errors.push(`pixel data ends at ${end}, past end of file ${b.length}`)
	const txi = end <= b.length ? b.subarray(end) : Buffer.alloc(0)
	if (
		txi.some(
			(x) => x !== 0 && x !== 9 && x !== 10 && x !== 13 && (x < 32 || x > 126),
		)
	)
		errors.push("trailer not ASCII")
	return {
		dataSize,
		alpha,
		width,
		height,
		enc,
		mips,
		kind,
		layout,
		images,
		pixels,
		txi: txi.toString("latin1"),
		errors,
	}
}

const c565 = (v) => [
	((((v >> 11) & 31) * 255) / 31 + 0.5) | 0,
	((((v >> 5) & 63) * 255) / 63 + 0.5) | 0,
	(((v & 31) * 255) / 31 + 0.5) | 0,
]

// Decode one image (w x h) of the given kind at b[o] into RGBA, top row first.
function decodeImage(b, o, kind, w, h) {
	const out = Buffer.alloc(w * h * 4)
	if (kind === "gray" || kind === "rgb" || kind === "rgba") {
		const n = { gray: 1, rgb: 3, rgba: 4 }[kind]
		for (let i = 0; i < w * h; i++) {
			const s = o + i * n
			if (n === 1) (out.fill(b[s], 4 * i, 4 * i + 3), (out[4 * i + 3] = 255))
			else {
				out[4 * i] = b[s]
				out[4 * i + 1] = b[s + 1]
				out[4 * i + 2] = b[s + 2]
				out[4 * i + 3] = n === 4 ? b[s + 3] : 255
			}
		}
		return out
	}
	const bs = kind === "dxt1" ? 8 : 16,
		bw = Math.max(1, (w + 3) >> 2),
		bh = Math.max(1, (h + 3) >> 2)
	for (let by = 0; by < bh; by++)
		for (let bx = 0; bx < bw; bx++) {
			let p = o + (by * bw + bx) * bs
			const al = new Array(16).fill(255)
			if (kind === "dxt5") {
				const a0 = b[p],
					a1 = b[p + 1],
					t = [a0, a1]
				if (a0 > a1)
					for (let i = 1; i < 7; i++)
						t.push((((7 - i) * a0 + i * a1) / 7 + 0.5) | 0)
				else {
					for (let i = 1; i < 5; i++)
						t.push((((5 - i) * a0 + i * a1) / 5 + 0.5) | 0)
					t.push(0, 255)
				}
				const bits = b.readUIntLE(p + 2, 6)
				for (let i = 0; i < 16; i++)
					al[i] = t[Math.floor(bits / 2 ** (3 * i)) & 7]
				p += 8
			}
			const v0 = b.readUInt16LE(p),
				v1 = b.readUInt16LE(p + 2),
				C0 = c565(v0),
				C1 = c565(v1)
			const pal = [C0, C1]
			if (v0 > v1 || kind === "dxt5") {
				pal.push(
					C0.map((c, k) => ((2 * c + C1[k]) / 3 + 0.5) | 0),
					C0.map((c, k) => ((c + 2 * C1[k]) / 3 + 0.5) | 0),
				)
			} else
				pal.push(
					C0.map((c, k) => (c + C1[k]) >> 1),
					[0, 0, 0],
				)
			const idx = b.readUInt32LE(p + 4)
			for (let i = 0; i < 16; i++) {
				const x = bx * 4 + (i & 3),
					y = by * 4 + (i >> 2)
				if (x >= w || y >= h) continue
				const ci = (idx >>> (2 * i)) & 3,
					q = 4 * (y * w + x)
				out[q] = pal[ci][0]
				out[q + 1] = pal[ci][1]
				out[q + 2] = pal[ci][2]
				out[q + 3] = kind === "dxt1" && v0 <= v1 && ci === 3 ? 0 : al[i]
			}
		}
	return out
}

// Top mip level of every sub-image, placed at its (x, y), as RGBA rows, top row first.
// TPC stores the bottom row of each image first (texture coordinate v = 0), so rows are reversed.
// Sub-images keep reading order: flipbook frame 0 is at the top left and cube face 0 is at the top.
function decode(b, t = parse(b)) {
	if (t.errors.length) return null
	const rgba = Buffer.alloc(t.width * t.height * 4)
	for (const im of t.images) {
		const px = decodeImage(b, im.offset, t.kind, im.w, im.h)
		for (let y = 0; y < im.h; y++)
			px.copy(
				rgba,
				4 * ((im.y + im.h - 1 - y) * t.width + im.x),
				4 * y * im.w,
				4 * (y + 1) * im.w,
			)
	}
	return { w: t.width, h: t.height, rgba }
}

function report(label, t) {
	if (!t.kind) return `${label} FAIL: ${t.errors.join("; ")}`
	return (
		`${label} ${t.width}x${t.height} ${t.kind} mips=${t.mips} layout=${t.layout} alpha=${t.alpha.toFixed(4)} ` +
		`txi=${t.txi.length} ` +
		(t.errors.length ? "FAIL: " + t.errors.join("; ") : "PASS")
	)
}

if (require.main === module) {
	const [fin, fout] = process.argv.slice(2)
	const b = fs.readFileSync(fin),
		t = parse(b),
		d = decode(b, t)
	console.log(report(fin, t))
	if (d) fs.writeFileSync(fout, PNG.encode(d.w, d.h, d.rgba))
}
module.exports = { parse, decode, report, levelSize }
