// usage: node mdl2obj.js in.mdl in.mdx out.obj
// MDL + MDX -> Wavefront OBJ. Layout: see FILE_FORMATS.md. MDL offsets relative to file + 12.
const fs = require("fs")

const NONE = 0xffffffff

// Quaternions are [w, x, y, z]. qrot rotates a vector by a unit quaternion; qmul multiplies two quaternions.
function qrot([w, x, y, z], [vx, vy, vz]) {
	const tx = 2 * (y * vz - z * vy),
		ty = 2 * (z * vx - x * vz),
		tz = 2 * (x * vy - y * vx)
	return [
		vx + w * tx + y * tz - z * ty,
		vy + w * ty + z * tx - x * tz,
		vz + w * tz + x * ty - y * tx,
	]
}
function qmul([aw, ax, ay, az], [bw, bx, by, bz]) {
	return [
		aw * bw - ax * bx - ay * by - az * bz,
		aw * bx + ax * bw + ay * bz - az * by,
		aw * by - ax * bz + ay * bw + az * bx,
		aw * bz + ax * by - ay * bx + az * bw,
	]
}
const formatNumber = (value) => value.toFixed(6)

// opts.mtllib: material library file name written at the top of the OBJ.
// Meshes whose render byte (mesh header 0x139) is 0 are skipped; the game does not appear to draw them.
function convert(mdl, mdx, label = "", opts = {}) {
	const base = 12,
		len = mdl.length - base,
		errors = []
	const stats = {
		nodes: 0,
		hidden: 0,
		meshes: 0,
		verts: 0,
		faces: 0,
		mismatch: 0,
		normalsMatching: 0,
		normalsChecked: 0,
	}
	const inBounds = (offset, count, what) => {
		if (offset < 0 || offset + count > len) {
			errors.push(`${what} out of bounds (${offset}+${count}>${len})`)
			return false
		}
		return true
	}
	const u32 = (offset) =>
		inBounds(offset, 4, "u32") ? mdl.readUInt32LE(base + offset) : undefined
	const u16 = (offset) =>
		inBounds(offset, 2, "u16") ? mdl.readUInt16LE(base + offset) : undefined
	const u8 = (offset) =>
		inBounds(offset, 1, "u8") ? mdl[base + offset] : undefined
	const f32 = (offset, count) =>
		inBounds(offset, 4 * count, "f32")
			? Array.from({ length: count }, (_, i) =>
					mdl.readFloatLE(base + offset + 4 * i),
				)
			: undefined
	const readString = (offset, count) =>
		mdl
			.toString("latin1", base + offset, base + offset + count)
			.replace(/\0.*$/s, "")

	const root = u32(0x28),
		hdrNodes = u32(0x2c)
	const nameTableOffset = u32(0xb8),
		nameCount = u32(0xc0)
	const names = []
	for (let i = 0; i < nameCount; i++)
		names.push(readString(u32(nameTableOffset + 4 * i), 64))

	const out = [`# converted from ${label}`]
	if (opts.mtllib) out.push(`mtllib ${opts.mtllib}`)
	const textures = new Set()
	let vbase = 1
	const seen = new Set()

	function mesh(header, name, rotation, position) {
		const faceOffset = u32(header + 0x08),
			faceCount = u32(header + 0x0c)
		const tex = readString(header + 0x58, 32)
		const stride = u32(header + 0xfc),
			positionOffset = u32(header + 0x104),
			uvOffset = u32(header + 0x110)
		const vertexCount = u16(header + 0x130)
		const mdxoff = u32(header + 0x144),
			vertexOffset = u32(header + 0x148)
		if (vertexOffset === undefined) return
		inBounds(faceOffset, 32 * faceCount, `faces ${name}`)
		inBounds(vertexOffset, 12 * vertexCount, `verts ${name}`)
		let hasmdx = stride > 0 && vertexCount > 0 && mdx.length > 0
		if (hasmdx && mdxoff + stride * vertexCount > mdx.length) {
			errors.push(`mdx range ${name}`)
			hasmdx = false
		}
		if (u8(header + 0x139) === 0) {
			stats.hidden++
			return
		}
		const hasuv = hasmdx && uvOffset !== NONE
		stats.meshes++
		stats.verts += vertexCount
		stats.faces += faceCount
		out.push(`o ${name}`)
		if (tex && tex.toLowerCase() !== "null") {
			out.push(`usemtl ${tex.toLowerCase()}`)
			textures.add(tex.toLowerCase())
		}
		const verts = []
		for (let i = 0; i < vertexCount; i++) {
			const vertex = f32(vertexOffset + 12 * i, 3)
			verts.push(vertex)
			if (hasmdx && positionOffset !== NONE) {
				const mdxPos = mdxoff + stride * i + positionOffset
				if (
					[0, 1, 2].some(
						(axis) =>
							Math.abs(vertex[axis] - mdx.readFloatLE(mdxPos + 4 * axis)) >
							1e-5,
					)
				)
					stats.mismatch++
			}
			const world = qrot(rotation, vertex)
			out.push(
				`v ${formatNumber(world[0] + position[0])} ${formatNumber(world[1] + position[1])} ${formatNumber(world[2] + position[2])}`,
			)
			// vt for every vertex keeps v/vt indices aligned
			const uv = hasuv
				? [
						mdx.readFloatLE(mdxoff + stride * i + uvOffset),
						mdx.readFloatLE(mdxoff + stride * i + uvOffset + 4),
					]
				: [0, 0]
			out.push(`vt ${formatNumber(uv[0])} ${formatNumber(uv[1])}`)
		}
		for (let face = 0; face < faceCount; face++) {
			const faceEntry = faceOffset + 32 * face
			const ix = [u16(faceEntry + 26), u16(faceEntry + 28), u16(faceEntry + 30)]
			if (ix.some((index) => !(index < vertexCount))) {
				errors.push(`face index >= ${vertexCount} in ${name}`)
				continue
			}
			// stored normal must match winding
			const normal = f32(faceEntry, 3),
				[corner0, corner1, corner2] = ix.map((index) => verts[index])
			const edge1 = [0, 1, 2].map((axis) => corner1[axis] - corner0[axis]),
				edge2 = [0, 1, 2].map((axis) => corner2[axis] - corner0[axis])
			const cross = [
				edge1[1] * edge2[2] - edge1[2] * edge2[1],
				edge1[2] * edge2[0] - edge1[0] * edge2[2],
				edge1[0] * edge2[1] - edge1[1] * edge2[0],
			]
			const length = Math.hypot(...cross)
			if (length > 1e-9) {
				stats.normalsChecked++
				if (
					(cross[0] * normal[0] + cross[1] * normal[1] + cross[2] * normal[2]) /
						length >
					0.99
				)
					stats.normalsMatching++
			}
			out.push(
				"f " + ix.map((index) => `${index + vbase}/${index + vbase}`).join(" "),
			)
		}
		vbase += vertexCount
	}

	function node(offset, parentRotation, parentPosition, depth) {
		if (seen.has(offset)) return errors.push(`node loop at ${offset}`)
		if (depth > 200) return errors.push("depth")
		seen.add(offset)
		stats.nodes++
		const flags = u16(offset) ?? 0,
			nameIndex = u16(offset + 4) ?? 0
		const pos = f32(offset + 0x10, 3),
			rotation = f32(offset + 0x1c, 4)
		const childOffset = u32(offset + 0x2c),
			childCount = u32(offset + 0x30) ?? 0
		if (!pos || !rotation) return
		const worldRotation = qmul(parentRotation, rotation),
			rotated = qrot(parentRotation, pos),
			worldPosition = [
				rotated[0] + parentPosition[0],
				rotated[1] + parentPosition[1],
				rotated[2] + parentPosition[2],
			]
		const name = names[nameIndex] ?? `node${offset}`
		if (flags & 0x20) mesh(offset + 80, name, worldRotation, worldPosition)
		for (let i = 0; i < childCount; i++) {
			const child = u32(childOffset + 4 * i)
			if (child !== undefined)
				node(child, worldRotation, worldPosition, depth + 1)
		}
	}

	node(root, [1, 0, 0, 0], [0, 0, 0], 0)
	return {
		obj: out.join("\n") + "\n",
		stats: { ...stats, hdrNodes },
		textures,
		errors,
	}
}

function report(label, { stats, errors }) {
	return (
		`${label} nodes=${stats.nodes} meshes=${stats.meshes} hidden=${stats.hidden} verts=${stats.verts} faces=${stats.faces} ` +
		`mdl/mdx_mismatch=${stats.mismatch} hdrnodes=${stats.hdrNodes} normals_ok=${stats.normalsMatching}/${stats.normalsChecked} ` +
		(errors.length ? "FAIL: " + errors.slice(0, 5).join("; ") : "PASS")
	)
}

if (require.main === module) {
	const [fmdl, fmdx, fout] = process.argv.slice(2)
	const result = convert(
		fs.readFileSync(fmdl),
		fs.existsSync(fmdx) ? fs.readFileSync(fmdx) : Buffer.alloc(0),
		fmdl,
	)
	fs.writeFileSync(fout, result.obj)
	console.log(report(fmdl, result))
}
module.exports = { convert, report }
