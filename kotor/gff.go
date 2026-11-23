package kotor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// Generic tree file (GFF), used by GUI layouts and by most module resources such as "ARE V3.2" and "UTC V3.2". 56-byte header: char[4] fileType, char[4] version,
// then six u32 pairs: structs (off, count), fields (off, count), labels (off, count),
// fieldData (off, bytes), fieldIndices (off, bytes), listIndices (off, bytes).
// Struct 12 bytes: u32 type, u32 data, u32 fieldCount. Field 12 bytes: u32 type, u32 labelIndex, u32 value.
// Every field and every non-root struct must be used exactly once, and no byte of the three data regions may
// be used twice. Unused bytes are counted, because some module files contain them.
const (
	gffHeaderSize = 56
	gffStructSize = 12
	gffFieldSize  = 12
	gffLabelSize  = 16
)

// Field type codes. Types 0, 2, 3, 4, 5 and 8 store their value in the field itself; the others point into
// the field data, the struct table or the list indices. Other codes have not been seen.
const (
	GffU8          = 0
	GffU16         = 2
	GffI16         = 3
	GffU32         = 4
	GffI32         = 5
	GffF32         = 8
	GffString      = 10
	GffResref      = 11
	GffLocString   = 12
	GffBinary      = 13
	GffStruct      = 14
	GffList        = 15
	GffOrientation = 16
	GffVector      = 17
)

// A localized string (type 12) is a u32 byte count of the rest, a u32 string number into dialog.tlk
// (0xFFFFFFFF for none), a u32 substring count, and per substring a u32 substring id, a u32 length and the
// characters.
const gffLocStringHeader = 12

// GffSubstring is one substring of a localized string.
type GffSubstring struct {
	ID   uint32
	Text string
}

// GffFieldValue holds a field's value. Only the members for the field's type are set: Number for the
// integer and f32 types, Text for string and resref, Number (string number) and Substrings for localized
// string, Bytes for binary, Struct, List, or Vector (three components for vector, four for orientation).
type GffFieldValue struct {
	Number     float64
	Text       string
	Substrings []GffSubstring
	Bytes      []byte
	Struct     *GffStructValue
	List       []*GffStructValue
	Vector     []float64
}

type GffField struct {
	Label string
	Type  uint32
	Value GffFieldValue
}

type GffStructValue struct {
	StructType uint32
	Fields     []GffField
}

// GffCounts describes a parsed file. UnusedBytes counts the bytes of the field data ("data"), field index
// ("idx") and list index ("list") regions that nothing refers to, by region.
type GffCounts struct {
	Structs      int
	Fields       int
	Labels       int
	LabelsUnused int
	UnusedBytes  map[string]int
	Types        map[uint32]int
}

type Gff struct {
	FileType string
	Version  string
	Root     *GffStructValue
	Counts   GffCounts
}

// ReadGff parses a GFF file. It returns an error text when the header regions do not chain to the end of
// the file, when a reference leaves its region or table, when a byte, field or struct is used twice, or
// when a field or struct is never used. Bytes that are never used are only counted.
func ReadGff(data []byte) (*Gff, string) {
	if len(data) < gffHeaderSize {
		return nil, "short header"
	}

	header := make([]int, 0, (gffHeaderSize-signatureLength)/4)
	for i := signatureLength; i < gffHeaderSize; i += 4 {
		header = append(header, int(u32(data, i)))
	}

	structOffset, structCount := header[0], header[1]
	fieldOffset, fieldCount := header[2], header[3]
	labelOffset, labelCount := header[4], header[5]
	fieldDataOffset, fieldDataLength := header[6], header[7]
	fieldIndicesOffset, fieldIndicesLength := header[8], header[9]
	listIndicesOffset, listIndicesLength := header[10], header[11]
	if structOffset != gffHeaderSize ||
		fieldOffset != structOffset+gffStructSize*structCount ||
		labelOffset != fieldOffset+gffFieldSize*fieldCount ||
		fieldDataOffset != labelOffset+gffLabelSize*labelCount ||
		fieldIndicesOffset != fieldDataOffset+fieldDataLength ||
		listIndicesOffset != fieldIndicesOffset+fieldIndicesLength ||
		listIndicesOffset+listIndicesLength != len(data) {
		return nil, "header regions do not chain to end of file"
	}

	// The first problem found is kept. Every reader returns as soon as there is one, so no read leaves
	// the data.
	problem := ""
	fail := func(message string) {
		if problem == "" {
			problem = message
		}
	}

	used := map[string][]uint8{
		"data": make([]uint8, fieldDataLength),
		"idx":  make([]uint8, fieldIndicesLength),
		"list": make([]uint8, listIndicesLength),
	}
	regionLength := map[string]int{"data": fieldDataLength, "idx": fieldIndicesLength, "list": listIndicesLength}
	mark := func(region string, offset, count int) bool {
		if problem != "" {
			return false
		}
		if offset < 0 || count < 0 || offset+count > regionLength[region] {
			fail(fmt.Sprintf("%s range %d+%d past %d", region, offset, count, regionLength[region]))
			return false
		}

		for i := offset; i < offset+count; i++ {
			used[region][i]++
			if used[region][i] > 1 {
				fail(fmt.Sprintf("%s byte %d used twice", region, i))
				return false
			}
		}
		return true
	}

	fieldUsed := make([]uint8, fieldCount)
	structUsed := make([]uint8, structCount)

	labels := make([]string, labelCount)
	for i := range labels {
		label := labelOffset + gffLabelSize*i
		labels[i] = CString(data[label : label+gffLabelSize])
	}
	labelUsed := make([]uint8, labelCount)
	types := map[uint32]int{}

	readFloat := func(offset int) float64 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset:])))
	}

	var readStruct func(structIndex int) *GffStructValue
	readField := func(fieldIndex int) GffField {
		if fieldIndex >= fieldCount {
			fail(fmt.Sprintf("field index %d >= %d", fieldIndex, fieldCount))
			return GffField{}
		}
		fieldUsed[fieldIndex]++
		if fieldUsed[fieldIndex] > 1 {
			fail(fmt.Sprintf("field %d used twice", fieldIndex))
			return GffField{}
		}

		entry := fieldOffset + gffFieldSize*fieldIndex
		fieldType := u32(data, entry)
		labelIndex := int(u32(data, entry+4))
		raw := u32(data, entry+8)
		offset := int(raw)
		if labelIndex >= labelCount {
			fail(fmt.Sprintf("label index %d >= %d", labelIndex, labelCount))
			return GffField{}
		}

		labelUsed[labelIndex] = 1
		types[fieldType]++

		var value GffFieldValue
		switch fieldType {
		case GffU8:
			if raw > 0xff {
				fail(fmt.Sprintf("u8 field %d value %d", fieldIndex, raw))
			}
			value.Number = float64(raw)

		case GffU16:
			if raw > 0xffff {
				fail(fmt.Sprintf("u16 field %d value %d", fieldIndex, raw))
			}
			value.Number = float64(raw)

		case GffI16:
			if raw > 0xffff {
				fail(fmt.Sprintf("i16 field %d value %d", fieldIndex, raw))
			}
			value.Number = float64(int16(raw))

		case GffU32:
			value.Number = float64(raw)

		case GffI32:
			value.Number = float64(int32(raw))

		case GffF32:
			value.Number = readFloat(entry + 8)

		case GffString, GffBinary:
			if !mark("data", offset, 4) {
				break
			}
			length := int(u32(data, fieldDataOffset+offset))
			if !mark("data", offset+4, length) {
				break
			}
			start := fieldDataOffset + offset + 4
			if fieldType == GffString {
				value.Text = Latin1(data[start : start+length])
			} else {
				value.Bytes = data[start : start+length]
			}

		case GffResref:
			if !mark("data", offset, 1) {
				break
			}
			length := int(data[fieldDataOffset+offset])
			if length > ResrefLength {
				fail(fmt.Sprintf("resref length %d", length))
				break
			}
			if !mark("data", offset+1, length) {
				break
			}
			start := fieldDataOffset + offset + 1
			value.Text = Latin1(data[start : start+length])

		case GffLocString:
			if !mark("data", offset, gffLocStringHeader) {
				break
			}
			start := fieldDataOffset + offset
			size := int(u32(data, start))
			value.Number = float64(u32(data, start+4))
			count := int(u32(data, start+8))
			position := offset + gffLocStringHeader
			for range count {
				if !mark("data", position, 8) {
					break
				}
				id := u32(data, fieldDataOffset+position)
				length := int(u32(data, fieldDataOffset+position+4))
				if !mark("data", position+8, length) {
					break
				}
				text := fieldDataOffset + position + 8
				value.Substrings = append(value.Substrings, GffSubstring{ID: id, Text: Latin1(data[text : text+length])})
				position += 8 + length
			}
			if problem == "" && position != offset+4+size {
				fail(fmt.Sprintf("localized string field %d size %d does not match its content", fieldIndex, size))
			}

		case GffStruct:
			value.Struct = readStruct(offset)

		case GffList:
			if !mark("list", offset, 4) {
				break
			}
			list := listIndicesOffset + offset
			count := int(u32(data, list))
			if !mark("list", offset+4, 4*count) {
				break
			}
			value.List = make([]*GffStructValue, 0, count)
			for i := range count {
				value.List = append(value.List, readStruct(int(u32(data, list+4+4*i))))
			}

		case GffVector, GffOrientation:
			components := 3
			if fieldType == GffOrientation {
				components = 4
			}
			if !mark("data", offset, 4*components) {
				break
			}
			for component := range components {
				value.Vector = append(value.Vector, readFloat(fieldDataOffset+offset+4*component))
			}

		default:
			fail(fmt.Sprintf("unknown field type %d (field %d)", fieldType, fieldIndex))
		}

		return GffField{Label: labels[labelIndex], Type: fieldType, Value: value}
	}

	readStruct = func(structIndex int) *GffStructValue {
		if problem != "" {
			return &GffStructValue{}
		}
		if structIndex >= structCount {
			fail(fmt.Sprintf("struct index %d >= %d", structIndex, structCount))
			return &GffStructValue{}
		}
		structUsed[structIndex]++
		if structUsed[structIndex] > 1 {
			fail(fmt.Sprintf("struct %d used twice", structIndex))
			return &GffStructValue{}
		}

		entry := structOffset + gffStructSize*structIndex
		structType := u32(data, entry)
		first := int(u32(data, entry+4))
		count := int(u32(data, entry+8))

		var ids []int
		if count == 1 {
			ids = []int{first}
		} else if count > 1 && mark("idx", first, 4*count) {
			for i := range count {
				ids = append(ids, int(u32(data, fieldIndicesOffset+first+4*i)))
			}
		}

		fields := make([]GffField, 0, len(ids))
		for _, id := range ids {
			if problem != "" {
				break
			}
			fields = append(fields, readField(id))
		}
		return &GffStructValue{StructType: structType, Fields: fields}
	}

	root := readStruct(0)
	if problem != "" {
		return nil, problem
	}

	for i, count := range fieldUsed {
		if count == 0 {
			return nil, fmt.Sprintf("field %d never used", i)
		}
	}
	for i, count := range structUsed {
		if count == 0 {
			return nil, fmt.Sprintf("struct %d never used", i)
		}
	}

	unusedBytes := map[string]int{}
	for region, counts := range used {
		unusedBytes[region] = bytes.Count(counts, []byte{0})
	}

	labelsUsed := 0
	for _, count := range labelUsed {
		labelsUsed += int(count)
	}

	return &Gff{
		FileType: Latin1(data[0:4]),
		Version:  Latin1(data[4:8]),
		Root:     root,
		Counts: GffCounts{
			Structs:      structCount,
			Fields:       fieldCount,
			Labels:       labelCount,
			LabelsUnused: labelCount - labelsUsed,
			UnusedBytes:  unusedBytes,
			Types:        types,
		},
	}, ""
}
