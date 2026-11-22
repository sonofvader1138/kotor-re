// Shared readers: KEY V1, BIFFV1.
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const GAME = path.join(ROOT, 'swkotor');
const EXT = { 2002: 'mdl', 3008: 'mdx' };

// KEY: header 8 magic, u32 bifCount, keyCount, bifTableOff, keyTableOff.
// Bif entry 12 bytes: u32 size, u32 nameOff, u16 nameLen, u16 drives.
// Key entry 22 bytes: char[16] resref, u16 type, u32 id (bif << 20 | index).
function readKey(file = path.join(GAME, 'chitin.key')) {
  const b = fs.readFileSync(file);
  if (b.toString('latin1', 0, 8) !== 'KEY V1  ') throw new Error(`${file}: not KEY V1`);
  const nb = b.readUInt32LE(8), nk = b.readUInt32LE(12), ob = b.readUInt32LE(16), ok = b.readUInt32LE(20);
  const bifs = [];
  for (let i = 0; i < nb; i++) {
    const o = ob + 12 * i, no = b.readUInt32LE(o + 4), nl = b.readUInt16LE(o + 8);
    bifs.push(b.toString('latin1', no, no + nl).replace(/\0.*$/, '').replace(/\\/g, '/'));
  }
  const keys = [];
  for (let i = 0; i < nk; i++) {
    const o = ok + 22 * i;
    const resref = b.toString('latin1', o, o + 16).replace(/\0.*$/, '').toLowerCase();
    const id = b.readUInt32LE(o + 18);
    keys.push({ resref, type: b.readUInt16LE(o + 16), bif: id >>> 20, index: id & 0xfffff });
  }
  return { bifs, keys };
}

// BIF: 8 magic, u32 varCount, u32 fixedCount, u32 tableOff.
// Var entry 16 bytes: u32 id, u32 offset, u32 size, u32 type.
function readBif(file) {
  const buf = fs.readFileSync(file);
  if (buf.toString('latin1', 0, 8) !== 'BIFFV1  ') throw new Error(`${file}: not BIFFV1`);
  const nv = buf.readUInt32LE(8), to = buf.readUInt32LE(16);
  const entries = [];
  for (let i = 0; i < nv; i++) {
    const o = to + 16 * i, offset = buf.readUInt32LE(o + 4), size = buf.readUInt32LE(o + 8);
    if (offset + size > buf.length) throw new Error(`${file}: entry ${i} out of bounds`);
    entries.push({ id: buf.readUInt32LE(o), offset, size, type: buf.readUInt32LE(o + 12) });
  }
  return { buf, entries };
}

// Resources of one BIF, named via KEY: [{resref, type, ext, data}]
function bifResources(bifName, key = readKey()) {
  const bi = key.bifs.indexOf(bifName);
  if (bi < 0) throw new Error(`${bifName} not in chitin.key`);
  const names = new Map();
  for (const k of key.keys) if (k.bif === bi) names.set(`${k.index}/${k.type}`, k.resref);
  const { buf, entries } = readBif(path.join(GAME, bifName));
  return entries.map(e => ({
    resref: names.get(`${e.id & 0xfffff}/${e.type}`) ?? `unnamed_${e.id & 0xfffff}`,
    type: e.type, ext: EXT[e.type] ?? String(e.type),
    data: buf.subarray(e.offset, e.offset + e.size),
  }));
}

module.exports = { ROOT, GAME, EXT, readKey, readBif, bifResources };
