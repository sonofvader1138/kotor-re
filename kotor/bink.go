// This file parses the header and frame index of Bink video files (signature "BIKi"). Layout: see
// FILE_FORMATS.md. A file passes when the header size field matches the file size, the frame index is
// increasing and ends exactly at end of file, the largest frame field equals the largest frame, and every
// frame's audio packets fit inside the frame.

package kotor

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Header 44 bytes, then 12 bytes per audio track (three arrays of 4 bytes each), then the frame index.
const (
	binkHeaderSize = 44
	trackSize      = 12
)

var binkSignature = regexp.MustCompile(`^BIK[a-z]$`)

// The size field holds the file size minus 8.
const sizeFieldBias = 8

// Frame index entries are u32. Bit 0 marks a keyframe.
const (
	indexEntrySize = 4
	keyframeBit    = 1
)

// Audio packets start with a u32 size. A packet's decoded size is also a u32.
const packetSizeField = 4

// BinkTrackStereo is bit 13 of a track's flags, which probably marks stereo audio. All files have it set, so
// this is not confirmed.
const BinkTrackStereo = 0x2000

type BinkTrack struct {
	MaxDecoded int
	SampleRate int
	Flags      int
	ID         int
}

type BinkHeader struct {
	Signature    string
	SizeMinus8   int
	Frames       int
	LargestFrame int
	Frames2      int
	Width        int
	Height       int
	FpsNum       int
	FpsDen       int
	VideoFlags   int
	AudioTracks  int
	Tracks       []BinkTrack
}

// BinkResult is a parsed file. Header is nil when the file is shorter than the header. The frame statistics
// are set only when the frame index lies inside the file (Indexed).
type BinkResult struct {
	Header           *BinkHeader
	Errors           []string
	Indexed          bool
	Keyframes        int
	FirstKey         int
	AudioBytes       int
	MaxPacketDecoded []int
	DecodedBytes     []int
	HeaderEnd        int
}

func ParseBink(file []byte) *BinkResult {
	result := &BinkResult{}
	if len(file) < binkHeaderSize {
		result.Errors = []string{"shorter than header"}
		return result
	}

	u32 := func(offset int) int {
		return int(binary.LittleEndian.Uint32(file[offset:]))
	}
	header := &BinkHeader{
		Signature:    Latin1(file[0:4]),
		SizeMinus8:   u32(4),
		Frames:       u32(8),
		LargestFrame: u32(12),
		Frames2:      u32(16),
		Width:        u32(20),
		Height:       u32(24),
		FpsNum:       u32(28),
		FpsDen:       u32(32),
		VideoFlags:   u32(36),
		AudioTracks:  u32(40),
	}
	result.Header = header
	if !binkSignature.MatchString(header.Signature) {
		result.Errors = append(result.Errors, "binkSignature "+header.Signature)
	}
	if header.SizeMinus8+sizeFieldBias != len(file) {
		result.Errors = append(result.Errors,
			fmt.Sprintf("size field %d != file size %d", header.SizeMinus8+sizeFieldBias, len(file)))
	}

	trackCount := header.AudioTracks
	tableAt := binkHeaderSize + trackSize*trackCount
	headerEnd := tableAt + indexEntrySize*(header.Frames+1)
	if headerEnd > len(file) {
		result.Errors = append(result.Errors, "frame index past end of file")
		return result
	}

	// The three track arrays follow each other; each holds 4 bytes per track.
	for track := 0; track < trackCount; track++ {
		maxDecoded := binkHeaderSize + 4*track
		rateAndFlags := maxDecoded + 4*trackCount
		id := rateAndFlags + 4*trackCount
		header.Tracks = append(header.Tracks, BinkTrack{
			MaxDecoded: u32(maxDecoded),
			SampleRate: int(binary.LittleEndian.Uint16(file[rateAndFlags:])),
			Flags:      int(binary.LittleEndian.Uint16(file[rateAndFlags+2:])),
			ID:         u32(id),
		})
	}

	// Frame index: frames + 1 u32 entries. Bit 0 marks a keyframe; the rest is the frame's file offset.
	// The last entry is the end of the last frame.
	offsets := make([]int, header.Frames+1)
	keys := make([]int, header.Frames+1)
	for i := range offsets {
		entry := u32(tableAt + indexEntrySize*i)
		offsets[i] = entry &^ keyframeBit
		keys[i] = entry & keyframeBit
	}

	if offsets[0] != headerEnd {
		result.Errors = append(result.Errors, fmt.Sprintf("first frame at %d, header ends at %d", offsets[0], headerEnd))
	}
	if offsets[header.Frames] != len(file) {
		result.Errors = append(result.Errors,
			fmt.Sprintf("last frame ends at %d, file size %d", offsets[header.Frames], len(file)))
	}

	largest := 0
	result.Indexed = true
	result.MaxPacketDecoded = make([]int, trackCount)
	result.DecodedBytes = make([]int, trackCount)
	for i := 0; i < header.Frames; i++ {
		start, end := offsets[i], offsets[i+1]
		if end <= start || end > len(file) {
			result.Errors = append(result.Errors, fmt.Sprintf("frame %d range %d..%d", i, start, end))
			break
		}

		largest = max(largest, end-start)
		result.Keyframes += keys[i]

		// Each frame begins with one audio packet per track: u32 size, then size bytes.
		// A packet of 4 or more bytes starts with u32 decoded size in bytes.
		packet := start
		for track := 0; track < trackCount; track++ {
			size := u32(packet)
			if packet+packetSizeField+size > end {
				result.Errors = append(result.Errors,
					fmt.Sprintf("frame %d track %d audio packet %d past frame end", i, track, size))
				break
			}

			if size >= packetSizeField {
				decoded := u32(packet + packetSizeField)
				result.MaxPacketDecoded[track] = max(result.MaxPacketDecoded[track], decoded)
				result.DecodedBytes[track] += decoded
			}

			result.AudioBytes += size
			packet += packetSizeField + size
		}
	}

	if largest != header.LargestFrame {
		result.Errors = append(result.Errors,
			fmt.Sprintf("largest frame field %d, largest frame %d", header.LargestFrame, largest))
	}
	if header.Frames2 != header.Frames {
		result.Errors = append(result.Errors, fmt.Sprintf("second frame count %d != %d", header.Frames2, header.Frames))
	}

	result.FirstKey = keys[0]
	result.HeaderEnd = headerEnd
	return result
}

func ReportBink(name string, result *BinkResult) string {
	header := result.Header
	if header == nil {
		return name + " FAIL: " + strings.Join(result.Errors, "; ")
	}

	tracks := make([]string, len(header.Tracks))
	for i, track := range header.Tracks {
		packetMax := "undefined"
		if result.Indexed {
			packetMax = strconv.Itoa(result.MaxPacketDecoded[i])
		}
		tracks[i] = fmt.Sprintf("%dHz flags=0x%x id=%d maxDecoded=%d packetMax=%s",
			track.SampleRate, track.Flags, track.ID, track.MaxDecoded, packetMax)
	}

	// The JS version printed missing statistics as "undefined" when the frame index lay past the end.
	statistic := func(value int) string {
		if !result.Indexed {
			return "undefined"
		}
		return strconv.Itoa(value)
	}

	return fmt.Sprintf("%s %s %dx%d frames=%d fps=%d/%d videoFlags=0x%x keyframes=%s firstKey=%s tracks=%d [%s] audioBytes=%s %s",
		name, header.Signature, header.Width, header.Height, header.Frames, header.FpsNum, header.FpsDen,
		header.VideoFlags, statistic(result.Keyframes), statistic(result.FirstKey), header.AudioTracks,
		strings.Join(tracks, ","), statistic(result.AudioBytes), Verdict(result.Errors))
}
