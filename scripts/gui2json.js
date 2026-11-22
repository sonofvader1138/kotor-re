// usage: node gui2json.js assetroot
// Converts every GUI resource (type 2047) to <resref>.json. Resources in data/gui.bif go to <assetroot>/data/gui/.
// Resources in patch.erf go to <assetroot>/patch/. Five patch.erf resources replace gui.bif resources of the same name.
// A struct becomes an object keyed by field label, in file order. A struct type other than 0 is kept as "__struct_type".
// Lists become arrays of objects, vectors become 3-element arrays, f32 values use the shortest decimal that round-trips.
const fs = require('fs');
const path = require('path');
const { GAME, bifResources, readErf, readGff } = require('./kotor');

function f32(x) {
  for (let p = 1; p <= 9; p++) { const s = +x.toPrecision(p); if (Math.fround(s) === x) return s; }
  return x;
}
function toJson(s) {
  const o = {};
  if (s.structType !== 0) o.__struct_type = s.structType;
  for (const f of s.fields) {
    o[f.label] = f.type === 14 ? toJson(f.value) : f.type === 15 ? f.value.map(toJson)
      : f.type === 8 ? f32(f.value) : f.type === 17 ? f.value.map(f32) : f.value;
  }
  return o;
}

const root = process.argv[2];
if (!root) { console.error('usage: node gui2json.js assetroot'); process.exit(1); }
const sources = [
  ['data/gui', bifResources('data/gui.bif').filter(r => r.type === 2047)],
  ['patch', readErf(path.join(GAME, 'patch.erf')).entries.filter(e => e.type === 2047)
    .map(e => ({ resref: e.resref, data: e.data() }))],
];
for (const [dir, list] of sources) {
  const out = path.join(root, dir);
  fs.mkdirSync(out, { recursive: true });
  for (const r of list) {
    const g = readGff(r.data, r.resref);
    if (g.fileType !== 'GUI ' || g.version !== 'V3.2') throw new Error(`${r.resref}: unexpected ${g.fileType}${g.version}`);
    fs.writeFileSync(path.join(out, `${r.resref}.json`), JSON.stringify(toJson(g.root), null, 1) + '\n');
  }
  console.log(`${list.length} GUI files written to ${out}`);
}
