// This file reads TPC textures and decodes them to RGBA, 8 bits per channel. Layout: see FILE_FORMATS.md.
// Header 128 bytes: u32 dataSize (0 = uncompressed), f32 alpha threshold, u16 width, u16 height,
// u8 encoding, u8 mip count, zero padding. Pixel data for all mip levels follows. An optional
// ASCII TXI (texture info) trailer fills the rest of the file.
// Only the top mip level is decoded. Cube faces are stacked vertically; flipbook frames keep their grid.

package kotor

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const TpcHeaderSize = 128

// Storage kind for each encoding byte. Uncompressed files have dataSize 0.
var (
	uncompressedKind = map[byte]string{1: "gray", 2: "rgb", 4: "rgba"}
	compressedKind   = map[byte]string{2: "dxt1", 4: "dxt5"}
	bytesPerPixel    = map[string]int{"gray": 1, "rgb": 3, "rgba": 4}
)

// DXT compresses blocks of 4 x 4 pixels. A DXT1 block is 8 bytes. A DXT5 block is 8 bytes of alpha,
// then a DXT1-style 8-byte color block.
const (
	blockPixels    = 4
	dxt1BlockSize  = 8
	dxt5BlockSize  = 16
	blockTexels    = blockPixels * blockPixels
	cubeFaces      = 6
	opaque         = 255
	printableFirst = 32
	printableLast  = 126
)

// The TXI trailer gives a flipbook's frame grid in "numx <columns>" and "numy <rows>" lines.
var (
	numxPattern = regexp.MustCompile(`(?im)^\s*numx\s+(\d+)`)
	numyPattern = regexp.MustCompile(`(?im)^\s*numy\s+(\d+)`)
)

func blocksAcross(pixels int) int {
	return max(1, (pixels+blockPixels-1)/blockPixels)
}

// LevelSize is the byte size of one image of width x height pixels for a given storage kind.
func LevelSize(kind string, width, height int) int {
	if kind == "dxt1" || kind == "dxt5" {
		blockSize := dxt5BlockSize
		if kind == "dxt1" {
			blockSize = dxt1BlockSize
		}
		return blocksAcross(width) * blocksAcross(height) * blockSize
	}
	return width * height * bytesPerPixel[kind]
}

// SubImage is one image of the pixel data with its own mip chain, placed at (X, Y) in the output.
type SubImage struct {
	X      int
	Y      int
	Width  int
	Height int
	Levels float64
	Offset int
}

// TpcTexture holds the parsed header. Kind is empty when the file is shorter than the header or uses an
// unknown encoding, in which case only Errors is set.
type TpcTexture struct {
	DataSize int
	Alpha    float64
	Width    int
	Height   int
	Enc      int
	Mips     int
	Kind     string
	Layout   string
	Images   []SubImage
	Pixels   int
	Txi      []byte
	Errors   []string
}

// countIn returns the count that pattern captures in text, or 0 when it does not match.
func countIn(pattern *regexp.Regexp, text string) int {
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return 0
	}
	count, _ := strconv.Atoi(match[1])
	return count
}

func ParseTpc(file []byte) *TpcTexture {
	texture := &TpcTexture{}
	if len(file) < TpcHeaderSize {
		texture.Errors = []string{"shorter than header"}
		return texture
	}

	dataSize := int(binary.LittleEndian.Uint32(file[0:]))
	alpha := float64(math.Float32frombits(binary.LittleEndian.Uint32(file[4:])))
	width := int(binary.LittleEndian.Uint16(file[8:]))
	height := int(binary.LittleEndian.Uint16(file[10:]))
	enc := file[12]
	mips := int(file[13])
	for _, b := range file[14:TpcHeaderSize] {
		if b != 0 {
			texture.Errors = append(texture.Errors, "header padding not zero")
			break
		}
	}

	kinds := compressedKind
	if dataSize == 0 {
		kinds = uncompressedKind
	}
	kind := kinds[enc]
	if kind == "" {
		texture.Errors = append(texture.Errors, fmt.Sprintf("unknown encoding %d with dataSize %d", enc, dataSize))
		return texture
	}

	// The pixel data is a list of sub-images, each with its own mip chain. Each sub-image is
	// placed at (x, y) in the output. Three layouts exist:
	//   plain:     one image with `mips` levels.
	//   cube:      height = 6 * width; six square faces with `mips` levels each, stacked vertically.
	//   flipbook:  dataSize covers numx * numy animation frames (counts from the TXI trailer),
	//              each with a full mip chain down to 1x1. The header mip count is 1 and is ignored.
	chain := func(image SubImage) int {
		size := 0
		for level := 0; float64(level) < image.Levels; level++ {
			size += LevelSize(kind, max(1, image.Width>>level), max(1, image.Height>>level))
		}
		return size
	}

	layout := "plain"
	images := []SubImage{{Width: width, Height: height, Levels: float64(mips)}}
	if width > 0 && height == cubeFaces*width {
		layout = "cube"
		images = nil
		for face := 0; face < cubeFaces; face++ {
			images = append(images, SubImage{Y: face * width, Width: width, Height: width, Levels: float64(mips)})
		}
	} else if dataSize != 0 && dataSize != LevelSize(kind, width, height) && TpcHeaderSize+dataSize <= len(file) {
		txiText := Latin1(file[TpcHeaderSize+dataSize:])
		columns := countIn(numxPattern, txiText)
		rows := countIn(numyPattern, txiText)
		if columns > 0 && rows > 0 && width%columns == 0 && height%rows == 0 {
			frameWidth := width / columns
			frameHeight := height / rows
			levels := math.Log2(float64(max(frameWidth, frameHeight))) + 1
			layout = "flipbook"
			images = nil
			for j := 0; j < rows; j++ {
				for i := 0; i < columns; i++ {
					images = append(images, SubImage{
						X:      i * frameWidth,
						Y:      j * frameHeight,
						Width:  frameWidth,
						Height: frameHeight,
						Levels: levels,
					})
				}
			}
		}
	}

	// A recorded size of 16 for a single 8-byte DXT1 block is accepted: all such files end exactly after the mip chain.
	top := LevelSize(kind, images[0].Width, images[0].Height)
	if dataSize != 0 && dataSize != top && (dataSize != dxt5BlockSize || top != dxt1BlockSize) && layout != "flipbook" {
		texture.Errors = append(texture.Errors, fmt.Sprintf("dataSize %d matches no known layout", dataSize))
	}

	pixels := 0
	for i := range images {
		images[i].Offset = TpcHeaderSize + pixels
		pixels += chain(images[i])
	}
	if layout == "flipbook" && pixels != dataSize {
		texture.Errors = append(texture.Errors, fmt.Sprintf("flipbook frames total %d, dataSize %d", pixels, dataSize))
	}

	end := TpcHeaderSize + pixels
	if end > len(file) {
		texture.Errors = append(texture.Errors, fmt.Sprintf("pixel data ends at %d, past end of file %d", end, len(file)))
	}

	var txi []byte
	if end <= len(file) {
		txi = file[end:]
	}
	for _, b := range txi {
		control := b == 0 || b == '\t' || b == '\n' || b == '\r'
		if !control && (b < printableFirst || b > printableLast) {
			texture.Errors = append(texture.Errors, "trailer not ASCII")
			break
		}
	}

	texture.DataSize = dataSize
	texture.Alpha = alpha
	texture.Width = width
	texture.Height = height
	texture.Enc = int(enc)
	texture.Mips = mips
	texture.Kind = kind
	texture.Layout = layout
	texture.Images = images
	texture.Pixels = pixels
	texture.Txi = txi
	return texture
}

// expandRgb565 expands a 16-bit RGB 5:6:5 color to 8 bits per channel.
func expandRgb565(color int) [3]int {
	return [3]int{
		int(float64((color>>11&31)*255)/31 + 0.5),
		int(float64((color>>5&63)*255)/63 + 0.5),
		int(float64((color&31)*255)/31 + 0.5),
	}
}

// decodeImage decodes one image (width x height) of the given kind at file[offset] into RGBA, top row first.
func decodeImage(file []byte, offset int, kind string, width, height int) []byte {
	out := make([]byte, width*height*4)
	channels := bytesPerPixel[kind]
	if channels > 0 {
		for i := 0; i < width*height; i++ {
			src := offset + i*channels
			if channels == 1 {
				out[4*i] = file[src]
				out[4*i+1] = file[src]
				out[4*i+2] = file[src]
				out[4*i+3] = opaque
				continue
			}

			copy(out[4*i:], file[src:src+3])
			out[4*i+3] = opaque
			if channels == 4 {
				out[4*i+3] = file[src+3]
			}
		}
		return out
	}

	blockSize := dxt5BlockSize
	if kind == "dxt1" {
		blockSize = dxt1BlockSize
	}
	blocksWide := blocksAcross(width)
	blocksHigh := blocksAcross(height)
	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			block := offset + (blockY*blocksWide+blockX)*blockSize

			var alphas [blockTexels]int
			for i := range alphas {
				alphas[i] = opaque
			}
			if kind == "dxt5" {
				alpha0, alpha1 := int(file[block]), int(file[block+1])
				alphaTable := []int{alpha0, alpha1}
				if alpha0 > alpha1 {
					for i := 1; i < 7; i++ {
						alphaTable = append(alphaTable, int(float64((7-i)*alpha0+i*alpha1)/7+0.5))
					}
				} else {
					for i := 1; i < 5; i++ {
						alphaTable = append(alphaTable, int(float64((5-i)*alpha0+i*alpha1)/5+0.5))
					}
					alphaTable = append(alphaTable, 0, opaque)
				}

				// 16 indices of 3 bits each, stored in 6 little-endian bytes.
				var bits uint64
				for i := 5; i >= 0; i-- {
					bits = bits<<8 | uint64(file[block+2+i])
				}
				for i := 0; i < blockTexels; i++ {
					alphas[i] = alphaTable[bits>>(3*i)&7]
				}
				block += dxt1BlockSize
			}

			raw0 := int(binary.LittleEndian.Uint16(file[block:]))
			raw1 := int(binary.LittleEndian.Uint16(file[block+2:]))
			color0, color1 := expandRgb565(raw0), expandRgb565(raw1)
			palette := [4][3]int{color0, color1}
			if raw0 > raw1 || kind == "dxt5" {
				for i := 0; i < 3; i++ {
					palette[2][i] = int(float64(2*color0[i]+color1[i])/3 + 0.5)
					palette[3][i] = int(float64(color0[i]+2*color1[i])/3 + 0.5)
				}
			} else {
				for i := 0; i < 3; i++ {
					palette[2][i] = (color0[i] + color1[i]) >> 1
				}
			}

			indices := binary.LittleEndian.Uint32(file[block+4:])
			for i := 0; i < blockTexels; i++ {
				x := blockX*blockPixels + i%blockPixels
				y := blockY*blockPixels + i/blockPixels
				if x >= width || y >= height {
					continue
				}

				colorIndex := indices >> (2 * i) & 3
				dst := 4 * (y*width + x)
				out[dst] = byte(palette[colorIndex][0])
				out[dst+1] = byte(palette[colorIndex][1])
				out[dst+2] = byte(palette[colorIndex][2])
				out[dst+3] = byte(alphas[i])
				if kind == "dxt1" && raw0 <= raw1 && colorIndex == 3 {
					out[dst+3] = 0
				}
			}
		}
	}

	return out
}

// TpcImage is a decoded texture as RGBA rows, top row first.
type TpcImage struct {
	Width  int
	Height int
	Rgba   []byte
}

// DecodeTpc returns the top mip level of every sub-image, placed at its (x, y), or nil when the texture has errors.
// TPC stores the bottom row of each image first (texture coordinate v = 0), so rows are reversed.
// Sub-images keep reading order: flipbook frame 0 is at the top left and cube face 0 is at the top.
func DecodeTpc(file []byte, texture *TpcTexture) *TpcImage {
	if len(texture.Errors) > 0 {
		return nil
	}

	rgba := make([]byte, texture.Width*texture.Height*4)
	for _, image := range texture.Images {
		pix := decodeImage(file, image.Offset, texture.Kind, image.Width, image.Height)
		for y := 0; y < image.Height; y++ {
			dst := 4 * ((image.Y+image.Height-1-y)*texture.Width + image.X)
			copy(rgba[dst:], pix[4*y*image.Width:4*(y+1)*image.Width])
		}
	}

	return &TpcImage{Width: texture.Width, Height: texture.Height, Rgba: rgba}
}

func ReportTpc(label string, texture *TpcTexture) string {
	if texture.Kind == "" {
		return label + " FAIL: " + strings.Join(texture.Errors, "; ")
	}

	return fmt.Sprintf("%s %dx%d %s mips=%d layout=%s alpha=%.4f txi=%d %s",
		label, texture.Width, texture.Height, texture.Kind, texture.Mips, texture.Layout,
		texture.Alpha, len(texture.Txi), Verdict(texture.Errors))
}
