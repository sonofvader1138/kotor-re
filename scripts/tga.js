// usage: node tga.js in.tga out.png
// TGA (Truevision) image -> PNG. Layout: see FILE_FORMATS.md.
// Supports the layouts that occur in the game: uncompressed true color (image type 2) at 24 or 32 bits
// per pixel, and uncompressed grayscale (image type 3) at 8 bits per pixel, with no color map.
// An 18-byte header and an optional image ID precede the pixels. An optional TGA 2.0 extension area
// and 26-byte footer follow them.
const fs = require("fs")
const PNG = require("./png")

const FOOTER = "TRUEVISION-XFILE.\0"

// Reads the header and checks that the header, pixels and optional trailer consume the file exactly.
function parse(b) {
	const t = { errors: [] }
	if (b.length < 18) {
		t.errors.push(`file is ${b.length} bytes, shorter than the 18-byte header`)
		return t
	}
	t.idLength = b[0]
	t.colorMapType = b[1]
	t.imageType = b[2]
	t.width = b.readUInt16LE(12)
	t.height = b.readUInt16LE(14)
	t.bpp = b[16]
	t.descriptor = b[17]
	t.alphaBits = t.descriptor & 15
	t.topDown = (t.descriptor & 0x20) !== 0
	t.rightToLeft = (t.descriptor & 0x10) !== 0
	const kind = `${t.imageType}/${t.bpp}`
	if (t.colorMapType !== 0) t.errors.push(`color map type ${t.colorMapType}`)
	if (!["2/24", "2/32", "3/8"].includes(kind))
		t.errors.push(`image type ${t.imageType} at ${t.bpp} bits per pixel`)
	if (t.rightToLeft) t.errors.push("right-to-left pixel order")
	if (t.width === 0 || t.height === 0) t.errors.push("zero width or height")
	t.channels = t.bpp / 8
	t.pixelOffset = 18 + t.idLength
	t.pixelEnd = t.pixelOffset + t.width * t.height * t.channels
	t.trailer = "none"
	if (t.pixelEnd > b.length) {
		t.errors.push(`pixels end at ${t.pixelEnd}, past end of file ${b.length}`)
		return t
	}
	const rest = b.length - t.pixelEnd
	if (rest === 0) return t
	if (rest < 26 || b.toString("latin1", b.length - 18) !== FOOTER) {
		t.errors.push(`${rest} bytes after the pixels without a TGA 2.0 footer`)
		return t
	}
	const extOffset = b.readUInt32LE(b.length - 26),
		devOffset = b.readUInt32LE(b.length - 22)
	if (devOffset !== 0) t.errors.push(`developer area at ${devOffset}`)
	if (rest === 26) {
		t.trailer = "footer"
		if (extOffset !== 0)
			t.errors.push(`extension area at ${extOffset}, but no room for one`)
		return t
	}
	t.trailer = "extension+footer"
	t.extSize = b.length - 26 - t.pixelEnd
	t.extSizeField = b.readUInt16LE(t.pixelEnd)
	if (extOffset !== t.pixelEnd)
		t.errors.push(`extension area at ${extOffset}, pixels end at ${t.pixelEnd}`)
	return t
}

// Returns the pixels top row first. pix has t.channels bytes per pixel in PNG order: gray, RGB or RGBA.
// rgba is the same image expanded to RGBA with alpha 255 where the file has no alpha channel.
function decode(b, t = parse(b)) {
	if (t.errors.length) return null
	const w = t.width,
		h = t.height,
		n = t.channels,
		stride = w * n
	const pix = Buffer.alloc(stride * h),
		rgba = Buffer.alloc(w * h * 4)
	for (let y = 0; y < h; y++) {
		const srcRow = t.pixelOffset + (t.topDown ? y : h - 1 - y) * stride
		for (let x = 0; x < w; x++) {
			const s = srcRow + x * n,
				d = y * stride + x * n,
				q = (y * w + x) * 4
			if (n === 1) {
				pix[d] = b[s]
				rgba[q] = rgba[q + 1] = rgba[q + 2] = b[s]
				rgba[q + 3] = 255
				continue
			}
			// TGA stores true color pixels as blue, green, red and, at 32 bits, alpha.
			pix[d] = rgba[q] = b[s + 2]
			pix[d + 1] = rgba[q + 1] = b[s + 1]
			pix[d + 2] = rgba[q + 2] = b[s]
			rgba[q + 3] = n === 4 ? b[s + 3] : 255
			if (n === 4) pix[d + 3] = b[s + 3]
		}
	}
	return { w, h, channels: n, pix, rgba }
}

function report(label, t) {
	if (t.width === undefined) return `${label} FAIL: ${t.errors.join("; ")}`
	return (
		`${label} ${t.width}x${t.height} type=${t.imageType} bpp=${t.bpp} alphaBits=${t.alphaBits} ` +
		`origin=${t.topDown ? "top" : "bottom"} trailer=${t.trailer}` +
		(t.extSize !== undefined ? ` ext=${t.extSize}/${t.extSizeField}` : "") +
		" " +
		(t.errors.length ? "FAIL: " + t.errors.join("; ") : "PASS")
	)
}

if (require.main === module) {
	const [fin, fout] = process.argv.slice(2)
	const b = fs.readFileSync(fin),
		t = parse(b),
		d = decode(b, t)
	console.log(report(fin, t))
	if (d) fs.writeFileSync(fout, PNG.encode(d.w, d.h, d.pix, d.channels))
}
module.exports = { parse, decode, report }
