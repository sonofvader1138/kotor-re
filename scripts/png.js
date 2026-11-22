// Minimal PNG codec. encode: 8-bit RGB or RGBA, filter 0. decode: 8-bit gray, RGB, gray+alpha, RGBA,
// non-interlaced, all five filter types; always returns RGBA.
const zlib = require('zlib');

function encode(w, h, pix, channels = 4) {
  const stride = w * channels, raw = Buffer.alloc((stride + 1) * h);
  for (let y = 0; y < h; y++) pix.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  const chunk = (type, data) => {
    const len = Buffer.alloc(4), crc = Buffer.alloc(4), td = Buffer.concat([Buffer.from(type, 'latin1'), data]);
    len.writeUInt32BE(data.length); crc.writeUInt32BE(zlib.crc32(td));
    return Buffer.concat([len, td, crc]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = channels === 4 ? 6 : 2;
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]);
}

function decode(b) {
  let o = 8, w, h, ct, idat = [];
  while (o < b.length) {
    const len = b.readUInt32BE(o), type = b.toString('latin1', o + 4, o + 8), d = b.subarray(o + 8, o + 8 + len);
    if (type === 'IHDR') {
      w = d.readUInt32BE(0); h = d.readUInt32BE(4); ct = d[9];
      if (d[8] !== 8 || d[12] !== 0 || ![0, 2, 4, 6].includes(ct)) throw new Error('unsupported PNG');
    } else if (type === 'IDAT') idat.push(d);
    o += 12 + len;
  }
  const n = { 0: 1, 2: 3, 4: 2, 6: 4 }[ct], stride = w * n, raw = zlib.inflateSync(Buffer.concat(idat));
  const px = Buffer.alloc(stride * h);
  for (let y = 0; y < h; y++) {
    const f = raw[y * (stride + 1)], src = y * (stride + 1) + 1, dst = y * stride;
    for (let x = 0; x < stride; x++) {
      const a = x >= n ? px[dst + x - n] : 0, up = y ? px[dst - stride + x] : 0, c = y && x >= n ? px[dst - stride + x - n] : 0;
      const p = a + up - c, pa = Math.abs(p - a), pb = Math.abs(p - up), pc = Math.abs(p - c);
      const pred = [0, a, up, (a + up) >> 1, pa <= pb && pa <= pc ? a : pb <= pc ? up : c][f];
      px[dst + x] = (raw[src + x] + pred) & 255;
    }
  }
  const rgba = Buffer.alloc(w * h * 4);
  for (let i = 0; i < w * h; i++) {
    const s = i * n;
    if (n <= 2) rgba.fill(px[s], 4 * i, 4 * i + 3), rgba[4 * i + 3] = n === 2 ? px[s + 1] : 255;
    else { px.copy(rgba, 4 * i, s, s + 3); rgba[4 * i + 3] = n === 4 ? px[s + 3] : 255; }
  }
  return { w, h, rgba };
}

module.exports = { encode, decode };
