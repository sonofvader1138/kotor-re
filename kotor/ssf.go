// SSF V1.1: a sound set, the voice lines of one creature type for game events such as battle cries.
// 12-byte header: char[8] signature, u32 offset of the slot table (always 12). Then one u32 string number
// of dialog.tlk per slot up to the end of the file, 0xFFFFFFFF for an empty slot. Files have 40 or 49 slots.
package kotor

import (
	"fmt"
)

const (
	ssfSignature  = "SSF V1.1"
	SsfHeaderSize = 12
	ssfSlotSize   = 4
)

// SsfEmpty is the string number of an empty slot.
const SsfEmpty = 0xffffffff

type Ssf struct {
	Slots  []uint32
	Errors []string
}

// ParseSsf reads a sound set. It requires the slot table to start right after the header and to end
// exactly at the end of the file.
func ParseSsf(data []byte) *Ssf {
	ssf := &Ssf{}
	if len(data) < SsfHeaderSize || string(data[:signatureLength]) != ssfSignature {
		ssf.Errors = append(ssf.Errors, "not SSF V1.1")
		return ssf
	}
	if offset := u32(data, 8); offset != SsfHeaderSize {
		ssf.Errors = append(ssf.Errors, fmt.Sprintf("slot table at %d", offset))
		return ssf
	}
	if (len(data)-SsfHeaderSize)%ssfSlotSize != 0 {
		ssf.Errors = append(ssf.Errors, fmt.Sprintf("%d bytes after the header", len(data)-SsfHeaderSize))
		return ssf
	}

	for offset := SsfHeaderSize; offset < len(data); offset += ssfSlotSize {
		ssf.Slots = append(ssf.Slots, u32(data, offset))
	}
	return ssf
}

func ReportSsf(label string, ssf *Ssf) string {
	used := 0
	for _, number := range ssf.Slots {
		if number != SsfEmpty {
			used++
		}
	}
	return fmt.Sprintf("%s slots=%d used=%d %s", label, len(ssf.Slots), used, Verdict(ssf.Errors...))
}
