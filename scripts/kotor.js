// Shared readers: KEY V1, BIFFV1, ERF V1.0.
const fs = require("fs")
const path = require("path")

const ROOT = path.resolve(__dirname, "..")
const GAME = path.join(ROOT, "swkotor")
const EXT = { 3: "tga", 2002: "mdl", 2022: "txi", 3007: "tpc", 3008: "mdx" }

// Prints an error message and ends the process with exit status 1. Scripts use this for errors they cannot recover from.
function fatal(message) {
	console.error(message)
	process.exit(1)
}

// KEY: header 8 magic, u32 bifCount, keyCount, bifTableOff, keyTableOff.
// Bif entry 12 bytes: u32 size, u32 nameOff, u16 nameLen, u16 drives.
// Key entry 22 bytes: char[16] resref, u16 type, u32 id (bif << 20 | index).
function readKey(file = path.join(GAME, "chitin.key")) {
	const b = fs.readFileSync(file)
	if (b.toString("latin1", 0, 8) !== "KEY V1  ") fatal(`${file}: not KEY V1`)
	const nb = b.readUInt32LE(8),
		nk = b.readUInt32LE(12),
		ob = b.readUInt32LE(16),
		ok = b.readUInt32LE(20)
	const bifs = []
	for (let i = 0; i < nb; i++) {
		const o = ob + 12 * i,
			no = b.readUInt32LE(o + 4),
			nl = b.readUInt16LE(o + 8)
		bifs.push(
			b
				.toString("latin1", no, no + nl)
				.replace(/\0.*$/, "")
				.replace(/\\/g, "/"),
		)
	}
	const keys = []
	for (let i = 0; i < nk; i++) {
		const o = ok + 22 * i
		const resref = b
			.toString("latin1", o, o + 16)
			.replace(/\0.*$/, "")
			.toLowerCase()
		const id = b.readUInt32LE(o + 18)
		keys.push({
			resref,
			type: b.readUInt16LE(o + 16),
			bif: id >>> 20,
			index: id & 0xfffff,
		})
	}
	return { bifs, keys }
}

// BIF: 8 magic, u32 varCount, u32 fixedCount, u32 tableOff.
// Var entry 16 bytes: u32 id, u32 offset, u32 size, u32 type.
function readBif(file) {
	const buf = fs.readFileSync(file)
	if (buf.toString("latin1", 0, 8) !== "BIFFV1  ") fatal(`${file}: not BIFFV1`)
	const nv = buf.readUInt32LE(8),
		to = buf.readUInt32LE(16)
	const entries = []
	for (let i = 0; i < nv; i++) {
		const o = to + 16 * i,
			offset = buf.readUInt32LE(o + 4),
			size = buf.readUInt32LE(o + 8)
		if (offset + size > buf.length) fatal(`${file}: entry ${i} out of bounds`)
		entries.push({
			id: buf.readUInt32LE(o),
			offset,
			size,
			type: buf.readUInt32LE(o + 12),
		})
	}
	return { buf, entries }
}

// Resources of one BIF, named via KEY: [{resref, type, ext, data}]
function bifResources(bifName, key = readKey()) {
	const bi = key.bifs.indexOf(bifName)
	if (bi < 0) fatal(`${bifName} not in chitin.key`)
	const names = new Map()
	for (const k of key.keys)
		if (k.bif === bi) names.set(`${k.index}/${k.type}`, k.resref)
	const { buf, entries } = readBif(path.join(GAME, bifName))
	return entries.map((e) => ({
		resref:
			names.get(`${e.id & 0xfffff}/${e.type}`) ?? `unnamed_${e.id & 0xfffff}`,
		type: e.type,
		ext: EXT[e.type] ?? String(e.type),
		data: buf.subarray(e.offset, e.offset + e.size),
	}))
}

module.exports = { ROOT, GAME, EXT, fatal, readKey, readBif, bifResources }

// ERF V1.0: 160-byte header. 0x10 u32 entryCount, 0x18 u32 keyListOff, 0x1C u32 resListOff.
// Key entry 24 bytes: char[16] resref, u32 id, u16 type, u16 unused. Resource entry 8 bytes: u32 offset, u32 size.
// Reads tables only; data() reads one resource on demand, since the packs are up to 400 MB.
function readErf(file) {
	const fd = fs.openSync(file, "r"),
		size = fs.fstatSync(fd).size,
		h = Buffer.alloc(160)
	fs.readSync(fd, h, 0, 160, 0)
	const sig = h.toString("latin1", 0, 8)
	if (!/^(ERF |MOD |HAK |SAV )V1\.0$/.test(sig)) fatal(`${file}: not ERF V1.0`)
	const n = h.readUInt32LE(0x10),
		ko = h.readUInt32LE(0x18),
		ro = h.readUInt32LE(0x1c)
	const k = Buffer.alloc(24 * n),
		r = Buffer.alloc(8 * n)
	fs.readSync(fd, k, 0, k.length, ko)
	fs.readSync(fd, r, 0, r.length, ro)
	const entries = []
	for (let i = 0; i < n; i++) {
		const offset = r.readUInt32LE(8 * i),
			len = r.readUInt32LE(8 * i + 4)
		if (offset + len > size) fatal(`${file}: entry ${i} out of bounds`)
		const type = k.readUInt16LE(24 * i + 20)
		entries.push({
			resref: k
				.toString("latin1", 24 * i, 24 * i + 16)
				.replace(/\0.*$/s, "")
				.toLowerCase(),
			type,
			ext: EXT[type] ?? String(type),
			offset,
			size: len,
			data() {
				const b = Buffer.alloc(len)
				fs.readSync(fd, b, 0, len, offset)
				return b
			},
		})
	}
	return { sig, entries, size }
}

module.exports.readErf = readErf

// Generic tree file ("GUI V3.2" and possibly others). 56-byte header: char[4] fileType, char[4] version,
// then six u32 pairs: structs (off, count), fields (off, count), labels (off, count),
// fieldData (off, bytes), fieldIndices (off, bytes), listIndices (off, bytes).
// Struct 12 bytes: u32 type, u32 data, u32 fieldCount. Field 12 bytes: u32 type, u32 labelIndex, u32 value.
// Strict: every byte of the three data regions, every field and every non-root struct must be used exactly once.
const GFF_INLINE = { 0: "u8", 4: "u32", 5: "i32", 8: "f32" }
function readGff(b, name = "gff") {
	const fail = (m) => {
		fatal(`${name}: ${m}`)
	}
	if (b.length < 56) fail("short header")
	const h = []
	for (let i = 8; i < 56; i += 4) h.push(b.readUInt32LE(i))
	const [so, sc, fo, fc, lo, lc, dO, dL, iO, iL, xO, xL] = h
	if (
		so !== 56 ||
		fo !== so + 12 * sc ||
		lo !== fo + 12 * fc ||
		dO !== lo + 16 * lc ||
		iO !== dO + dL ||
		xO !== iO + iL ||
		xO + xL !== b.length
	)
		fail("header regions do not chain to end of file")
	const use = {
		data: new Uint8Array(dL),
		idx: new Uint8Array(iL),
		list: new Uint8Array(xL),
	}
	const mark = (k, off, n, len) => {
		if (off + n > len) fail(`${k} range ${off}+${n} past ${len}`)
		for (let i = off; i < off + n; i++)
			if (use[k][i]++) fail(`${k} byte ${i} used twice`)
	}
	const fieldUsed = new Uint8Array(fc),
		structUsed = new Uint8Array(sc)
	const labels = []
	for (let i = 0; i < lc; i++)
		labels.push(
			b.toString("latin1", lo + 16 * i, lo + 16 * i + 16).replace(/\0.*$/s, ""),
		)
	const labelUsed = new Uint8Array(lc),
		types = {}

	function field(fi) {
		if (fi >= fc) fail(`field index ${fi} >= ${fc}`)
		if (fieldUsed[fi]++) fail(`field ${fi} used twice`)
		const o = fo + 12 * fi,
			t = b.readUInt32LE(o),
			li = b.readUInt32LE(o + 4),
			v = b.readUInt32LE(o + 8)
		if (li >= lc) fail(`label index ${li} >= ${lc}`)
		labelUsed[li] = 1
		types[t] = (types[t] || 0) + 1
		let value
		switch (t) {
			case 0:
				if (v > 0xff) fail(`u8 field ${fi} value ${v}`)
				value = v
				break
			case 4:
				value = v
				break
			case 5:
				value = b.readInt32LE(o + 8)
				break
			case 8:
				value = b.readFloatLE(o + 8)
				break
			case 10: {
				mark("data", v, 4, dL)
				const n = b.readUInt32LE(dO + v)
				mark("data", v + 4, n, dL)
				value = b.toString("latin1", dO + v + 4, dO + v + 4 + n)
				break
			}
			case 11: {
				mark("data", v, 1, dL)
				const n = b[dO + v]
				if (n > 16) fail(`resref length ${n}`)
				mark("data", v + 1, n, dL)
				value = b.toString("latin1", dO + v + 1, dO + v + 1 + n)
				break
			}
			case 14:
				value = struct(v)
				break
			case 15: {
				mark("list", v, 4, xL)
				const n = b.readUInt32LE(xO + v)
				mark("list", v + 4, 4 * n, xL)
				value = []
				for (let k = 0; k < n; k++)
					value.push(struct(b.readUInt32LE(xO + v + 4 + 4 * k)))
				break
			}
			case 17:
				mark("data", v, 12, dL)
				value = [0, 4, 8].map((k) => b.readFloatLE(dO + v + k))
				break
			default:
				fail(`unknown field type ${t} (field ${fi})`)
		}
		return { label: labels[li], type: t, value }
	}
	function struct(si, root = false) {
		if (si >= sc) fail(`struct index ${si} >= ${sc}`)
		if (structUsed[si]++) fail(`struct ${si} used twice`)
		const o = so + 12 * si,
			type = b.readUInt32LE(o),
			d = b.readUInt32LE(o + 4),
			n = b.readUInt32LE(o + 8)
		let ids = []
		if (n === 1) ids = [d]
		else if (n > 1) {
			mark("idx", d, 4 * n, iL)
			for (let k = 0; k < n; k++) ids.push(b.readUInt32LE(iO + d + 4 * k))
		}
		return { structType: type, fields: ids.map(field) }
	}
	const root = struct(0, true)
	for (const [k, a] of Object.entries(use)) {
		const i = a.indexOf(0)
		if (i >= 0) fail(`${k} byte ${i} never used`)
	}
	if (fieldUsed.indexOf(0) >= 0)
		fail(`field ${fieldUsed.indexOf(0)} never used`)
	if (structUsed.indexOf(0) >= 0)
		fail(`struct ${structUsed.indexOf(0)} never used`)
	return {
		fileType: b.toString("latin1", 0, 4),
		version: b.toString("latin1", 4, 8),
		root,
		counts: {
			structs: sc,
			fields: fc,
			labels: lc,
			labelsUnused: labelUsed.length - labelUsed.reduce((a, x) => a + x, 0),
			types,
		},
	}
}

module.exports.readGff = readGff
