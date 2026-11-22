// usage: node gui2json.js assetroot
// Converts every GUI resource (type 2047) to <resref>.json. Resources in data/gui.bif go to <assetroot>/data/gui/.
// Resources in patch.erf go to <assetroot>/patch/. Five patch.erf resources replace gui.bif resources of the same name.
// A struct becomes an object keyed by field label, in file order. A struct type other than 0 is kept as "__struct_type".
// Lists become arrays of objects, vectors become 3-element arrays, f32 values use the shortest decimal that round-trips.
const fs = require("fs")
const path = require("path")
const kotor = require("./kotor")

function f32(value) {
	for (let digits = 1; digits <= 9; digits++) {
		const shortest = +value.toPrecision(digits)
		if (Math.fround(shortest) === value) return shortest
	}
	return value
}
function toJson(struct) {
	const object = {}
	if (struct.structType !== 0) object.__struct_type = struct.structType
	for (const field of struct.fields) {
		object[field.label] =
			field.type === 14
				? toJson(field.value)
				: field.type === 15
					? field.value.map(toJson)
					: field.type === 8
						? f32(field.value)
						: field.type === 17
							? field.value.map(f32)
							: field.value
	}
	return object
}

const root = process.argv[2]
if (!root) {
	console.error("usage: node gui2json.js assetroot")
	process.exit(1)
}
const sources = [
	[
		"data/gui",
		kotor
			.bifResources("data/gui.bif")
			.filter((resource) => resource.type === 2047),
	],
	[
		"patch",
		kotor
			.readErf(path.join(kotor.GAME, "patch.erf"))
			.entries.filter((entry) => entry.type === 2047)
			.map((entry) => ({ resref: entry.resref, data: entry.data() })),
	],
]
for (const [dir, list] of sources) {
	const out = path.join(root, dir)
	fs.mkdirSync(out, { recursive: true })
	for (const resource of list) {
		const gff = kotor.readGff(resource.data, resource.resref)
		if (gff.fileType !== "GUI " || gff.version !== "V3.2")
			kotor.fatal(
				`${resource.resref}: unexpected ${gff.fileType}${gff.version}`,
			)
		fs.writeFileSync(
			path.join(out, `${resource.resref}.json`),
			JSON.stringify(toJson(gff.root), null, 1) + "\n",
		)
	}
	console.log(`${list.length} GUI files written to ${out}`)
}
