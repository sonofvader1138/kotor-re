// usage: node extract.js data/models.bif outdir [resref...]
// Writes <resref>.<ext> for matching resources (all if none given).
const fs = require('fs');
const path = require('path');
const { bifResources } = require('./kotor');

const [bif, out, ...want] = process.argv.slice(2);
const w = new Set(want.map(s => s.toLowerCase()));
fs.mkdirSync(out, { recursive: true });
let n = 0;
for (const r of bifResources(bif)) {
  if (w.size && !w.has(r.resref)) continue;
  fs.writeFileSync(path.join(out, `${r.resref}.${r.ext}`), r.data);
  n++;
}
console.log(`${n} resources written to ${out}`);
