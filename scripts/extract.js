// usage: node extract.js data/models.bif outdir [resref...]
// Writes <resref>.<ext> for matching resources (all if none given).
const fs = require("fs")
const path = require("path")
const kotor = require("./kotor")

const [bif, out, ...want] = process.argv.slice(2)
const wanted = new Set(want.map((resref) => resref.toLowerCase()))
fs.mkdirSync(out, { recursive: true })

let count = 0
for (const resource of kotor.bifResources(bif)) {
	if (wanted.size && !wanted.has(resource.resref)) {
		continue
	}

	fs.writeFileSync(
		path.join(out, `${resource.resref}.${resource.ext}`),
		resource.data,
	)
	count++
}

console.log(`${count} resources written to ${out}`)
