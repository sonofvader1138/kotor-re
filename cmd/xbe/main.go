// Command xbe reads an Xbox executable (XBE), prints its header, sections and kernel imports, and writes a flat
// memory image in which every section lies at its virtual address minus the base address. A disassembler such
// as objdump can then read the image as raw 32-bit x86 code loaded at the base address.
//
// usage: go run ./cmd/xbe [-in dump/xbox/default.xbe] [-out temp/xbe/default.img]
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"re/kotor"
)

const (
	xbeMagic = "XBEH"

	// The entry point and the kernel thunk address are stored XOR-masked. A retail build uses the first key of
	// each pair and a debug build the second.
	entryKeyRetail  = 0xA8FC57AB
	entryKeyDebug   = 0x94859D4B
	thunkKeyRetail  = 0x5B6D40B6
	thunkKeyDebug   = 0xEFB1F152
	sectionHeadSize = 56

	// A kernel import is an ordinal with the high bit set.
	ordinalFlag = 0x80000000
)

func u32(data []byte, offset uint32) uint32 {
	return binary.LittleEndian.Uint32(data[offset:])
}

func cString(data []byte, offset uint32) string {
	end := offset
	for int(end) < len(data) && data[end] != 0 {
		end++
	}
	return string(data[offset:end])
}

func main() {
	in := flag.String("in", filepath.Join(kotor.Root, "dump", "xbox", "default.xbe"), "XBE file to read")
	out := flag.String("out", filepath.Join(kotor.Root, "temp", "xbe", "default.img"), "flat memory image to write")
	flag.Parse()

	data := kotor.ReadFile(*in)
	if len(data) < 0x178 || string(data[:4]) != xbeMagic {
		kotor.Fatal(*in + " is not an XBE file")
	}

	base := u32(data, 0x104)
	headerSize := u32(data, 0x108)
	imageSize := u32(data, 0x10C)
	sectionCount := u32(data, 0x11C)
	sectionHeaders := u32(data, 0x120)

	// The header is mapped at the base address, so a header address minus the base is a file offset.
	fileOffset := func(address uint32) uint32 {
		return address - base
	}

	// The key that yields an entry point inside the image is the right one.
	entry := u32(data, 0x128) ^ entryKeyRetail
	thunk := u32(data, 0x158) ^ thunkKeyRetail
	build := "retail"
	if entry < base || entry >= base+imageSize {
		entry = u32(data, 0x128) ^ entryKeyDebug
		thunk = u32(data, 0x158) ^ thunkKeyDebug
		build = "debug"
	}

	fmt.Printf("base 0x%08X, header size 0x%X, image size 0x%X, %s build\n", base, headerSize, imageSize, build)
	fmt.Printf("entry point 0x%08X, kernel thunk table 0x%08X\n", entry, thunk)
	fmt.Printf("debug path %s\n", cString(data, fileOffset(u32(data, 0x14C))))

	image := make([]byte, imageSize)
	copy(image, data[:headerSize])

	fmt.Printf("%-10s %-10s %-10s %-10s %-10s %s\n", "section", "address", "size", "file", "raw size", "flags")
	for i := uint32(0); i < sectionCount; i++ {
		header := fileOffset(sectionHeaders) + i*sectionHeadSize
		flags := u32(data, header)
		address := u32(data, header+0x04)
		size := u32(data, header+0x08)
		raw := u32(data, header+0x0C)
		rawSize := u32(data, header+0x10)
		name := cString(data, fileOffset(u32(data, header+0x14)))

		if raw+rawSize > uint32(len(data)) || address-base+rawSize > imageSize {
			kotor.Fatal(fmt.Sprintf("section %s lies outside the file or the image", name))
		}
		copy(image[address-base:], data[raw:raw+rawSize])

		fmt.Printf("%-10s 0x%08X 0x%08X 0x%08X 0x%08X 0x%X\n", name, address, size, raw, rawSize, flags)
	}

	// The thunk table holds one kernel ordinal per import and ends with 0. The loader replaces each entry with
	// the address of the kernel routine, so code calls through these addresses.
	fmt.Println("kernel imports (thunk address, ordinal):")
	for address := thunk; ; address += 4 {
		value := u32(image, address-base)
		if value == 0 {
			break
		}
		fmt.Printf("0x%08X %d\n", address, value&^ordinalFlag)
	}

	kotor.MakeDir(filepath.Dir(*out))
	if err := os.WriteFile(*out, image, 0o644); err != nil {
		kotor.Fatal(err.Error())
	}
	fmt.Printf("wrote %s\n", *out)
}
