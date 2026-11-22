// usage: node render_obj.js in.obj out.png [size]
// Software render, 4 orthographic views (front, back, left, right; Z up), z-buffer, back-face culling.
// Shading by face normal. Culled faces are not drawn, like a typical viewer.
const fs = require('fs');
const zlib = require('zlib');
const [inp, out, size = '400'] = process.argv.slice(2);
const S = +size;

const V = [], F = [];
for (const l of fs.readFileSync(inp, 'utf8').split('\n')) {
  const t = l.trim().split(/\s+/);
  if (t[0] === 'v') V.push(t.slice(1, 4).map(Number));
  else if (t[0] === 'f') {
    const ix = t.slice(1).map(s => +s.split('/')[0] - 1);
    for (let i = 1; i + 1 < ix.length; i++) F.push([ix[0], ix[i], ix[i + 1]]);
  }
}
const lo = [0, 1, 2].map(k => Math.min(...V.map(v => v[k]))), hi = [0, 1, 2].map(k => Math.max(...V.map(v => v[k])));
const c = [0, 1, 2].map(k => (lo[k] + hi[k]) / 2), r = Math.max(...[0, 1, 2].map(k => hi[k] - lo[k])) / 2 || 1;

// view: camera looks along -fwd; screen x = right, y = up (Z)
const views = [
  { right: [1, 0, 0], fwd: [0, -1, 0] },   // camera on -Y looking +Y? fwd = toward camera
  { right: [-1, 0, 0], fwd: [0, 1, 0] },
  { right: [0, -1, 0], fwd: [-1, 0, 0] },
  { right: [0, 1, 0], fwd: [1, 0, 0] },
];
const W = S * 4, H = S, img = Buffer.alloc(W * H * 3, 40);
const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];

views.forEach((vw, vi) => {
  const z = new Float64Array(S * S).fill(-Infinity), up = [0, 0, 1];
  const P = V.map(v => { const d = [0, 1, 2].map(k => (v[k] - c[k]) / r);
    return [S / 2 + dot(d, vw.right) * S * 0.45, S / 2 - dot(d, up) * S * 0.45, dot(d, vw.fwd)]; });
  for (const [a, b, d] of F) {
    const A = V[a], B = V[b], C = V[d];
    const u = [0, 1, 2].map(k => B[k] - A[k]), w = [0, 1, 2].map(k => C[k] - A[k]);
    const n = [u[1] * w[2] - u[2] * w[1], u[2] * w[0] - u[0] * w[2], u[0] * w[1] - u[1] * w[0]];
    const ln = Math.hypot(...n); if (!ln) continue;
    const facing = dot(n, vw.fwd) / ln;
    if (facing <= 0) continue;                       // back-face cull (CCW = front)
    const shade = Math.round(60 + 195 * facing);
    const [p, q, s] = [P[a], P[b], P[d]];
    const x0 = Math.max(0, Math.floor(Math.min(p[0], q[0], s[0]))), x1 = Math.min(S - 1, Math.ceil(Math.max(p[0], q[0], s[0])));
    const y0 = Math.max(0, Math.floor(Math.min(p[1], q[1], s[1]))), y1 = Math.min(S - 1, Math.ceil(Math.max(p[1], q[1], s[1])));
    const area = (q[0] - p[0]) * (s[1] - p[1]) - (q[1] - p[1]) * (s[0] - p[0]); if (!area) continue;
    for (let y = y0; y <= y1; y++) for (let x = x0; x <= x1; x++) {
      const w0 = ((q[0] - x) * (s[1] - y) - (q[1] - y) * (s[0] - x)) / area;
      const w1 = ((s[0] - x) * (p[1] - y) - (s[1] - y) * (p[0] - x)) / area, w2 = 1 - w0 - w1;
      if (w0 < 0 || w1 < 0 || w2 < 0) continue;
      const zz = w0 * p[2] + w1 * q[2] + w2 * s[2];
      if (zz <= z[y * S + x]) continue;
      z[y * S + x] = zz;
      const o = (y * W + vi * S + x) * 3; img[o] = img[o + 1] = img[o + 2] = shade;
    }
  }
});

// PNG encode
const crcT = Array.from({ length: 256 }, (_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
const crc = b => { let c = 0xffffffff; for (const x of b) c = crcT[(c ^ x) & 255] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
const chunk = (t, d) => { const l = Buffer.alloc(4); l.writeUInt32BE(d.length); const td = Buffer.concat([Buffer.from(t), d]); const cr = Buffer.alloc(4); cr.writeUInt32BE(crc(td)); return Buffer.concat([l, td, cr]); };
const raw = Buffer.alloc((W * 3 + 1) * H);
for (let y = 0; y < H; y++) img.copy(raw, y * (W * 3 + 1) + 1, y * W * 3, (y + 1) * W * 3);
const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(W, 0); ihdr.writeUInt32BE(H, 4); ihdr[8] = 8; ihdr[9] = 2;
fs.writeFileSync(out, Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]));
console.log(`${inp}: ${V.length} verts, ${F.length} faces -> ${out}`);
