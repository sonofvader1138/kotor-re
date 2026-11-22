// usage: node bink.js file.bik...
// Bink video (signature "BIKi") header and frame index parser. Layout: see FILE_FORMATS.md.
// Prints one line per file ending in PASS or FAIL. A file passes when the header size field matches
// the file size, the frame index is increasing and ends exactly at end of file, the largest frame
// field equals the largest frame, and every frame's audio packets fit inside the frame.
const fs = require("fs")
const path = require("path")

// Header 44 bytes, then 12 bytes per audio track (three arrays of 4 bytes each), then the frame index.
const HEADER_SIZE = 44
const TRACK_SIZE = 12
const SIGNATURE = /^BIK[a-z]$/

// The size field holds the file size minus 8.
const SIZE_FIELD_BIAS = 8

// Frame index entries are u32. Bit 0 marks a keyframe.
const INDEX_ENTRY_SIZE = 4
const KEYFRAME_BIT = 1

// Audio packets start with a u32 size. A packet's decoded size is also a u32.
const PACKET_SIZE_FIELD = 4

// Bit 13 of a track's flags probably marks stereo audio. All files have it set, so this is not confirmed.
const TRACK_STEREO = 0x2000

function parse(file) {
	const errors = []
	if (file.length < HEADER_SIZE) {
		return { errors: ["shorter than header"] }
	}

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
	if (!SIGNATURE.test(header.signature)) {
		errors.push(`signature ${header.signature}`)
	}
	if (header.sizeMinus8 + SIZE_FIELD_BIAS !== file.length) {
		errors.push(
			`size field ${header.sizeMinus8 + SIZE_FIELD_BIAS} != file size ${file.length}`,
		)
	}

	const trackCount = header.audioTracks
	const tableAt = HEADER_SIZE + TRACK_SIZE * trackCount
	const headerEnd = tableAt + INDEX_ENTRY_SIZE * (header.frames + 1)
	if (headerEnd > file.length) {
		return { header, errors: [...errors, "frame index past end of file"] }
	}

	// The three track arrays follow each other; each holds 4 bytes per track.
	for (let track = 0; track < trackCount; track++) {
		const maxDecoded = HEADER_SIZE + 4 * track,
			rateAndFlags = maxDecoded + 4 * trackCount,
			id = rateAndFlags + 4 * trackCount
		header.tracks.push({
			maxDecoded: file.readUInt32LE(maxDecoded),
			sampleRate: file.readUInt16LE(rateAndFlags),
			flags: file.readUInt16LE(rateAndFlags + 2),
			id: file.readUInt32LE(id),
		})
	}

	// Frame index: frames + 1 u32 entries. Bit 0 marks a keyframe; the rest is the frame's file offset.
	// The last entry is the end of the last frame.
	const offs = [],
		keys = []
	for (let i = 0; i <= header.frames; i++) {
		const entry = file.readUInt32LE(tableAt + INDEX_ENTRY_SIZE * i)
		offs.push(entry & ~KEYFRAME_BIT)
		keys.push(entry & KEYFRAME_BIT)
	}

	if (offs[0] !== headerEnd) {
		errors.push(`first frame at ${offs[0]}, header ends at ${headerEnd}`)
	}
	if (offs[header.frames] !== file.length) {
		errors.push(
			`last frame ends at ${offs[header.frames]}, file size ${file.length}`,
		)
	}

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
			if (packet + PACKET_SIZE_FIELD + size > end) {
				errors.push(
					`frame ${i} track ${track} audio packet ${size} past frame end`,
				)
				break
			}

			if (size >= PACKET_SIZE_FIELD) {
				const decoded = file.readUInt32LE(packet + PACKET_SIZE_FIELD)
				maxPacketDecoded[track] = Math.max(maxPacketDecoded[track], decoded)
				decodedBytes[track] += decoded
			}

			audioBytes += size
			packet += PACKET_SIZE_FIELD + size
		}
	}

	if (largest !== header.largestFrame) {
		errors.push(
			`largest frame field ${header.largestFrame}, largest frame ${largest}`,
		)
	}
	if (header.frames2 !== header.frames) {
		errors.push(`second frame count ${header.frames2} != ${header.frames}`)
	}

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
	if (!header) {
		return `${name} FAIL: ${result.errors.join("; ")}`
	}

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

if (require.main === module) {
	for (const file of process.argv.slice(2)) {
		console.log(report(path.basename(file), parse(fs.readFileSync(file))))
	}
}

module.exports = { TRACK_STEREO, parse, report }
