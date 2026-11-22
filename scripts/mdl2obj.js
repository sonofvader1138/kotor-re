// usage: node mdl2obj.js in.mdl in.mdx out.obj
// MDL + MDX -> Wavefront OBJ. Layout: see FILE_FORMATS.md. MDL offsets relative to file + 12.
const fs = require('fs');

const NONE = 0xffffffff;

function qrot([w, x, y, z], [vx, vy, vz]) {
  const tx = 2 * (y * vz - z * vy), ty = 2 * (z * vx - x * vz), tz = 2 * (x * vy - y * vx);
  return [vx + w * tx + y * tz - z * ty, vy + w * ty + z * tx - x * tz, vz + w * tz + x * ty - y * tx];
}
function qmul([aw, ax, ay, az], [bw, bx, by, bz]) {
  return [aw * bw - ax * bx - ay * by - az * bz, aw * bx + ax * bw + ay * bz - az * by,
    aw * by - ax * bz + ay * bw + az * bx, aw * bz + ax * by - ay * bx + az * bw];
}
const f6 = v => v.toFixed(6);

// opts.mtllib: material library file name written at the top of the OBJ.
// Meshes whose render byte (mesh header 0x139) is 0 are skipped; the game does not appear to draw them.
function convert(mdl, mdx, label = '', opts = {}) {
  const B = 12, len = mdl.length - B, errors = [];
  const st = { nodes: 0, hidden: 0, meshes: 0, verts: 0, faces: 0, mismatch: 0, nok: 0, nchk: 0 };
  const inb = (o, n, what) => {
    if (o < 0 || o + n > len) { errors.push(`${what} out of bounds (${o}+${n}>${len})`); return false; }
    return true;
  };
  const u32 = o => inb(o, 4, 'u32') ? mdl.readUInt32LE(B + o) : undefined;
  const u16 = o => inb(o, 2, 'u16') ? mdl.readUInt16LE(B + o) : undefined;
  const u8 = o => inb(o, 1, 'u8') ? mdl[B + o] : undefined;
  const f32 = (o, n) => inb(o, 4 * n, 'f32') ? Array.from({ length: n }, (_, i) => mdl.readFloatLE(B + o + 4 * i)) : undefined;
  const cstr = (o, n) => mdl.toString('latin1', B + o, B + o + n).replace(/\0.*$/s, '');

  const root = u32(0x28), hdrNodes = u32(0x2c);
  const noff = u32(0xb8), ncnt = u32(0xc0);
  const names = [];
  for (let i = 0; i < ncnt; i++) names.push(cstr(u32(noff + 4 * i), 64));

  const out = [`# converted from ${label}`];
  if (opts.mtllib) out.push(`mtllib ${opts.mtllib}`);
  const textures = new Set();
  let vbase = 1;
  const seen = new Set();

  function mesh(m, name, q, p) {
    const foff = u32(m + 0x08), fcnt = u32(m + 0x0c);
    const tex = cstr(m + 0x58, 32);
    const stride = u32(m + 0xfc), opos = u32(m + 0x104), ouv = u32(m + 0x110);
    const vcnt = u16(m + 0x130);
    const mdxoff = u32(m + 0x144), vcoff = u32(m + 0x148);
    if (vcoff === undefined) return;
    inb(foff, 32 * fcnt, `faces ${name}`); inb(vcoff, 12 * vcnt, `verts ${name}`);
    let hasmdx = stride > 0 && vcnt > 0 && mdx.length > 0;
    if (hasmdx && mdxoff + stride * vcnt > mdx.length) { errors.push(`mdx range ${name}`); hasmdx = false; }
    if (u8(m + 0x139) === 0) { st.hidden++; return; }
    const hasuv = hasmdx && ouv !== NONE;
    st.meshes++; st.verts += vcnt; st.faces += fcnt;
    out.push(`o ${name}`);
    if (tex && tex.toLowerCase() !== 'null') { out.push(`usemtl ${tex.toLowerCase()}`); textures.add(tex.toLowerCase()); }
    const verts = [];
    for (let i = 0; i < vcnt; i++) {
      const v = f32(vcoff + 12 * i, 3); verts.push(v);
      if (hasmdx && opos !== NONE) {
        const o = mdxoff + stride * i + opos;
        if ([0, 1, 2].some(k => Math.abs(v[k] - mdx.readFloatLE(o + 4 * k)) > 1e-5)) st.mismatch++;
      }
      const w = qrot(q, v);
      out.push(`v ${f6(w[0] + p[0])} ${f6(w[1] + p[1])} ${f6(w[2] + p[2])}`);
      // vt for every vertex keeps v/vt indices aligned
      const t = hasuv ? [mdx.readFloatLE(mdxoff + stride * i + ouv), mdx.readFloatLE(mdxoff + stride * i + ouv + 4)] : [0, 0];
      out.push(`vt ${f6(t[0])} ${f6(t[1])}`);
    }
    for (let f = 0; f < fcnt; f++) {
      const fo = foff + 32 * f;
      const ix = [u16(fo + 26), u16(fo + 28), u16(fo + 30)];
      if (ix.some(i => !(i < vcnt))) { errors.push(`face index >= ${vcnt} in ${name}`); continue; }
      // stored normal must match winding
      const n = f32(fo, 3), [a, b, c] = ix.map(i => verts[i]);
      const u = [0, 1, 2].map(k => b[k] - a[k]), w = [0, 1, 2].map(k => c[k] - a[k]);
      const x = [u[1] * w[2] - u[2] * w[1], u[2] * w[0] - u[0] * w[2], u[0] * w[1] - u[1] * w[0]];
      const l = Math.hypot(...x);
      if (l > 1e-9) { st.nchk++; if ((x[0] * n[0] + x[1] * n[1] + x[2] * n[2]) / l > 0.99) st.nok++; }
      out.push('f ' + ix.map(i => `${i + vbase}/${i + vbase}`).join(' '));
    }
    vbase += vcnt;
  }

  function node(o, pq, pp, depth) {
    if (seen.has(o)) return errors.push(`node loop at ${o}`);
    if (depth > 200) return errors.push('depth');
    seen.add(o); st.nodes++;
    const flags = u16(o) ?? 0, nidx = u16(o + 4) ?? 0;
    const pos = f32(o + 0x10, 3), q = f32(o + 0x1c, 4);
    const coff = u32(o + 0x2c), ccnt = u32(o + 0x30) ?? 0;
    if (!pos || !q) return;
    const wq = qmul(pq, q), r = qrot(pq, pos), wp = [r[0] + pp[0], r[1] + pp[1], r[2] + pp[2]];
    const name = names[nidx] ?? `node${o}`;
    if (flags & 0x20) mesh(o + 80, name, wq, wp);
    for (let i = 0; i < ccnt; i++) { const c = u32(coff + 4 * i); if (c !== undefined) node(c, wq, wp, depth + 1); }
  }

  node(root, [1, 0, 0, 0], [0, 0, 0], 0);
  return { obj: out.join('\n') + '\n', st: { ...st, hdrNodes }, textures, errors };
}

function report(label, { st, errors }) {
  return `${label} nodes=${st.nodes} meshes=${st.meshes} hidden=${st.hidden} verts=${st.verts} faces=${st.faces} ` +
    `mdl/mdx_mismatch=${st.mismatch} hdrnodes=${st.hdrNodes} normals_ok=${st.nok}/${st.nchk} ` +
    (errors.length ? 'FAIL: ' + errors.slice(0, 5).join('; ') : 'PASS');
}

if (require.main === module) {
  const [fmdl, fmdx, fout] = process.argv.slice(2);
  const r = convert(fs.readFileSync(fmdl), fs.existsSync(fmdx) ? fs.readFileSync(fmdx) : Buffer.alloc(0), fmdl);
  fs.writeFileSync(fout, r.obj);
  console.log(report(fmdl, r));
}
module.exports = { convert, report };
