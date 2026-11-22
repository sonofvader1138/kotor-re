// usage: node bink.js file.bik...
// Bink video (signature "BIKi") header and frame index parser. Layout: see FILE_FORMATS.md.
// Prints one line per file ending in PASS or FAIL. A file passes when the header size field matches
// the file size, the frame index is increasing and ends exactly at end of file, the largest frame
// field equals the largest frame, and every frame's audio packets fit inside the frame.
const fs = require("fs")
const path = require("path")

// Header 44 bytes, then 12 bytes per audio track (three arrays of 4 bytes each), then the frame index.
function parse(b) {
	const errors = []
	if (b.length < 44) return { errors: ["shorter than header"] }
	const h = {
		signature: b.toString("latin1", 0, 4),
		sizeMinus8: b.readUInt32LE(4),
		frames: b.readUInt32LE(8),
		largestFrame: b.readUInt32LE(12),
		frames2: b.readUInt32LE(16),
		width: b.readUInt32LE(20),
		height: b.readUInt32LE(24),
		fpsNum: b.readUInt32LE(28),
		fpsDen: b.readUInt32LE(32),
		videoFlags: b.readUInt32LE(36),
		audioTracks: b.readUInt32LE(40),
		tracks: [],
	}
	if (!/^BIK[a-z]$/.test(h.signature)) errors.push(`signature ${h.signature}`)
	if (h.sizeMinus8 + 8 !== b.length)
		errors.push(`size field ${h.sizeMinus8 + 8} != file size ${b.length}`)
	const n = h.audioTracks
	const tableAt = 44 + 12 * n
	if (tableAt + 4 * (h.frames + 1) > b.length)
		return { h, errors: [...errors, "frame index past end of file"] }
	for (let t = 0; t < n; t++)
		h.tracks.push({
			maxDecoded: b.readUInt32LE(44 + 4 * t),
			sampleRate: b.readUInt16LE(44 + 4 * n + 4 * t),
			flags: b.readUInt16LE(46 + 4 * n + 4 * t),
			id: b.readUInt32LE(44 + 8 * n + 4 * t),
		})
	// Frame index: frames + 1 u32 entries. Bit 0 marks a keyframe; the rest is the frame's file offset.
	// The last entry is the end of the last frame.
	const offs = [],
		keys = []
	for (let i = 0; i <= h.frames; i++) {
		const v = b.readUInt32LE(tableAt + 4 * i)
		offs.push(v & ~1)
		keys.push(v & 1)
	}
	const headerEnd = tableAt + 4 * (h.frames + 1)
	if (offs[0] !== headerEnd)
		errors.push(`first frame at ${offs[0]}, header ends at ${headerEnd}`)
	if (offs[h.frames] !== b.length)
		errors.push(`last frame ends at ${offs[h.frames]}, file size ${b.length}`)
	let largest = 0,
		keyframes = 0,
		audioBytes = 0,
		maxPacketDecoded = new Array(n).fill(0),
		decodedBytes = new Array(n).fill(0)
	for (let i = 0; i < h.frames; i++) {
		const start = offs[i],
			end = offs[i + 1]
		if (end <= start || end > b.length) {
			errors.push(`frame ${i} range ${start}..${end}`)
			break
		}
		largest = Math.max(largest, end - start)
		keyframes += keys[i]
		// Each frame begins with one audio packet per track: u32 size, then size bytes.
		// A packet of 4 or more bytes starts with u32 decoded size in bytes.
		let p = start
		for (let t = 0; t < n; t++) {
			const size = b.readUInt32LE(p)
			if (p + 4 + size > end) {
				errors.push(`frame ${i} track ${t} audio packet ${size} past frame end`)
				break
			}
			if (size >= 4) {
				const d = b.readUInt32LE(p + 4)
				maxPacketDecoded[t] = Math.max(maxPacketDecoded[t], d)
				decodedBytes[t] += d
			}
			audioBytes += size
			p += 4 + size
		}
	}
	if (largest !== h.largestFrame)
		errors.push(
			`largest frame field ${h.largestFrame}, largest frame ${largest}`,
		)
	if (h.frames2 !== h.frames)
		errors.push(`second frame count ${h.frames2} != ${h.frames}`)
	return {
		h,
		errors,
		keyframes,
		firstKey: keys[0],
		audioBytes,
		maxPacketDecoded,
		decodedBytes,
		headerEnd,
	}
}

function report(name, r) {
	const h = r.h
	if (!h) return `${name} FAIL: ${r.errors.join("; ")}`
	const tr = h.tracks
		.map(
			(t) =>
				`${t.sampleRate}Hz flags=0x${t.flags.toString(16)} id=${t.id} maxDecoded=${t.maxDecoded} packetMax=${r.maxPacketDecoded?.[h.tracks.indexOf(t)]}`,
		)
		.join(",")
	return (
		`${name} ${h.signature} ${h.width}x${h.height} frames=${h.frames} fps=${h.fpsNum}/${h.fpsDen} ` +
		`videoFlags=0x${h.videoFlags.toString(16)} keyframes=${r.keyframes} firstKey=${r.firstKey} ` +
		`tracks=${h.audioTracks} [${tr}] audioBytes=${r.audioBytes} ` +
		(r.errors.length ? "FAIL: " + r.errors.join("; ") : "PASS")
	)
}

module.exports = { parse, report }

if (require.main === module)
	for (const f of process.argv.slice(2))
		console.log(report(path.basename(f), parse(fs.readFileSync(f))))
