// usage: node tpc2png.js in.tpc out.png
// TPC texture -> PNG (RGBA, 8 bits per channel). Layout: see FILE_FORMATS.md.
// Header 128 bytes: u32 dataSize (0 = uncompressed), f32 alpha threshold, u16 width, u16 height,
// u8 encoding, u8 mip count, zero padding. Pixel data for all mip levels follows. An optional
// ASCII TXI (texture info) trailer fills the rest of the file.
// Only the top mip level is decoded. Cube faces are stacked vertically; flipbook frames keep their grid.
const fs = require("fs")
const png = require("./png")

// Byte size of one image of width x height pixels for a given storage kind.
function levelSize(kind, width, height) {
	if (kind === "dxt1") {
		return Math.max(1, (width + 3) >> 2) * Math.max(1, (height + 3) >> 2) * 8
	}
	if (kind === "dxt5") {
		return Math.max(1, (width + 3) >> 2) * Math.max(1, (height + 3) >> 2) * 16
	}
	return width * height * { gray: 1, rgb: 3, rgba: 4 }[kind]
}

function parse(file) {
	const errors = []
	if (file.length < 128) {
		return { errors: ["shorter than header"] }
	}

	const dataSize = file.readUInt32LE(0),
		alpha = file.readFloatLE(4)
	const width = file.readUInt16LE(8),
		height = file.readUInt16LE(10),
		enc = file[12],
		mips = file[13]
	if (file.subarray(14, 128).some((byte) => byte)) {
		errors.push("header padding not zero")
	}

	let kind
	if (dataSize === 0) {
		kind = { 1: "gray", 2: "rgb", 4: "rgba" }[enc]
	} else {
		kind = { 2: "dxt1", 4: "dxt5" }[enc]
	}
	if (!kind) {
		return {
			errors: [...errors, `unknown encoding ${enc} with dataSize ${dataSize}`],
		}
	}

	// The pixel data is a list of sub-images, each with its own mip chain. Each sub-image is
	// placed at (x, y) in the output. Three layouts exist:
	//   plain:     one image with `mips` levels.
	//   cube:      height = 6 * width; six square faces with `mips` levels each, stacked vertically.
	//   flipbook:  dataSize covers numx * numy animation frames (counts from the TXI trailer),
	//              each with a full mip chain down to 1x1. The header mip count is 1 and is ignored.
	const chain = (imageWidth, imageHeight, levels) => {
		let size = 0
		for (let level = 0; level < levels; level++) {
			size += levelSize(
				kind,
				Math.max(1, imageWidth >> level),
				Math.max(1, imageHeight >> level),
			)
		}
		return size
	}

	let layout = "plain",
		images = [{ x: 0, y: 0, width, height, levels: mips }]
	if (width > 0 && height === 6 * width) {
		layout = "cube"
		images = Array.from({ length: 6 }, (_, face) => ({
			x: 0,
			y: face * width,
			width,
			height: width,
			levels: mips,
		}))
	} else if (
		dataSize &&
		dataSize !== levelSize(kind, width, height) &&
		128 + dataSize <= file.length
	) {
		const txiText = file.toString("latin1", 128 + dataSize)
		const columns = +(/^\s*numx\s+(\d+)/im.exec(txiText) || [])[1],
			rows = +(/^\s*numy\s+(\d+)/im.exec(txiText) || [])[1]
		if (
			columns > 0 &&
			rows > 0 &&
			width % columns === 0 &&
			height % rows === 0
		) {
			const frameWidth = width / columns,
				frameHeight = height / rows,
				levels = Math.log2(Math.max(frameWidth, frameHeight)) + 1
			layout = "flipbook"
			images = []
			for (let j = 0; j < rows; j++) {
				for (let i = 0; i < columns; i++) {
					images.push({
						x: i * frameWidth,
						y: j * frameHeight,
						width: frameWidth,
						height: frameHeight,
						levels,
					})
				}
			}
		}
	}

	// A recorded size of 16 for a single 8-byte DXT1 block is accepted: all such files end exactly after the mip chain.
	const top = levelSize(kind, images[0].width, images[0].height)
	if (
		dataSize &&
		dataSize !== top &&
		!(dataSize === 16 && top === 8) &&
		layout !== "flipbook"
	) {
		errors.push(`dataSize ${dataSize} matches no known layout`)
	}

	let pixels = 0
	for (const image of images) {
		image.offset = 128 + pixels
		pixels += chain(image.width, image.height, image.levels)
	}
	if (layout === "flipbook" && pixels !== dataSize) {
		errors.push(`flipbook frames total ${pixels}, dataSize ${dataSize}`)
	}

	const end = 128 + pixels
	if (end > file.length) {
		errors.push(`pixel data ends at ${end}, past end of file ${file.length}`)
	}

	const txi = end <= file.length ? file.subarray(end) : Buffer.alloc(0)
	if (
		txi.some(
			(byte) =>
				byte !== 0 &&
				byte !== 9 &&
				byte !== 10 &&
				byte !== 13 &&
				(byte < 32 || byte > 126),
		)
	) {
		errors.push("trailer not ASCII")
	}

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

// Expands a 16-bit RGB 5:6:5 color to 8 bits per channel.
const expandRgb565 = (color) => [
	((((color >> 11) & 31) * 255) / 31 + 0.5) | 0,
	((((color >> 5) & 63) * 255) / 63 + 0.5) | 0,
	(((color & 31) * 255) / 31 + 0.5) | 0,
]

// Decode one image (width x height) of the given kind at file[offset] into RGBA, top row first.
function decodeImage(file, offset, kind, width, height) {
	const out = Buffer.alloc(width * height * 4)
	if (kind === "gray" || kind === "rgb" || kind === "rgba") {
		const channels = { gray: 1, rgb: 3, rgba: 4 }[kind]
		for (let i = 0; i < width * height; i++) {
			const src = offset + i * channels
			if (channels === 1) {
				out.fill(file[src], 4 * i, 4 * i + 3)
				out[4 * i + 3] = 255
			} else {
				out[4 * i] = file[src]
				out[4 * i + 1] = file[src + 1]
				out[4 * i + 2] = file[src + 2]
				out[4 * i + 3] = channels === 4 ? file[src + 3] : 255
			}
		}
		return out
	}

	const blockSize = kind === "dxt1" ? 8 : 16,
		blocksWide = Math.max(1, (width + 3) >> 2),
		blocksHigh = Math.max(1, (height + 3) >> 2)
	for (let blockY = 0; blockY < blocksHigh; blockY++) {
		for (let blockX = 0; blockX < blocksWide; blockX++) {
			let block = offset + (blockY * blocksWide + blockX) * blockSize

			const alphas = new Array(16).fill(255)
			if (kind === "dxt5") {
				const alpha0 = file[block],
					alpha1 = file[block + 1],
					alphaTable = [alpha0, alpha1]
				if (alpha0 > alpha1) {
					for (let i = 1; i < 7; i++) {
						alphaTable.push((((7 - i) * alpha0 + i * alpha1) / 7 + 0.5) | 0)
					}
				} else {
					for (let i = 1; i < 5; i++) {
						alphaTable.push((((5 - i) * alpha0 + i * alpha1) / 5 + 0.5) | 0)
					}
					alphaTable.push(0, 255)
				}

				const bits = file.readUIntLE(block + 2, 6)
				for (let i = 0; i < 16; i++) {
					alphas[i] = alphaTable[Math.floor(bits / 2 ** (3 * i)) & 7]
				}
				block += 8
			}

			const raw0 = file.readUInt16LE(block),
				raw1 = file.readUInt16LE(block + 2),
				color0 = expandRgb565(raw0),
				color1 = expandRgb565(raw1)
			const palette = [color0, color1]
			if (raw0 > raw1 || kind === "dxt5") {
				palette.push(
					color0.map((channel, i) => ((2 * channel + color1[i]) / 3 + 0.5) | 0),
					color0.map((channel, i) => ((channel + 2 * color1[i]) / 3 + 0.5) | 0),
				)
			} else {
				palette.push(
					color0.map((channel, i) => (channel + color1[i]) >> 1),
					[0, 0, 0],
				)
			}

			const indices = file.readUInt32LE(block + 4)
			for (let i = 0; i < 16; i++) {
				const x = blockX * 4 + (i & 3),
					y = blockY * 4 + (i >> 2)
				if (x >= width || y >= height) {
					continue
				}

				const colorIndex = (indices >>> (2 * i)) & 3,
					dst = 4 * (y * width + x)
				out[dst] = palette[colorIndex][0]
				out[dst + 1] = palette[colorIndex][1]
				out[dst + 2] = palette[colorIndex][2]
				out[dst + 3] =
					kind === "dxt1" && raw0 <= raw1 && colorIndex === 3 ? 0 : alphas[i]
			}
		}
	}

	return out
}

// Top mip level of every sub-image, placed at its (x, y), as RGBA rows, top row first.
// TPC stores the bottom row of each image first (texture coordinate v = 0), so rows are reversed.
// Sub-images keep reading order: flipbook frame 0 is at the top left and cube face 0 is at the top.
function decode(file, texture = parse(file)) {
	if (texture.errors.length) {
		return null
	}

	const rgba = Buffer.alloc(texture.width * texture.height * 4)
	for (const image of texture.images) {
		const pix = decodeImage(
			file,
			image.offset,
			texture.kind,
			image.width,
			image.height,
		)
		for (let y = 0; y < image.height; y++) {
			pix.copy(
				rgba,
				4 * ((image.y + image.height - 1 - y) * texture.width + image.x),
				4 * y * image.width,
				4 * (y + 1) * image.width,
			)
		}
	}

	return { width: texture.width, height: texture.height, rgba }
}

function report(label, texture) {
	if (!texture.kind) {
		return `${label} FAIL: ${texture.errors.join("; ")}`
	}

	return (
		`${label} ${texture.width}x${texture.height} ${texture.kind} mips=${texture.mips} layout=${texture.layout} alpha=${texture.alpha.toFixed(4)} ` +
		`txi=${texture.txi.length} ` +
		(texture.errors.length ? "FAIL: " + texture.errors.join("; ") : "PASS")
	)
}

if (require.main === module) {
	const [fin, fout] = process.argv.slice(2)
	const file = fs.readFileSync(fin),
		texture = parse(file),
		image = decode(file, texture)
	console.log(report(fin, texture))

	if (image) {
		fs.writeFileSync(fout, png.encode(image.width, image.height, image.rgba))
	}
}

module.exports = { parse, decode, report, levelSize }
