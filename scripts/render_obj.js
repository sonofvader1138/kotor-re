// usage: node render_obj.js in.obj out.png [size]
// Software render, 4 orthographic views (front, back, left, right; Z up), z-buffer, back-face culling.
// Faces whose material has a map_Kd PNG are textured, using the OBJ convention that v = 0 is the
// bottom row of the image. Other faces are gray. Both are shaded by the face normal.
// Texels with alpha below 128 are not drawn. Culled faces are not drawn, like a typical viewer.
const fs = require("fs")
const path = require("path")
const PNG = require("./png")
const [inp, out, size = "400"] = process.argv.slice(2)
const S = +size

const V = [],
	T = [],
	F = [],
	mats = new Map()
let cur = null
const dir = path.dirname(inp)
for (const l of fs.readFileSync(inp, "utf8").split("\n")) {
	const t = l.trim().split(/\s+/)
	if (t[0] === "v") V.push(t.slice(1, 4).map(Number))
	else if (t[0] === "vt") T.push(t.slice(1, 3).map(Number))
	else if (t[0] === "usemtl") cur = t[1]
	else if (t[0] === "mtllib") {
		const f = path.join(dir, t.slice(1).join(" "))
		if (!fs.existsSync(f)) continue
		let m = null
		for (const ml of fs.readFileSync(f, "utf8").split("\n")) {
			const mt = ml.trim().split(/\s+/)
			if (mt[0] === "newmtl") m = mt[1]
			else if (mt[0] === "map_Kd" && m) {
				const tf = path.join(path.dirname(f), mt.slice(1).join(" "))
				if (fs.existsSync(tf)) mats.set(m, PNG.decode(fs.readFileSync(tf)))
			}
		}
	} else if (t[0] === "f") {
		const ix = t.slice(1).map((s) => s.split("/").map((n) => +n - 1))
		for (let i = 1; i + 1 < ix.length; i++)
			F.push({
				v: [ix[0][0], ix[i][0], ix[i + 1][0]],
				t: [ix[0][1], ix[i][1], ix[i + 1][1]],
				m: cur,
			})
	}
}
const lo = [0, 1, 2].map((k) => Math.min(...V.map((v) => v[k]))),
	hi = [0, 1, 2].map((k) => Math.max(...V.map((v) => v[k])))
const c = [0, 1, 2].map((k) => (lo[k] + hi[k]) / 2),
	r = Math.max(...[0, 1, 2].map((k) => hi[k] - lo[k])) / 2 || 1

// Each view gives the screen's right axis and the direction toward the camera (fwd). Screen up is +Z.
const views = [
	{ right: [1, 0, 0], fwd: [0, -1, 0] },
	{ right: [-1, 0, 0], fwd: [0, 1, 0] },
	{ right: [0, -1, 0], fwd: [-1, 0, 0] },
	{ right: [0, 1, 0], fwd: [1, 0, 0] },
]
const W = S * 4,
	H = S,
	img = Buffer.alloc(W * H * 3, 40)
const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
const fract = (x) => x - Math.floor(x)

views.forEach((vw, vi) => {
	const z = new Float64Array(S * S).fill(-Infinity),
		up = [0, 0, 1]
	const P = V.map((v) => {
		const d = [0, 1, 2].map((k) => (v[k] - c[k]) / r)
		return [
			S / 2 + dot(d, vw.right) * S * 0.45,
			S / 2 - dot(d, up) * S * 0.45,
			dot(d, vw.fwd),
		]
	})
	for (const {
		v: [a, b, d],
		t: ti,
		m,
	} of F) {
		const A = V[a],
			B = V[b],
			C = V[d]
		const u = [0, 1, 2].map((k) => B[k] - A[k]),
			w = [0, 1, 2].map((k) => C[k] - A[k])
		const n = [
			u[1] * w[2] - u[2] * w[1],
			u[2] * w[0] - u[0] * w[2],
			u[0] * w[1] - u[1] * w[0],
		]
		const ln = Math.hypot(...n)
		if (!ln) continue
		const facing = dot(n, vw.fwd) / ln
		if (facing <= 0) continue // back-face cull (CCW = front)
		const shade = 0.25 + 0.75 * facing
		const tex = mats.get(m),
			uv = tex && ti.every((i) => T[i]) ? ti.map((i) => T[i]) : null
		const [p, q, s] = [P[a], P[b], P[d]]
		const x0 = Math.max(0, Math.floor(Math.min(p[0], q[0], s[0]))),
			x1 = Math.min(S - 1, Math.ceil(Math.max(p[0], q[0], s[0])))
		const y0 = Math.max(0, Math.floor(Math.min(p[1], q[1], s[1]))),
			y1 = Math.min(S - 1, Math.ceil(Math.max(p[1], q[1], s[1])))
		const area = (q[0] - p[0]) * (s[1] - p[1]) - (q[1] - p[1]) * (s[0] - p[0])
		if (!area) continue
		for (let y = y0; y <= y1; y++)
			for (let x = x0; x <= x1; x++) {
				const w0 = ((q[0] - x) * (s[1] - y) - (q[1] - y) * (s[0] - x)) / area
				const w1 = ((s[0] - x) * (p[1] - y) - (s[1] - y) * (p[0] - x)) / area,
					w2 = 1 - w0 - w1
				if (w0 < 0 || w1 < 0 || w2 < 0) continue
				const zz = w0 * p[2] + w1 * q[2] + w2 * s[2]
				if (zz <= z[y * S + x]) continue
				let col = [200, 200, 200]
				if (uv) {
					const tu = w0 * uv[0][0] + w1 * uv[1][0] + w2 * uv[2][0],
						tv = w0 * uv[0][1] + w1 * uv[1][1] + w2 * uv[2][1]
					const tx = Math.min(tex.w - 1, Math.floor(fract(tu) * tex.w)),
						ty = Math.min(tex.h - 1, Math.floor((1 - fract(tv)) * tex.h))
					const o = 4 * (ty * tex.w + tx)
					if (tex.rgba[o + 3] < 128) continue
					col = [tex.rgba[o], tex.rgba[o + 1], tex.rgba[o + 2]]
				}
				z[y * S + x] = zz
				const o = (y * W + vi * S + x) * 3
				for (let k = 0; k < 3; k++) img[o + k] = Math.round(col[k] * shade)
			}
	}
})

fs.writeFileSync(out, PNG.encode(W, H, img, 3))
console.log(
	`${inp}: ${V.length} verts, ${F.length} faces, ${mats.size} textures -> ${out}`,
)
