// This file unwraps KotOR stream audio. Two layouts exist:
//
//	A: a 58-byte fake WAV header with data size 0, then an MP3 stream, which becomes <name>.mp3.
//	B: three MP3 frames of silence, then a complete RIFF WAV file, which becomes <name>.wav. The WAV file is
//	   returned as stored and may hold IMA ADPCM.
//
// The MP3 frame walk or RIFF chunk walk must consume the payload exactly.

package kotor

import (
	"encoding/binary"
	"fmt"
	"slices"
)

var (
	bitratesMpeg1 = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	bitratesMpeg2 = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
)

// MPEG audio version codes from the frame header. Code 1 is reserved.
const (
	mpeg25        = 0
	mpegReserved  = 1
	mpeg2         = 2
	mpeg1         = 3
	layerIII      = 1
	bitrateFree   = 0
	bitrateBad    = 15
	rateReserved  = 3
	frameSync     = 0x7ff
	id3v2Header   = 10
	id3v1TagSize  = 128
	coefficient1  = 144
	coefficient2  = 72
	fakeRiffSize  = 50
	fakeDataField = 54
)

// Layer code 1 is Layer III. Bitrate index 0 means free format and 15 is invalid. Sample rate index 3 is
// reserved. Every frame header starts with 11 set bits. A Layer III frame holds 1152 samples in MPEG-1
// and 576 in MPEG-2 and 2.5. Its byte length is samples / 8 * bitrate / sample rate, which gives the
// coefficients 144 and 72.
var sampleRates = map[int][]int{
	mpeg1:  {44100, 48000, 32000},
	mpeg2:  {22050, 24000, 16000},
	mpeg25: {11025, 12000, 8000},
}

// RIFF: 4-byte id, u32 size of the rest of the file, 4-byte form type, then chunks of
// 4-byte id, u32 size, data padded to an even length.
const (
	RiffHeaderSize  = 12
	ChunkHeaderSize = 8
)

// Layout A: a 58-byte WAV header whose RIFF size field is 50 and whose data chunk size, at offset 54, is 0.
const fakeHeaderSize = 58

func text(data []byte, start, end int) string {
	end = min(end, len(data))
	if start >= end {
		return ""
	}
	return string(data[start:end])
}

// Walk is the result of an MP3 frame walk. Err is "riff" when the walk stopped at an embedded RIFF file.
type Walk struct {
	End    int
	Frames int
	Err    string
}

// Mp3Walk walks MP3 frames from pos.
func Mp3Walk(mp3 []byte, pos int) Walk {
	frames := 0
	if text(mp3, pos, pos+3) == "ID3" {
		pos += id3v2Header + (int(mp3[pos+6])<<21 | int(mp3[pos+7])<<14 | int(mp3[pos+8])<<7 | int(mp3[pos+9]))
	}

	for pos < len(mp3) {
		if len(mp3)-pos == id3v1TagSize && text(mp3, pos, pos+3) == "TAG" {
			return Walk{pos + id3v1TagSize, frames, ""}
		}
		if text(mp3, pos, pos+4) == "RIFF" {
			return Walk{pos, frames, "riff"}
		}
		if pos+4 > len(mp3) {
			return Walk{pos, frames, fmt.Sprintf("trailing %d", len(mp3)-pos)}
		}

		header := binary.BigEndian.Uint32(mp3[pos:])
		if header>>21 != frameSync {
			return Walk{pos, frames, fmt.Sprintf("no sync at %d", pos)}
		}

		version := int(header >> 19 & 3)
		layer := int(header >> 17 & 3)
		bitrateIndex := int(header >> 12 & 15)
		rateIndex := int(header >> 10 & 3)
		padding := int(header >> 9 & 1)
		if layer != layerIII || version == mpegReserved || bitrateIndex == bitrateFree ||
			bitrateIndex == bitrateBad || rateIndex == rateReserved {
			return Walk{pos, frames, fmt.Sprintf("bad header at %d", pos)}
		}

		bitrates, coefficient := bitratesMpeg2, coefficient2
		if version == mpeg1 {
			bitrates, coefficient = bitratesMpeg1, coefficient1
		}
		pos += coefficient*bitrates[bitrateIndex]*1000/sampleRates[version][rateIndex] + padding
		frames++
	}

	err := ""
	if pos > len(mp3) {
		err = "overrun"
	}
	return Walk{pos, frames, err}
}

// RiffWalk checks that the chunks of a RIFF file end exactly at its end and include a data chunk.
func RiffWalk(riff []byte) string {
	if int(binary.LittleEndian.Uint32(riff[4:])) != len(riff)-ChunkHeaderSize {
		return "bad RIFF size"
	}

	pos := RiffHeaderSize
	var ids []string
	for pos+ChunkHeaderSize <= len(riff) {
		chunkSize := int(binary.LittleEndian.Uint32(riff[pos+4:]))
		ids = append(ids, string(riff[pos:pos+4]))
		pos += ChunkHeaderSize + chunkSize + chunkSize&1
	}

	if pos != len(riff) {
		return fmt.Sprintf("chunk end %d/%d", pos, len(riff))
	}
	if !slices.Contains(ids, "data") {
		return "no data chunk"
	}
	return ""
}

// Unwrapped is the audio file inside a stream file. Kind is "mp3" or "wav".
type Unwrapped struct {
	Kind    string
	Payload []byte
	Info    string
	Err     string
}

func UnwrapAudio(file []byte) Unwrapped {
	if text(file, 0, 4) == "RIFF" && binary.LittleEndian.Uint32(file[4:]) == fakeRiffSize &&
		binary.LittleEndian.Uint32(file[fakeDataField:]) == 0 {
		payload := file[fakeHeaderSize:]
		walk := Mp3Walk(payload, 0)
		return Unwrapped{"mp3", payload, fmt.Sprintf("frames=%d", walk.Frames), walk.Err}
	}

	walk := Mp3Walk(file, 0)
	if walk.Err == "riff" {
		payload := file[walk.End:]
		info := fmt.Sprintf("stub=%d fmt=%d ch=%d sr=%d bits=%d", walk.End,
			binary.LittleEndian.Uint16(payload[20:]), binary.LittleEndian.Uint16(payload[22:]),
			binary.LittleEndian.Uint32(payload[24:]), binary.LittleEndian.Uint16(payload[34:]))
		return Unwrapped{"wav", payload, info, RiffWalk(payload)}
	}

	return Unwrapped{"mp3", file, fmt.Sprintf("raw frames=%d", walk.Frames), walk.Err}
}
