// usage: node dump.js file reloffset_hex count [base]
// Prints count u32 words as hex, int, float, ascii. Offset relative to base (default 12, MDL data).
const fs = require("fs")
const [file, off, count, base = "12"] = process.argv.slice(2)
const data = fs.readFileSync(file),
	baseOffset = +base,
	start = baseOffset + parseInt(off, 16)
for (
	let pos = start;
	pos < start + 4 * count && pos + 4 <= data.length;
	pos += 4
) {
	const word = data.readUInt32LE(pos),
		float = data.readFloatLE(pos)
	const ascii = data
		.toString("latin1", pos, pos + 4)
		.replace(/[^\x20-\x7e]/g, ".")
	console.log(
		`+${(pos - baseOffset).toString(16).padStart(4, "0")} ${word.toString(16).padStart(8, "0")} ${String(word).padStart(10)} ${float.toPrecision(5).padStart(12)} ${ascii}`,
	)
}
