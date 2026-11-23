// This file reads TGA (Truevision) images. Layout: see FILE_FORMATS.md.
// It supports the layouts that occur in the game: uncompressed true color (image type 2) at 24 or 32 bits
// per pixel, and uncompressed grayscale (image type 3) at 8 bits per pixel, with no color map.
// An 18-byte header and an optional image ID precede the pixels. An optional TGA 2.0 extension area
// and 26-byte footer follow them.

package kotor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
)

const tgaHeaderSize = 18

// The 26-byte footer holds u32 extension area offset, u32 developer area offset and this 18-byte signature.
const (
	footerSize      = 26
	footerSignature = "TRUEVISION-XFILE.\x00"
)

const (
	imageTrueColor = 2
	imageGrayscale = 3
)

// "image type/bits per pixel" pairs that occur in the game.
var supportedKinds = []string{
	fmt.Sprintf("%d/24", imageTrueColor),
	fmt.Sprintf("%d/32", imageTrueColor),
	fmt.Sprintf("%d/8", imageGrayscale),
}

// Image descriptor bits: alpha bit count in bits 0 to 3, right-to-left order in bit 4, top-down order in bit 5.
const (
	descriptorAlphaBits   = 0x0f
	descriptorRightToLeft = 0x10
	descriptorTopDown     = 0x20
)

// TgaHeader holds the parsed header. Complete is false when the file is shorter than the header, in which
// case only Errors is set. ExtSize and ExtSizeField are set only when Trailer is "extension+footer".
type TgaHeader struct {
	Complete     bool
	Errors       []string
	IdLength     int
	ColorMapType int
	ImageType    int
	Width        int
	Height       int
	Bpp          int
	Descriptor   int
	AlphaBits    int
	TopDown      bool
	RightToLeft  bool
	Channels     int
	PixelOffset  int
	PixelEnd     int
	Trailer      string
	ExtSize      int
	ExtSizeField int
}

// ParseTga reads the header and checks that the header, pixels and optional trailer consume the file exactly.
func ParseTga(file []byte) *TgaHeader {
	header := &TgaHeader{}
	if len(file) < tgaHeaderSize {
		header.Errors = append(header.Errors,
			fmt.Sprintf("file is %d bytes, shorter than the %d-byte header", len(file), tgaHeaderSize))
		return header
	}

	header.Complete = true
	header.IdLength = int(file[0])
	header.ColorMapType = int(file[1])
	header.ImageType = int(file[2])
	header.Width = int(binary.LittleEndian.Uint16(file[12:]))
	header.Height = int(binary.LittleEndian.Uint16(file[14:]))
	header.Bpp = int(file[16])
	header.Descriptor = int(file[17])
	header.AlphaBits = header.Descriptor & descriptorAlphaBits
	header.TopDown = header.Descriptor&descriptorTopDown != 0
	header.RightToLeft = header.Descriptor&descriptorRightToLeft != 0

	kind := fmt.Sprintf("%d/%d", header.ImageType, header.Bpp)
	if header.ColorMapType != 0 {
		header.Errors = append(header.Errors, fmt.Sprintf("color map type %d", header.ColorMapType))
	}
	if !slices.Contains(supportedKinds, kind) {
		header.Errors = append(header.Errors,
			fmt.Sprintf("image type %d at %d bits per pixel", header.ImageType, header.Bpp))
	}
	if header.RightToLeft {
		header.Errors = append(header.Errors, "right-to-left pixel order")
	}
	if header.Width == 0 || header.Height == 0 {
		header.Errors = append(header.Errors, "zero width or height")
	}

	// Unsupported bit depths that are not a multiple of 8 are rejected above; the pixel size is then
	// rounded down only to keep the bounds checks meaningful.
	header.Channels = header.Bpp / 8
	header.PixelOffset = tgaHeaderSize + header.IdLength
	header.PixelEnd = header.PixelOffset + header.Width*header.Height*header.Channels
	header.Trailer = "none"
	if header.PixelEnd > len(file) {
		header.Errors = append(header.Errors,
			fmt.Sprintf("pixels end at %d, past end of file %d", header.PixelEnd, len(file)))
		return header
	}

	rest := len(file) - header.PixelEnd
	if rest == 0 {
		return header
	}
	if rest < footerSize || !bytes.HasSuffix(file, []byte(footerSignature)) {
		header.Errors = append(header.Errors,
			fmt.Sprintf("%d bytes after the pixels without a TGA 2.0 footer", rest))
		return header
	}

	footer := len(file) - footerSize
	extOffset := int(binary.LittleEndian.Uint32(file[footer:]))
	devOffset := int(binary.LittleEndian.Uint32(file[footer+4:]))
	if devOffset != 0 {
		header.Errors = append(header.Errors, fmt.Sprintf("developer area at %d", devOffset))
	}
	if rest == footerSize {
		header.Trailer = "footer"
		if extOffset != 0 {
			header.Errors = append(header.Errors,
				fmt.Sprintf("extension area at %d, but no room for one", extOffset))
		}
		return header
	}

	header.Trailer = "extension+footer"
	header.ExtSize = footer - header.PixelEnd
	header.ExtSizeField = int(binary.LittleEndian.Uint16(file[header.PixelEnd:]))
	if extOffset != header.PixelEnd {
		header.Errors = append(header.Errors,
			fmt.Sprintf("extension area at %d, pixels end at %d", extOffset, header.PixelEnd))
	}
	return header
}

// TgaImage holds the pixels top row first. Pix has Channels bytes per pixel in PNG order: gray, RGB or RGBA.
type TgaImage struct {
	Width    int
	Height   int
	Channels int
	Pix      []byte
}

// DecodeTga returns nil when the header has errors.
func DecodeTga(file []byte, header *TgaHeader) *TgaImage {
	if len(header.Errors) > 0 {
		return nil
	}

	width, height, channels := header.Width, header.Height, header.Channels
	stride := width * channels
	pix := make([]byte, stride*height)

	for y := 0; y < height; y++ {
		sourceRow := height - 1 - y
		if header.TopDown {
			sourceRow = y
		}
		srcRow := header.PixelOffset + sourceRow*stride

		for x := 0; x < width; x++ {
			src := srcRow + x*channels
			dst := y*stride + x*channels
			if channels == 1 {
				pix[dst] = file[src]
				continue
			}

			// TGA stores true color pixels as blue, green, red and, at 32 bits, alpha.
			pix[dst] = file[src+2]
			pix[dst+1] = file[src+1]
			pix[dst+2] = file[src]
			if channels == 4 {
				pix[dst+3] = file[src+3]
			}
		}
	}

	return &TgaImage{Width: width, Height: height, Channels: channels, Pix: pix}
}

func ReportTga(label string, header *TgaHeader) string {
	if !header.Complete {
		return label + " " + Verdict(header.Errors...)
	}

	origin := "bottom"
	if header.TopDown {
		origin = "top"
	}

	ext := ""
	if header.Trailer == "extension+footer" {
		ext = fmt.Sprintf(" ext=%d/%d", header.ExtSize, header.ExtSizeField)
	}

	return fmt.Sprintf("%s %dx%d type=%d bpp=%d alphaBits=%d origin=%s trailer=%s%s %s",
		label, header.Width, header.Height, header.ImageType, header.Bpp, header.AlphaBits,
		origin, header.Trailer, ext, Verdict(header.Errors...))
}
