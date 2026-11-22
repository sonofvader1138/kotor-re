// Shared readers: KEY V1, BIFFV1, ERF V1.0.
const fs = require("fs")
const path = require("path")
const childProcess = require("child_process")

const ROOT = path.resolve(__dirname, "..")
const GAME = path.join(ROOT, "swkotor")
const ASSETS = path.join(ROOT, "assets")
const REPORTS = path.join(ROOT, "temp", "reports")

// Resource type codes used in KEY, BIF and ERF tables, and the file extension of each.
const RESOURCE_TYPE = {
	tga: 3,
	mdl: 2002,
	txi: 2022,
	gui: 2047,
	tpc: 3007,
	mdx: 3008,
}
const EXT = {}
for (const [ext, type] of Object.entries(RESOURCE_TYPE)) {
	EXT[type] = ext
}

// Texture packs used for model materials, in priority order. tpb and tpc hold the same textures at
// lower resolution and are not converted.
const TEXTURE_PACKS = ["swpc_tex_tpa", "swpc_tex_gui"]

// Groups of BIF archives that hold TGA images, with the output folder name of each group.
const TGA_GROUPS = [
	["lightmaps", /^data\/lightmaps\d*\.bif$/],
	["textures", /^data\/textures\.bif$/],
	["legacy", /^data\/legacy\.bif$/],
]

// Order in which convert.js and verify.js print their stage summaries.
const REPORT_ORDER = [
	"models",
	"textures",
	"tga",
	"music",
	"sounds",
	"waves",
	"movies",
	"gui",
]

// A resource reference (resref) is a resource name of at most 16 bytes, padded with NUL bytes.
const RESREF_LENGTH = 16

// Signatures are 8 bytes: a 4-byte file type and a 4-byte version.
const SIGNATURE_LENGTH = 8
const KEY_SIGNATURE = "KEY V1  "
const BIF_SIGNATURE = "BIFFV1  "
const KEY_BIF_ENTRY_SIZE = 12
const KEY_ENTRY_SIZE = 22

// A resource id holds the BIF index in its top 12 bits and the resource index in its low 20 bits.
const RESOURCE_INDEX_BITS = 20
const RESOURCE_INDEX_MASK = (1 << RESOURCE_INDEX_BITS) - 1
const BIF_ENTRY_SIZE = 16

const ERF_HEADER_SIZE = 160
const ERF_KEY_ENTRY_SIZE = 24
const ERF_RESOURCE_ENTRY_SIZE = 8

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
	if (data.toString("latin1", 0, SIGNATURE_LENGTH) !== KEY_SIGNATURE) {
		fatal(`${file}: not KEY V1`)
	}

	const bifCount = data.readUInt32LE(8),
		keyCount = data.readUInt32LE(12),
		bifTable = data.readUInt32LE(16),
		keyTable = data.readUInt32LE(20)

	const bifs = []
	for (let i = 0; i < bifCount; i++) {
		const entry = bifTable + KEY_BIF_ENTRY_SIZE * i,
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
		const entry = keyTable + KEY_ENTRY_SIZE * i
		const resref = data
			.toString("latin1", entry, entry + RESREF_LENGTH)
			.replace(/\0.*$/, "")
			.toLowerCase()
		const id = data.readUInt32LE(entry + 18)
		keys.push({
			resref,
			type: data.readUInt16LE(entry + RESREF_LENGTH),
			bif: id >>> RESOURCE_INDEX_BITS,
			index: id & RESOURCE_INDEX_MASK,
		})
	}

	return { bifs, keys }
}

// BIF: 8 magic, u32 varCount, u32 fixedCount, u32 tableOff.
// Var entry 16 bytes: u32 id, u32 offset, u32 size, u32 type.
function readBif(file) {
	const buf = fs.readFileSync(file)
	if (buf.toString("latin1", 0, SIGNATURE_LENGTH) !== BIF_SIGNATURE) {
		fatal(`${file}: not BIFFV1`)
	}

	const entryCount = buf.readUInt32LE(8),
		table = buf.readUInt32LE(16)

	const entries = []
	for (let i = 0; i < entryCount; i++) {
		const entry = table + BIF_ENTRY_SIZE * i,
			offset = buf.readUInt32LE(entry + 4),
			size = buf.readUInt32LE(entry + 8)
		if (offset + size > buf.length) {
			fatal(`${file}: entry ${i} out of bounds`)
		}

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
	if (bifIndex < 0) {
		fatal(`${bifName} not in chitin.key`)
	}

	const names = new Map()
	for (const entry of key.keys) {
		if (entry.bif === bifIndex) {
			names.set(`${entry.index}/${entry.type}`, entry.resref)
		}
	}

	const { buf, entries } = readBif(path.join(GAME, bifName))
	return entries.map((entry) => ({
		resref:
			names.get(`${entry.id & RESOURCE_INDEX_MASK}/${entry.type}`) ??
			`unnamed_${entry.id & RESOURCE_INDEX_MASK}`,
		type: entry.type,
		ext: EXT[entry.type] ?? String(entry.type),
		data: buf.subarray(entry.offset, entry.offset + entry.size),
	}))
}

// ERF V1.0: 160-byte header. 0x10 u32 entryCount, 0x18 u32 keyListOff, 0x1C u32 resListOff.
// Key entry 24 bytes: char[16] resref, u32 id, u16 type, u16 unused. Resource entry 8 bytes: u32 offset, u32 size.
// Reads tables only; data() reads one resource on demand, since the packs are up to 400 MB.
function readErf(file) {
	const fd = fs.openSync(file, "r"),
		size = fs.fstatSync(fd).size,
		header = Buffer.alloc(ERF_HEADER_SIZE)
	fs.readSync(fd, header, 0, ERF_HEADER_SIZE, 0)

	const sig = header.toString("latin1", 0, SIGNATURE_LENGTH)
	if (!/^(ERF |MOD |HAK |SAV )V1\.0$/.test(sig)) {
		fatal(`${file}: not ERF V1.0`)
	}

	const entryCount = header.readUInt32LE(0x10),
		keyListOffset = header.readUInt32LE(0x18),
		resourceListOffset = header.readUInt32LE(0x1c)
	const keyList = Buffer.alloc(ERF_KEY_ENTRY_SIZE * entryCount),
		resourceList = Buffer.alloc(ERF_RESOURCE_ENTRY_SIZE * entryCount)
	fs.readSync(fd, keyList, 0, keyList.length, keyListOffset)
	fs.readSync(fd, resourceList, 0, resourceList.length, resourceListOffset)

	const entries = []
	for (let i = 0; i < entryCount; i++) {
		const resourceEntry = ERF_RESOURCE_ENTRY_SIZE * i,
			keyEntry = ERF_KEY_ENTRY_SIZE * i,
			offset = resourceList.readUInt32LE(resourceEntry),
			len = resourceList.readUInt32LE(resourceEntry + 4)
		if (offset + len > size) {
			fatal(`${file}: entry ${i} out of bounds`)
		}

		const type = keyList.readUInt16LE(keyEntry + 20)
		entries.push({
			resref: keyList
				.toString("latin1", keyEntry, keyEntry + RESREF_LENGTH)
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

// Generic tree file ("GUI V3.2" and possibly others). 56-byte header: char[4] fileType, char[4] version,
// then six u32 pairs: structs (off, count), fields (off, count), labels (off, count),
// fieldData (off, bytes), fieldIndices (off, bytes), listIndices (off, bytes).
// Struct 12 bytes: u32 type, u32 data, u32 fieldCount. Field 12 bytes: u32 type, u32 labelIndex, u32 value.
// Strict: every byte of the three data regions, every field and every non-root struct must be used exactly once.
const GFF_HEADER_SIZE = 56
const GFF_STRUCT_SIZE = 12
const GFF_FIELD_SIZE = 12
const GFF_LABEL_SIZE = 16

// Field type codes. Types 0, 4, 5 and 8 store their value in the field itself; the others point into
// the field data, the struct table or the list indices. Other codes have not been seen.
const GFF_TYPE = {
	u8: 0,
	u32: 4,
	i32: 5,
	f32: 8,
	string: 10,
	resref: 11,
	struct: 14,
	list: 15,
	vector: 17,
}

function readGff(data, name = "gff") {
	const fail = (message) => {
		fatal(`${name}: ${message}`)
	}

	if (data.length < GFF_HEADER_SIZE) {
		fail("short header")
	}

	const header = []
	for (let i = SIGNATURE_LENGTH; i < GFF_HEADER_SIZE; i += 4) {
		header.push(data.readUInt32LE(i))
	}

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
		structOffset !== GFF_HEADER_SIZE ||
		fieldOffset !== structOffset + GFF_STRUCT_SIZE * structCount ||
		labelOffset !== fieldOffset + GFF_FIELD_SIZE * fieldCount ||
		fieldDataOffset !== labelOffset + GFF_LABEL_SIZE * labelCount ||
		fieldIndicesOffset !== fieldDataOffset + fieldDataLength ||
		listIndicesOffset !== fieldIndicesOffset + fieldIndicesLength ||
		listIndicesOffset + listIndicesLength !== data.length
	) {
		fail("header regions do not chain to end of file")
	}

	const use = {
		data: new Uint8Array(fieldDataLength),
		idx: new Uint8Array(fieldIndicesLength),
		list: new Uint8Array(listIndicesLength),
	}
	const mark = (region, offset, count, len) => {
		if (offset + count > len) {
			fail(`${region} range ${offset}+${count} past ${len}`)
		}

		for (let i = offset; i < offset + count; i++) {
			if (use[region][i]++) {
				fail(`${region} byte ${i} used twice`)
			}
		}
	}

	const fieldUsed = new Uint8Array(fieldCount),
		structUsed = new Uint8Array(structCount)

	const labels = []
	for (let i = 0; i < labelCount; i++) {
		const label = labelOffset + GFF_LABEL_SIZE * i
		labels.push(
			data
				.toString("latin1", label, label + GFF_LABEL_SIZE)
				.replace(/\0.*$/s, ""),
		)
	}
	const labelUsed = new Uint8Array(labelCount),
		types = {}

	function field(fieldIndex) {
		if (fieldIndex >= fieldCount) {
			fail(`field index ${fieldIndex} >= ${fieldCount}`)
		}
		if (fieldUsed[fieldIndex]++) {
			fail(`field ${fieldIndex} used twice`)
		}

		const entry = fieldOffset + GFF_FIELD_SIZE * fieldIndex,
			type = data.readUInt32LE(entry),
			labelIndex = data.readUInt32LE(entry + 4),
			raw = data.readUInt32LE(entry + 8)
		if (labelIndex >= labelCount) {
			fail(`label index ${labelIndex} >= ${labelCount}`)
		}

		labelUsed[labelIndex] = 1
		types[type] = (types[type] || 0) + 1

		let value
		switch (type) {
			case GFF_TYPE.u8:
				if (raw > 0xff) {
					fail(`u8 field ${fieldIndex} value ${raw}`)
				}
				value = raw
				break
			case GFF_TYPE.u32:
				value = raw
				break
			case GFF_TYPE.i32:
				value = data.readInt32LE(entry + 8)
				break
			case GFF_TYPE.f32:
				value = data.readFloatLE(entry + 8)
				break
			case GFF_TYPE.string: {
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
			case GFF_TYPE.resref: {
				mark("data", raw, 1, fieldDataLength)
				const len = data[fieldDataOffset + raw]
				if (len > RESREF_LENGTH) {
					fail(`resref length ${len}`)
				}
				mark("data", raw + 1, len, fieldDataLength)
				value = data.toString(
					"latin1",
					fieldDataOffset + raw + 1,
					fieldDataOffset + raw + 1 + len,
				)
				break
			}
			case GFF_TYPE.struct:
				value = struct(raw)
				break
			case GFF_TYPE.list: {
				mark("list", raw, 4, listIndicesLength)
				const count = data.readUInt32LE(listIndicesOffset + raw)
				mark("list", raw + 4, 4 * count, listIndicesLength)
				value = []
				for (let i = 0; i < count; i++) {
					value.push(
						struct(data.readUInt32LE(listIndicesOffset + raw + 4 + 4 * i)),
					)
				}
				break
			}
			case GFF_TYPE.vector:
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
		if (structIndex >= structCount) {
			fail(`struct index ${structIndex} >= ${structCount}`)
		}
		if (structUsed[structIndex]++) {
			fail(`struct ${structIndex} used twice`)
		}

		const entry = structOffset + GFF_STRUCT_SIZE * structIndex,
			type = data.readUInt32LE(entry),
			first = data.readUInt32LE(entry + 4),
			count = data.readUInt32LE(entry + 8)

		let ids = []
		if (count === 1) {
			ids = [first]
		} else if (count > 1) {
			mark("idx", first, 4 * count, fieldIndicesLength)
			for (let i = 0; i < count; i++) {
				ids.push(data.readUInt32LE(fieldIndicesOffset + first + 4 * i))
			}
		}

		return { structType: type, fields: ids.map(field) }
	}

	const root = struct(0, true)

	for (const [region, used] of Object.entries(use)) {
		const unused = used.indexOf(0)
		if (unused >= 0) {
			fail(`${region} byte ${unused} never used`)
		}
	}
	if (fieldUsed.indexOf(0) >= 0) {
		fail(`field ${fieldUsed.indexOf(0)} never used`)
	}
	if (structUsed.indexOf(0) >= 0) {
		fail(`struct ${structUsed.indexOf(0)} never used`)
	}

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

// Runs a program without blocking and resolves to { status, stdout (Buffer), stderr (string), error }.
// A missing program resolves with error set, like childProcess.spawnSync.
function exec(program, args, input) {
	return new Promise((resolve) => {
		const child = childProcess.spawn(program, args)
		const stdout = [],
			stderr = []
		child.stdout.on("data", (chunk) => stdout.push(chunk))
		child.stderr.on("data", (chunk) => stderr.push(chunk))

		// A program that exits before reading all of its input closes the pipe. The exit status reports that.
		child.stdin.on("error", () => {})
		child.stdin.end(input)

		const finish = (status, error) =>
			resolve({
				status,
				error,
				stdout: Buffer.concat(stdout),
				stderr: Buffer.concat(stderr).toString(),
			})
		child.on("error", (error) => finish(null, error))
		child.on("close", (status) => finish(status))
	})
}

module.exports = {
	ROOT,
	GAME,
	ASSETS,
	REPORTS,
	RESOURCE_TYPE,
	EXT,
	TEXTURE_PACKS,
	TGA_GROUPS,
	REPORT_ORDER,
	GFF_TYPE,
	fatal,
	readKey,
	readBif,
	bifResources,
	readErf,
	readGff,
	exec,
}
