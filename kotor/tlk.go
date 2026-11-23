// TLK V3.0: the string table dialog.tlk, which localized strings in GFF files refer to by string number.
// 20-byte header: char[8] signature, u32 language, u32 entry count, u32 offset of the string data.
// Then one 40-byte entry per string: u32 flags, char[16] sound resref, u32 volume variance, u32 pitch
// variance, u32 offset into the string data, u32 length, f32 sound length.
package kotor

import (
	"fmt"
)

const (
	tlkSignature  = "TLK V3.0"
	tlkHeaderSize = 20
	tlkEntrySize  = 40
)

// Bit 0 of the entry flags is set exactly when the entry has text.
const tlkHasText = 0x1

type TlkEntry struct {
	Flags uint32
	Sound string
	Text  string
}

type Tlk struct {
	Language uint32
	Entries  []TlkEntry
}

// ReadTlk parses a TLK file. It returns an error text unless the entry table ends exactly at the string
// data, every string lies inside the file, and the strings cover the string data in table order with no
// gap or overlap.
func ReadTlk(data []byte) (*Tlk, string) {
	if len(data) < tlkHeaderSize || string(data[:signatureLength]) != tlkSignature {
		return nil, "not TLK V3.0"
	}

	count := int(u32(data, 0x0c))
	stringOffset := int(u32(data, 0x10))
	if stringOffset != tlkHeaderSize+tlkEntrySize*count || stringOffset > len(data) {
		return nil, "entry table does not end at the string data"
	}

	tlk := &Tlk{Language: u32(data, 0x08), Entries: make([]TlkEntry, count)}
	position := stringOffset
	for i := range count {
		entry := tlkHeaderSize + tlkEntrySize*i
		flags := u32(data, entry)
		start := stringOffset + int(u32(data, entry+28))
		length := int(u32(data, entry+32))
		if length == 0 {
			start = position
		}
		if start != position || start+length > len(data) {
			return nil, fmt.Sprintf("string %d at %d+%d is misplaced", i, start, length)
		}
		if (flags&tlkHasText != 0) != (length > 0) {
			return nil, fmt.Sprintf("string %d has flags %#x and length %d", i, flags, length)
		}

		tlk.Entries[i] = TlkEntry{
			Flags: flags,
			Sound: CString(data[entry+4 : entry+4+ResrefLength]),
			Text:  Latin1(data[start : start+length]),
		}
		position = start + length
	}

	if position != len(data) {
		return nil, "string data does not end at end of file"
	}
	return tlk, ""
}
