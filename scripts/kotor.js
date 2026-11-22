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
	const data = fs.readFileSync(file)
	if (data.toString("latin1", 0, 8) !== "KEY V1  ") fatal(`${file}: not KEY V1`)
	const bifCount = data.readUInt32LE(8),
		keyCount = data.readUInt32LE(12),
		bifTable = data.readUInt32LE(16),
		keyTable = data.readUInt32LE(20)
	const bifs = []
	for (let i = 0; i < bifCount; i++) {
		const entry = bifTable + 12 * i,
			nameOffset = data.readUInt32LE(entry + 4),
			nameLength = data.readUInt16LE(entry + 8)
		bifs.push(
			data
				.toString("latin1", nameOffset, nameOffset + nameLength)
				.replace(/\0.*$/, "")
				.replace(/\\/g, "/"),
		)
	}
	const keys = []
	for (let i = 0; i < keyCount; i++) {
		const entry = keyTable + 22 * i
		const resref = data
			.toString("latin1", entry, entry + 16)
			.replace(/\0.*$/, "")
			.toLowerCase()
		const id = data.readUInt32LE(entry + 18)
		keys.push({
			resref,
			type: data.readUInt16LE(entry + 16),
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
	const entryCount = buf.readUInt32LE(8),
		table = buf.readUInt32LE(16)
	const entries = []
	for (let i = 0; i < entryCount; i++) {
		const entry = table + 16 * i,
			offset = buf.readUInt32LE(entry + 4),
			size = buf.readUInt32LE(entry + 8)
		if (offset + size > buf.length) fatal(`${file}: entry ${i} out of bounds`)
		entries.push({
			id: buf.readUInt32LE(entry),
			offset,
			size,
			type: buf.readUInt32LE(entry + 12),
		})
	}
	return { buf, entries }
}

// Resources of one BIF, named via KEY: [{resref, type, ext, data}]
function bifResources(bifName, key = readKey()) {
	const bifIndex = key.bifs.indexOf(bifName)
	if (bifIndex < 0) fatal(`${bifName} not in chitin.key`)
	const names = new Map()
	for (const entry of key.keys)
		if (entry.bif === bifIndex)
			names.set(`${entry.index}/${entry.type}`, entry.resref)
	const { buf, entries } = readBif(path.join(GAME, bifName))
	return entries.map((entry) => ({
		resref:
			names.get(`${entry.id & 0xfffff}/${entry.type}`) ??
			`unnamed_${entry.id & 0xfffff}`,
		type: entry.type,
		ext: EXT[entry.type] ?? String(entry.type),
		data: buf.subarray(entry.offset, entry.offset + entry.size),
	}))
}

module.exports = { ROOT, GAME, EXT, fatal, readKey, readBif, bifResources }

// ERF V1.0: 160-byte header. 0x10 u32 entryCount, 0x18 u32 keyListOff, 0x1C u32 resListOff.
// Key entry 24 bytes: char[16] resref, u32 id, u16 type, u16 unused. Resource entry 8 bytes: u32 offset, u32 size.
// Reads tables only; data() reads one resource on demand, since the packs are up to 400 MB.
function readErf(file) {
	const fd = fs.openSync(file, "r"),
		size = fs.fstatSync(fd).size,
		header = Buffer.alloc(160)
	fs.readSync(fd, header, 0, 160, 0)
	const sig = header.toString("latin1", 0, 8)
	if (!/^(ERF |MOD |HAK |SAV )V1\.0$/.test(sig)) fatal(`${file}: not ERF V1.0`)
	const entryCount = header.readUInt32LE(0x10),
		keyListOffset = header.readUInt32LE(0x18),
		resourceListOffset = header.readUInt32LE(0x1c)
	const keyList = Buffer.alloc(24 * entryCount),
		resourceList = Buffer.alloc(8 * entryCount)
	fs.readSync(fd, keyList, 0, keyList.length, keyListOffset)
	fs.readSync(fd, resourceList, 0, resourceList.length, resourceListOffset)
	const entries = []
	for (let i = 0; i < entryCount; i++) {
		const offset = resourceList.readUInt32LE(8 * i),
			len = resourceList.readUInt32LE(8 * i + 4)
		if (offset + len > size) fatal(`${file}: entry ${i} out of bounds`)
		const type = keyList.readUInt16LE(24 * i + 20)
		entries.push({
			resref: keyList
				.toString("latin1", 24 * i, 24 * i + 16)
				.replace(/\0.*$/s, "")
				.toLowerCase(),
			type,
			ext: EXT[type] ?? String(type),
			offset,
			size: len,
			data() {
				const data = Buffer.alloc(len)
				fs.readSync(fd, data, 0, len, offset)
				return data
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
function readGff(data, name = "gff") {
	const fail = (message) => {
		fatal(`${name}: ${message}`)
	}
	if (data.length < 56) fail("short header")
	const header = []
	for (let i = 8; i < 56; i += 4) header.push(data.readUInt32LE(i))
	const [
		structOffset,
		structCount,
		fieldOffset,
		fieldCount,
		labelOffset,
		labelCount,
		fieldDataOffset,
		fieldDataLength,
		fieldIndicesOffset,
		fieldIndicesLength,
		listIndicesOffset,
		listIndicesLength,
	] = header
	if (
		structOffset !== 56 ||
		fieldOffset !== structOffset + 12 * structCount ||
		labelOffset !== fieldOffset + 12 * fieldCount ||
		fieldDataOffset !== labelOffset + 16 * labelCount ||
		fieldIndicesOffset !== fieldDataOffset + fieldDataLength ||
		listIndicesOffset !== fieldIndicesOffset + fieldIndicesLength ||
		listIndicesOffset + listIndicesLength !== data.length
	)
		fail("header regions do not chain to end of file")
	const use = {
		data: new Uint8Array(fieldDataLength),
		idx: new Uint8Array(fieldIndicesLength),
		list: new Uint8Array(listIndicesLength),
	}
	const mark = (region, offset, count, len) => {
		if (offset + count > len)
			fail(`${region} range ${offset}+${count} past ${len}`)
		for (let i = offset; i < offset + count; i++)
			if (use[region][i]++) fail(`${region} byte ${i} used twice`)
	}
	const fieldUsed = new Uint8Array(fieldCount),
		structUsed = new Uint8Array(structCount)
	const labels = []
	for (let i = 0; i < labelCount; i++)
		labels.push(
			data
				.toString("latin1", labelOffset + 16 * i, labelOffset + 16 * i + 16)
				.replace(/\0.*$/s, ""),
		)
	const labelUsed = new Uint8Array(labelCount),
		types = {}

	function field(fieldIndex) {
		if (fieldIndex >= fieldCount)
			fail(`field index ${fieldIndex} >= ${fieldCount}`)
		if (fieldUsed[fieldIndex]++) fail(`field ${fieldIndex} used twice`)
		const entry = fieldOffset + 12 * fieldIndex,
			type = data.readUInt32LE(entry),
			labelIndex = data.readUInt32LE(entry + 4),
			raw = data.readUInt32LE(entry + 8)
		if (labelIndex >= labelCount)
			fail(`label index ${labelIndex} >= ${labelCount}`)
		labelUsed[labelIndex] = 1
		types[type] = (types[type] || 0) + 1
		let value
		switch (type) {
			case 0:
				if (raw > 0xff) fail(`u8 field ${fieldIndex} value ${raw}`)
				value = raw
				break
			case 4:
				value = raw
				break
			case 5:
				value = data.readInt32LE(entry + 8)
				break
			case 8:
				value = data.readFloatLE(entry + 8)
				break
			case 10: {
				mark("data", raw, 4, fieldDataLength)
				const len = data.readUInt32LE(fieldDataOffset + raw)
				mark("data", raw + 4, len, fieldDataLength)
				value = data.toString(
					"latin1",
					fieldDataOffset + raw + 4,
					fieldDataOffset + raw + 4 + len,
				)
				break
			}
			case 11: {
				mark("data", raw, 1, fieldDataLength)
				const len = data[fieldDataOffset + raw]
				if (len > 16) fail(`resref length ${len}`)
				mark("data", raw + 1, len, fieldDataLength)
				value = data.toString(
					"latin1",
					fieldDataOffset + raw + 1,
					fieldDataOffset + raw + 1 + len,
				)
				break
			}
			case 14:
				value = struct(raw)
				break
			case 15: {
				mark("list", raw, 4, listIndicesLength)
				const count = data.readUInt32LE(listIndicesOffset + raw)
				mark("list", raw + 4, 4 * count, listIndicesLength)
				value = []
				for (let i = 0; i < count; i++)
					value.push(
						struct(data.readUInt32LE(listIndicesOffset + raw + 4 + 4 * i)),
					)
				break
			}
			case 17:
				mark("data", raw, 12, fieldDataLength)
				value = [0, 4, 8].map((component) =>
					data.readFloatLE(fieldDataOffset + raw + component),
				)
				break
			default:
				fail(`unknown field type ${type} (field ${fieldIndex})`)
		}
		return { label: labels[labelIndex], type, value }
	}
	function struct(structIndex, root = false) {
		if (structIndex >= structCount)
			fail(`struct index ${structIndex} >= ${structCount}`)
		if (structUsed[structIndex]++) fail(`struct ${structIndex} used twice`)
		const entry = structOffset + 12 * structIndex,
			type = data.readUInt32LE(entry),
			first = data.readUInt32LE(entry + 4),
			count = data.readUInt32LE(entry + 8)
		let ids = []
		if (count === 1) ids = [first]
		else if (count > 1) {
			mark("idx", first, 4 * count, fieldIndicesLength)
			for (let i = 0; i < count; i++)
				ids.push(data.readUInt32LE(fieldIndicesOffset + first + 4 * i))
		}
		return { structType: type, fields: ids.map(field) }
	}
	const root = struct(0, true)
	for (const [region, used] of Object.entries(use)) {
		const unused = used.indexOf(0)
		if (unused >= 0) fail(`${region} byte ${unused} never used`)
	}
	if (fieldUsed.indexOf(0) >= 0)
		fail(`field ${fieldUsed.indexOf(0)} never used`)
	if (structUsed.indexOf(0) >= 0)
		fail(`struct ${structUsed.indexOf(0)} never used`)
	return {
		fileType: data.toString("latin1", 0, 4),
		version: data.toString("latin1", 4, 8),
		root,
		counts: {
			structs: structCount,
			fields: fieldCount,
			labels: labelCount,
			labelsUnused:
				labelUsed.length - labelUsed.reduce((sum, used) => sum + used, 0),
			types,
		},
	}
}

module.exports.readGff = readGff
