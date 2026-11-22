// usage: node bink.js file.bik...
// Bink video (signature "BIKi") header and frame index parser. Layout: see FILE_FORMATS.md.
// Prints one line per file ending in PASS or FAIL. A file passes when the header size field matches
// the file size, the frame index is increasing and ends exactly at end of file, the largest frame
// field equals the largest frame, and every frame's audio packets fit inside the frame.
const fs = require("fs")
const path = require("path")

// Header 44 bytes, then 12 bytes per audio track (three arrays of 4 bytes each), then the frame index.
function parse(file) {
	const errors = []
	if (file.length < 44) return { errors: ["shorter than header"] }
	const header = {
		signature: file.toString("latin1", 0, 4),
		sizeMinus8: file.readUInt32LE(4),
		frames: file.readUInt32LE(8),
		largestFrame: file.readUInt32LE(12),
		frames2: file.readUInt32LE(16),
		width: file.readUInt32LE(20),
		height: file.readUInt32LE(24),
		fpsNum: file.readUInt32LE(28),
		fpsDen: file.readUInt32LE(32),
		videoFlags: file.readUInt32LE(36),
		audioTracks: file.readUInt32LE(40),
		tracks: [],
	}
	if (!/^BIK[a-z]$/.test(header.signature))
		errors.push(`signature ${header.signature}`)
	if (header.sizeMinus8 + 8 !== file.length)
		errors.push(
			`size field ${header.sizeMinus8 + 8} != file size ${file.length}`,
		)
	const trackCount = header.audioTracks
	const tableAt = 44 + 12 * trackCount
	if (tableAt + 4 * (header.frames + 1) > file.length)
		return { header, errors: [...errors, "frame index past end of file"] }
	for (let track = 0; track < trackCount; track++)
		header.tracks.push({
			maxDecoded: file.readUInt32LE(44 + 4 * track),
			sampleRate: file.readUInt16LE(44 + 4 * trackCount + 4 * track),
			flags: file.readUInt16LE(46 + 4 * trackCount + 4 * track),
			id: file.readUInt32LE(44 + 8 * trackCount + 4 * track),
		})
	// Frame index: frames + 1 u32 entries. Bit 0 marks a keyframe; the rest is the frame's file offset.
	// The last entry is the end of the last frame.
	const offs = [],
		keys = []
	for (let i = 0; i <= header.frames; i++) {
		const entry = file.readUInt32LE(tableAt + 4 * i)
		offs.push(entry & ~1)
		keys.push(entry & 1)
	}
	const headerEnd = tableAt + 4 * (header.frames + 1)
	if (offs[0] !== headerEnd)
		errors.push(`first frame at ${offs[0]}, header ends at ${headerEnd}`)
	if (offs[header.frames] !== file.length)
		errors.push(
			`last frame ends at ${offs[header.frames]}, file size ${file.length}`,
		)
	let largest = 0,
		keyframes = 0,
		audioBytes = 0,
		maxPacketDecoded = new Array(trackCount).fill(0),
		decodedBytes = new Array(trackCount).fill(0)
	for (let i = 0; i < header.frames; i++) {
		const start = offs[i],
			end = offs[i + 1]
		if (end <= start || end > file.length) {
			errors.push(`frame ${i} range ${start}..${end}`)
			break
		}
		largest = Math.max(largest, end - start)
		keyframes += keys[i]
		// Each frame begins with one audio packet per track: u32 size, then size bytes.
		// A packet of 4 or more bytes starts with u32 decoded size in bytes.
		let packet = start
		for (let track = 0; track < trackCount; track++) {
			const size = file.readUInt32LE(packet)
			if (packet + 4 + size > end) {
				errors.push(
					`frame ${i} track ${track} audio packet ${size} past frame end`,
				)
				break
			}
			if (size >= 4) {
				const decoded = file.readUInt32LE(packet + 4)
				maxPacketDecoded[track] = Math.max(maxPacketDecoded[track], decoded)
				decodedBytes[track] += decoded
			}
			audioBytes += size
			packet += 4 + size
		}
	}
	if (largest !== header.largestFrame)
		errors.push(
			`largest frame field ${header.largestFrame}, largest frame ${largest}`,
		)
	if (header.frames2 !== header.frames)
		errors.push(`second frame count ${header.frames2} != ${header.frames}`)
	return {
		header,
		errors,
		keyframes,
		firstKey: keys[0],
		audioBytes,
		maxPacketDecoded,
		decodedBytes,
		headerEnd,
	}
}

function report(name, result) {
	const header = result.header
	if (!header) return `${name} FAIL: ${result.errors.join("; ")}`
	const trackText = header.tracks
		.map(
			(track, index) =>
				`${track.sampleRate}Hz flags=0x${track.flags.toString(16)} id=${track.id} maxDecoded=${track.maxDecoded} packetMax=${result.maxPacketDecoded?.[index]}`,
		)
		.join(",")
	return (
		`${name} ${header.signature} ${header.width}x${header.height} frames=${header.frames} fps=${header.fpsNum}/${header.fpsDen} ` +
		`videoFlags=0x${header.videoFlags.toString(16)} keyframes=${result.keyframes} firstKey=${result.firstKey} ` +
		`tracks=${header.audioTracks} [${trackText}] audioBytes=${result.audioBytes} ` +
		(result.errors.length ? "FAIL: " + result.errors.join("; ") : "PASS")
	)
}

module.exports = { parse, report }

if (require.main === module)
	for (const file of process.argv.slice(2))
		console.log(report(path.basename(file), parse(fs.readFileSync(file))))
