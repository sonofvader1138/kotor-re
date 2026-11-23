// This file is a minimal PNG encoder: 8-bit gray, RGB or RGBA, filter 0, one IDAT chunk.

package kotor

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
)

var pngSignature = []byte{137, 80, 78, 71, 13, 10, 26, 10}

const (
	ihdrSize = 13
	bitDepth = 8
)

// PNG color type for each channel count: grayscale, true color, true color with alpha.
var colorType = map[int]byte{1: 0, 3: 2, 4: 6}

// EncodePng writes width x height pixels of the given channel count, top row first, as a PNG file.
func EncodePng(width, height int, pix []byte, channels int) []byte {
	stride := width * channels
	raw := make([]byte, (stride+1)*height)
	for y := range height {
		copy(raw[y*(stride+1)+1:], pix[y*stride:(y+1)*stride])
	}

	var out []byte
	chunk := func(chunkType string, data []byte) {
		out = binary.BigEndian.AppendUint32(out, uint32(len(data)))
		typeAndData := append([]byte(chunkType), data...)
		out = append(out, typeAndData...)
		out = binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(typeAndData))
	}

	ihdr := make([]byte, ihdrSize)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(height))
	ihdr[8] = bitDepth
	ihdr[9] = colorType[channels]

	// Writing to a bytes.Buffer cannot fail, so the errors are discarded.
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, _ = writer.Write(raw)
	_ = writer.Close()

	out = append(out, pngSignature...)
	chunk("IHDR", ihdr)
	chunk("IDAT", compressed.Bytes())
	chunk("IEND", nil)
	return out
}
