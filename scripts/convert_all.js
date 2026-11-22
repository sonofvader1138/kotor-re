// usage: node scripts/convert_all.js [models] [music]   (default: all)
// Converts all supported game assets into ~/re/assets. Reports: ~/re/temp/reports.
// Audio verified with ffprobe if installed. Models UNVERIFIED (no independent loader yet).
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const { ROOT, GAME, bifResources } = require('./kotor');
const { convert, report } = require('./mdl2obj');
const { unwrap } = require('./kotor_audio');
const { decode } = require('./adpcm2pcm');

const A = path.join(ROOT, 'assets'), R = path.join(ROOT, 'temp', 'reports');
fs.mkdirSync(R, { recursive: true });
const want = new Set(process.argv.slice(2));
const run = s => want.size === 0 || want.has(s);
const summary = [];
const tally = (name, lines) => {
  fs.writeFileSync(path.join(R, `${name}.txt`), lines.join('\n') + '\n');
  const pass = lines.filter(l => l.endsWith('PASS')).length;
  summary.push(`${name}: ${pass} pass, ${lines.length - pass} fail`);
};

// Models: data/models.bif -> assets/data/models/*.obj (passing, with geometry)
if (run("models")) {
  console.log('models...');
  const out = path.join(A, 'data', 'models');
  fs.mkdirSync(out, { recursive: true });
  const res = bifResources('data/models.bif');
  const mdx = new Map(res.filter(r => r.ext === 'mdx').map(r => [r.resref, r.data]));
  const lines = [];
  let skipped = 0;
  for (const r of res.filter(r => r.ext === 'mdl')) {
    const c = convert(r.data, mdx.get(r.resref) ?? Buffer.alloc(0), `${r.resref}.mdl`);
    lines.push(report(r.resref, c));
    if (c.errors.length) continue;
    if (c.st.meshes === 0) { skipped++; continue; }
    fs.writeFileSync(path.join(out, `${r.resref}.obj`), c.obj);
  }
  tally('models', lines);
  summary.push(`models without geometry skipped: ${skipped}`);
}

// Music: streammusic/*.wav -> assets/streammusic/*.mp3 | *.wav (PCM)
if (run("music")) {
  console.log('music...');
  const src = path.join(GAME, 'streammusic'), out = path.join(A, 'streammusic');
  fs.mkdirSync(out, { recursive: true });
  const lines = [];
  for (const f of fs.readdirSync(src).filter(f => /\.wav$/i.test(f)).sort()) {
    const name = f.replace(/\.wav$/i, ''), u = unwrap(fs.readFileSync(path.join(src, f)));
    let data = u.payload, err = u.err, info = u.info;
    if (!err && u.kind === 'wav') { const d = decode(data); data = d.out; err = d.err; info += ' ' + d.info; }
    lines.push(`${name} ${u.kind} ${info} ${err ? 'FAIL: ' + err : 'PASS'}`);
    if (!err) fs.writeFileSync(path.join(out, `${name}.${u.kind}`), data);
  }
  tally('music', lines);
}

// Independent verification (audio)
let ffprobe = true;
try { execFileSync('ffprobe', ['-version'], { stdio: 'ignore' }); } catch { ffprobe = false; }
if (ffprobe && run("music")) {
  const dir = path.join(A, 'streammusic');
  const lines = fs.readdirSync(dir).map(f => {
    try { execFileSync('ffprobe', ['-v', 'error', path.join(dir, f)], { stdio: 'pipe' }); return `${f} PASS`; }
    catch (e) { return `${f} FAIL: ${String(e.stderr).trim().split('\n')[0]}`; }
  });
  tally('ffprobe', lines);
} else if (run("music")) summary.push("ffprobe missing: audio UNVERIFIED");
if (run("models")) summary.push("models UNVERIFIED (no independent loader)");

console.log(summary.join('\n'));
