// usage: node render-obj.js in.obj out.png [size]
// Software render, 4 orthographic views (front, back, left, right; Z up), z-buffer, back-face culling.
// Faces whose material has a map_Kd PNG are textured, using the OBJ convention that v = 0 is the
// bottom row of the image. Other faces are gray. Both are shaded by the face normal.
// Texels with alpha below 128 are not drawn. Culled faces are not drawn, like a typical viewer.
const fs = require("fs")
const path = require("path")
const png = require("./png")
const [inp, out, size = "400"] = process.argv.slice(2)
const viewSize = +size

const positions = [],
	texCoords = [],
	triangles = [],
	mats = new Map()
let cur = null
const dir = path.dirname(inp)
for (const line of fs.readFileSync(inp, "utf8").split("\n")) {
	const tokens = line.trim().split(/\s+/)
	if (tokens[0] === "v") positions.push(tokens.slice(1, 4).map(Number))
	else if (tokens[0] === "vt") texCoords.push(tokens.slice(1, 3).map(Number))
	else if (tokens[0] === "usemtl") cur = tokens[1]
	else if (tokens[0] === "mtllib") {
		const mtlFile = path.join(dir, tokens.slice(1).join(" "))
		if (!fs.existsSync(mtlFile)) continue
		let material = null
		for (const mtlLine of fs.readFileSync(mtlFile, "utf8").split("\n")) {
			const mtlTokens = mtlLine.trim().split(/\s+/)
			if (mtlTokens[0] === "newmtl") material = mtlTokens[1]
			else if (mtlTokens[0] === "map_Kd" && material) {
				const texFile = path.join(
					path.dirname(mtlFile),
					mtlTokens.slice(1).join(" "),
				)
				if (fs.existsSync(texFile))
					mats.set(material, png.decode(fs.readFileSync(texFile)))
			}
		}
	} else if (tokens[0] === "f") {
		const ix = tokens
			.slice(1)
			.map((corner) => corner.split("/").map((index) => +index - 1))
		for (let i = 1; i + 1 < ix.length; i++)
			triangles.push({
				v: [ix[0][0], ix[i][0], ix[i + 1][0]],
				vt: [ix[0][1], ix[i][1], ix[i + 1][1]],
				material: cur,
			})
	}
}
const lo = [0, 1, 2].map((axis) =>
		Math.min(...positions.map((pos) => pos[axis])),
	),
	hi = [0, 1, 2].map((axis) => Math.max(...positions.map((pos) => pos[axis])))
const center = [0, 1, 2].map((axis) => (lo[axis] + hi[axis]) / 2),
	radius = Math.max(...[0, 1, 2].map((axis) => hi[axis] - lo[axis])) / 2 || 1

// Each view gives the screen's right axis and the direction toward the camera (fwd). Screen up is +Z.
const views = [
	{ right: [1, 0, 0], fwd: [0, -1, 0] },
	{ right: [-1, 0, 0], fwd: [0, 1, 0] },
	{ right: [0, -1, 0], fwd: [-1, 0, 0] },
	{ right: [0, 1, 0], fwd: [1, 0, 0] },
]
const imageWidth = viewSize * 4,
	imageHeight = viewSize,
	img = Buffer.alloc(imageWidth * imageHeight * 3, 40)
const dot = (lhs, rhs) => lhs[0] * rhs[0] + lhs[1] * rhs[1] + lhs[2] * rhs[2]
const fract = (x) => x - Math.floor(x)

views.forEach((view, viewIndex) => {
	const depth = new Float64Array(viewSize * viewSize).fill(-Infinity),
		up = [0, 0, 1]
	const screen = positions.map((pos) => {
		const rel = [0, 1, 2].map((axis) => (pos[axis] - center[axis]) / radius)
		return [
			viewSize / 2 + dot(rel, view.right) * viewSize * 0.45,
			viewSize / 2 - dot(rel, up) * viewSize * 0.45,
			dot(rel, view.fwd),
		]
	})
	for (const {
		v: [i0, i1, i2],
		vt: uvIndices,
		material,
	} of triangles) {
		const corner0 = positions[i0],
			corner1 = positions[i1],
			corner2 = positions[i2]
		const edge1 = [0, 1, 2].map((axis) => corner1[axis] - corner0[axis]),
			edge2 = [0, 1, 2].map((axis) => corner2[axis] - corner0[axis])
		const normal = [
			edge1[1] * edge2[2] - edge1[2] * edge2[1],
			edge1[2] * edge2[0] - edge1[0] * edge2[2],
			edge1[0] * edge2[1] - edge1[1] * edge2[0],
		]
		const ln = Math.hypot(...normal)
		if (!ln) continue
		const facing = dot(normal, view.fwd) / ln
		if (facing <= 0) continue // back-face cull (CCW = front)
		const shade = 0.25 + 0.75 * facing
		const tex = mats.get(material),
			uv =
				tex && uvIndices.every((index) => texCoords[index])
					? uvIndices.map((index) => texCoords[index])
					: null
		const [s0, s1, s2] = [screen[i0], screen[i1], screen[i2]]
		const x0 = Math.max(0, Math.floor(Math.min(s0[0], s1[0], s2[0]))),
			x1 = Math.min(viewSize - 1, Math.ceil(Math.max(s0[0], s1[0], s2[0])))
		const y0 = Math.max(0, Math.floor(Math.min(s0[1], s1[1], s2[1]))),
			y1 = Math.min(viewSize - 1, Math.ceil(Math.max(s0[1], s1[1], s2[1])))
		const area =
			(s1[0] - s0[0]) * (s2[1] - s0[1]) - (s1[1] - s0[1]) * (s2[0] - s0[0])
		if (!area) continue
		for (let y = y0; y <= y1; y++)
			for (let x = x0; x <= x1; x++) {
				const w0 =
					((s1[0] - x) * (s2[1] - y) - (s1[1] - y) * (s2[0] - x)) / area
				const w1 =
						((s2[0] - x) * (s0[1] - y) - (s2[1] - y) * (s0[0] - x)) / area,
					w2 = 1 - w0 - w1
				if (w0 < 0 || w1 < 0 || w2 < 0) continue
				const pixelDepth = w0 * s0[2] + w1 * s1[2] + w2 * s2[2]
				if (pixelDepth <= depth[y * viewSize + x]) continue
				let col = [200, 200, 200]
				if (uv) {
					const tu = w0 * uv[0][0] + w1 * uv[1][0] + w2 * uv[2][0],
						tv = w0 * uv[0][1] + w1 * uv[1][1] + w2 * uv[2][1]
					const tx = Math.min(tex.width - 1, Math.floor(fract(tu) * tex.width)),
						ty = Math.min(
							tex.height - 1,
							Math.floor((1 - fract(tv)) * tex.height),
						)
					const texel = 4 * (ty * tex.width + tx)
					if (tex.rgba[texel + 3] < 128) continue
					col = [tex.rgba[texel], tex.rgba[texel + 1], tex.rgba[texel + 2]]
				}
				depth[y * viewSize + x] = pixelDepth
				const dst = (y * imageWidth + viewIndex * viewSize + x) * 3
				for (let channel = 0; channel < 3; channel++)
					img[dst + channel] = Math.round(col[channel] * shade)
			}
	}
})

fs.writeFileSync(out, png.encode(imageWidth, imageHeight, img, 3))
console.log(
	`${inp}: ${positions.length} verts, ${triangles.length} faces, ${mats.size} textures -> ${out}`,
)
