// Minimal PNG encoder: 8-bit gray, RGB or RGBA, filter 0.
const zlib = require("zlib")

function encode(width, height, pix, channels = 4) {
	const stride = width * channels,
		raw = Buffer.alloc((stride + 1) * height)
	for (let y = 0; y < height; y++) {
		pix.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride)
	}

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

module.exports = { encode }
