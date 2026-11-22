// usage: node dump.js file reloffset_hex count [base]
// Prints count u32 words as hex, int, float, ascii. Offset relative to base (default 12, MDL data).
const fs = require('fs');
const [file, off, count, base = '12'] = process.argv.slice(2);
const b = fs.readFileSync(file), B = +base, o0 = B + parseInt(off, 16);
for (let o = o0; o < o0 + 4 * count && o + 4 <= b.length; o += 4) {
  const u = b.readUInt32LE(o), f = b.readFloatLE(o);
  const s = b.toString('latin1', o, o + 4).replace(/[^\x20-\x7e]/g, '.');
  console.log(`+${(o - B).toString(16).padStart(4, '0')} ${u.toString(16).padStart(8, '0')} ${String(u).padStart(10)} ${f.toPrecision(5).padStart(12)} ${s}`);
}
