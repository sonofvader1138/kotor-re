// usage: node tga.js in.tga out.png
// TGA (Truevision) image -> PNG. Layout: see FILE_FORMATS.md.
// Supports the layouts that occur in the game: uncompressed true color (image type 2) at 24 or 32 bits
// per pixel, and uncompressed grayscale (image type 3) at 8 bits per pixel, with no color map.
// An 18-byte header and an optional image ID precede the pixels. An optional TGA 2.0 extension area
// and 26-byte footer follow them.
const fs = require("fs")
const png = require("./png")

const HEADER_SIZE = 18

// The 26-byte footer holds u32 extension area offset, u32 developer area offset and this 18-byte signature.
const FOOTER_SIZE = 26
const FOOTER_SIGNATURE = "TRUEVISION-XFILE.\0"

const IMAGE_TYPE = { trueColor: 2, grayscale: 3 }

// "image type/bits per pixel" pairs that occur in the game.
const SUPPORTED_KINDS = [
	`${IMAGE_TYPE.trueColor}/24`,
	`${IMAGE_TYPE.trueColor}/32`,
	`${IMAGE_TYPE.grayscale}/8`,
]

// Image descriptor bits: alpha bit count in bits 0 to 3, right-to-left order in bit 4, top-down order in bit 5.
const DESCRIPTOR_ALPHA_BITS = 0x0f
const DESCRIPTOR_RIGHT_TO_LEFT = 0x10
const DESCRIPTOR_TOP_DOWN = 0x20

const OPAQUE = 255

// Reads the header and checks that the header, pixels and optional trailer consume the file exactly.
function parse(file) {
	const header = { errors: [] }
	if (file.length < HEADER_SIZE) {
		header.errors.push(
			`file is ${file.length} bytes, shorter than the ${HEADER_SIZE}-byte header`,
		)
		return header
	}

	header.idLength = file[0]
	header.colorMapType = file[1]
	header.imageType = file[2]
	header.width = file.readUInt16LE(12)
	header.height = file.readUInt16LE(14)
	header.bpp = file[16]
	header.descriptor = file[17]
	header.alphaBits = header.descriptor & DESCRIPTOR_ALPHA_BITS
	header.topDown = (header.descriptor & DESCRIPTOR_TOP_DOWN) !== 0
	header.rightToLeft = (header.descriptor & DESCRIPTOR_RIGHT_TO_LEFT) !== 0

	const kind = `${header.imageType}/${header.bpp}`
	if (header.colorMapType !== 0) {
		header.errors.push(`color map type ${header.colorMapType}`)
	}
	if (!SUPPORTED_KINDS.includes(kind)) {
		header.errors.push(
			`image type ${header.imageType} at ${header.bpp} bits per pixel`,
		)
	}
	if (header.rightToLeft) {
		header.errors.push("right-to-left pixel order")
	}
	if (header.width === 0 || header.height === 0) {
		header.errors.push("zero width or height")
	}

	header.channels = header.bpp / 8
	header.pixelOffset = HEADER_SIZE + header.idLength
	header.pixelEnd =
		header.pixelOffset + header.width * header.height * header.channels
	header.trailer = "none"
	if (header.pixelEnd > file.length) {
		header.errors.push(
			`pixels end at ${header.pixelEnd}, past end of file ${file.length}`,
		)
		return header
	}

	const rest = file.length - header.pixelEnd
	if (rest === 0) {
		return header
	}
	if (
		rest < FOOTER_SIZE ||
		file.toString("latin1", file.length - FOOTER_SIGNATURE.length) !==
			FOOTER_SIGNATURE
	) {
		header.errors.push(
			`${rest} bytes after the pixels without a TGA 2.0 footer`,
		)
		return header
	}

	const footer = file.length - FOOTER_SIZE,
		extOffset = file.readUInt32LE(footer),
		devOffset = file.readUInt32LE(footer + 4)
	if (devOffset !== 0) {
		header.errors.push(`developer area at ${devOffset}`)
	}
	if (rest === FOOTER_SIZE) {
		header.trailer = "footer"
		if (extOffset !== 0) {
			header.errors.push(`extension area at ${extOffset}, but no room for one`)
		}
		return header
	}

	header.trailer = "extension+footer"
	header.extSize = footer - header.pixelEnd
	header.extSizeField = file.readUInt16LE(header.pixelEnd)
	if (extOffset !== header.pixelEnd) {
		header.errors.push(
			`extension area at ${extOffset}, pixels end at ${header.pixelEnd}`,
		)
	}
	return header
}

// Returns the pixels top row first. pix has header.channels bytes per pixel in PNG order: gray, RGB or RGBA.
// rgba is the same image expanded to RGBA with alpha 255 where the file has no alpha channel.
function decode(file, header = parse(file)) {
	if (header.errors.length) {
		return null
	}

	const width = header.width,
		height = header.height,
		channels = header.channels,
		stride = width * channels
	const pix = Buffer.alloc(stride * height),
		rgba = Buffer.alloc(width * height * 4)

	for (let y = 0; y < height; y++) {
		const srcRow =
			header.pixelOffset + (header.topDown ? y : height - 1 - y) * stride

		for (let x = 0; x < width; x++) {
			const src = srcRow + x * channels,
				dst = y * stride + x * channels,
				dstRgba = (y * width + x) * 4
			if (channels === 1) {
				pix[dst] = file[src]
				rgba[dstRgba] = rgba[dstRgba + 1] = rgba[dstRgba + 2] = file[src]
				rgba[dstRgba + 3] = OPAQUE
				continue
			}

			// TGA stores true color pixels as blue, green, red and, at 32 bits, alpha.
			pix[dst] = rgba[dstRgba] = file[src + 2]
			pix[dst + 1] = rgba[dstRgba + 1] = file[src + 1]
			pix[dst + 2] = rgba[dstRgba + 2] = file[src]
			rgba[dstRgba + 3] = channels === 4 ? file[src + 3] : OPAQUE
			if (channels === 4) {
				pix[dst + 3] = file[src + 3]
			}
		}
	}

	return { width, height, channels, pix, rgba }
}

function report(label, header) {
	if (header.width === undefined) {
		return `${label} FAIL: ${header.errors.join("; ")}`
	}

	return (
		`${label} ${header.width}x${header.height} type=${header.imageType} bpp=${header.bpp} alphaBits=${header.alphaBits} ` +
		`origin=${header.topDown ? "top" : "bottom"} trailer=${header.trailer}` +
		(header.extSize !== undefined
			? ` ext=${header.extSize}/${header.extSizeField}`
			: "") +
		" " +
		(header.errors.length ? "FAIL: " + header.errors.join("; ") : "PASS")
	)
}

if (require.main === module) {
	const [fin, fout] = process.argv.slice(2)
	const file = fs.readFileSync(fin),
		header = parse(file),
		image = decode(file, header)
	console.log(report(fin, header))

	if (image) {
		fs.writeFileSync(
			fout,
			png.encode(image.width, image.height, image.pix, image.channels),
		)
	}
}

module.exports = { parse, decode, report }
