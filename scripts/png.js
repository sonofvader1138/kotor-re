// Minimal PNG codec. encode: 8-bit gray, RGB or RGBA, filter 0. decode: 8-bit gray, RGB, gray+alpha, RGBA,
// non-interlaced, all five filter types; always returns RGBA.
const zlib = require("zlib")
const kotor = require("./kotor")

function encode(width, height, pix, channels = 4) {
	const stride = width * channels,
		raw = Buffer.alloc((stride + 1) * height)
	for (let y = 0; y < height; y++)
		pix.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride)
	const chunk = (type, data) => {
		const len = Buffer.alloc(4),
			crc = Buffer.alloc(4),
			typeAndData = Buffer.concat([Buffer.from(type, "latin1"), data])
		len.writeUInt32BE(data.length)
		crc.writeUInt32BE(zlib.crc32(typeAndData))
		return Buffer.concat([len, typeAndData, crc])
	}
	const ihdr = Buffer.alloc(13)
	ihdr.writeUInt32BE(width, 0)
	ihdr.writeUInt32BE(height, 4)
	ihdr[8] = 8
	ihdr[9] = { 1: 0, 3: 2, 4: 6 }[channels]
	return Buffer.concat([
		Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
		chunk("IHDR", ihdr),
		chunk("IDAT", zlib.deflateSync(raw)),
		chunk("IEND", Buffer.alloc(0)),
	])
}

function decode(file) {
	let offset = 8,
		width,
		height,
		colorType,
		idat = []
	while (offset < file.length) {
		const len = file.readUInt32BE(offset),
			type = file.toString("latin1", offset + 4, offset + 8),
			data = file.subarray(offset + 8, offset + 8 + len)
		if (type === "IHDR") {
			width = data.readUInt32BE(0)
			height = data.readUInt32BE(4)
			colorType = data[9]
			if (data[8] !== 8 || data[12] !== 0 || ![0, 2, 4, 6].includes(colorType))
				kotor.fatal("unsupported PNG")
		} else if (type === "IDAT") idat.push(data)
		offset += 12 + len
	}
	const channels = { 0: 1, 2: 3, 4: 2, 6: 4 }[colorType],
		stride = width * channels,
		raw = zlib.inflateSync(Buffer.concat(idat))
	const pix = Buffer.alloc(stride * height)
	for (let y = 0; y < height; y++) {
		const filter = raw[y * (stride + 1)],
			src = y * (stride + 1) + 1,
			dst = y * stride
		for (let x = 0; x < stride; x++) {
			// The PNG specification names the left, upper and upper-left neighbors a, b and c.
			const left = x >= channels ? pix[dst + x - channels] : 0,
				up = y ? pix[dst - stride + x] : 0,
				upLeft = y && x >= channels ? pix[dst - stride + x - channels] : 0
			const paeth = left + up - upLeft,
				distLeft = Math.abs(paeth - left),
				distUp = Math.abs(paeth - up),
				distUpLeft = Math.abs(paeth - upLeft)
			const pred = [
				0,
				left,
				up,
				(left + up) >> 1,
				distLeft <= distUp && distLeft <= distUpLeft
					? left
					: distUp <= distUpLeft
						? up
						: upLeft,
			][filter]
			pix[dst + x] = (raw[src + x] + pred) & 255
		}
	}
	const rgba = Buffer.alloc(width * height * 4)
	for (let i = 0; i < width * height; i++) {
		const src = i * channels
		if (channels <= 2)
			(rgba.fill(pix[src], 4 * i, 4 * i + 3),
				(rgba[4 * i + 3] = channels === 2 ? pix[src + 1] : 255))
		else {
			pix.copy(rgba, 4 * i, src, src + 3)
			rgba[4 * i + 3] = channels === 4 ? pix[src + 3] : 255
		}
	}
	return { width, height, rgba }
}

module.exports = { encode, decode }
