package kotor

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Generic tree file ("GUI V3.2" and possibly others). 56-byte header: char[4] fileType, char[4] version,
// then six u32 pairs: structs (off, count), fields (off, count), labels (off, count),
// fieldData (off, bytes), fieldIndices (off, bytes), listIndices (off, bytes).
// Struct 12 bytes: u32 type, u32 data, u32 fieldCount. Field 12 bytes: u32 type, u32 labelIndex, u32 value.
// Strict: every byte of the three data regions, every field and every non-root struct must be used exactly once.
const (
	gffHeaderSize = 56
	gffStructSize = 12
	gffFieldSize  = 12
	gffLabelSize  = 16
)

// Field type codes. Types 0, 4, 5 and 8 store their value in the field itself; the others point into
// the field data, the struct table or the list indices. Other codes have not been seen.
const (
	GffU8     = 0
	GffU32    = 4
	GffI32    = 5
	GffF32    = 8
	GffString = 10
	GffResref = 11
	GffStruct = 14
	GffList   = 15
	GffVector = 17
)

// GffFieldValue holds a field's value. Only the member for the field's type is set: Number for u8, u32,
// i32 and f32, Text for string and resref, Struct, List, or Vector.
type GffFieldValue struct {
	Number float64
	Text   string
	Struct *GffStructValue
	List   []*GffStructValue
	Vector [3]float64
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

type GffCounts struct {
	Structs      int
	Fields       int
	Labels       int
	LabelsUnused int
	Types        map[uint32]int
}

type Gff struct {
	FileType string
	Version  string
	Root     *GffStructValue
	Counts   GffCounts
}

// ReadGff parses a GFF file and ends the process when the file breaks any of the strict rules above.
func ReadGff(data []byte, name string) *Gff {
	fail := func(message string) {
		Fatal(name + ": " + message)
	}

	if len(data) < gffHeaderSize {
		fail("short header")
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
		fail("header regions do not chain to end of file")
	}

	used := map[string][]uint8{
		"data": make([]uint8, fieldDataLength),
		"idx":  make([]uint8, fieldIndicesLength),
		"list": make([]uint8, listIndicesLength),
	}
	mark := func(region string, offset, count, length int) {
		if offset+count > length {
			fail(fmt.Sprintf("%s range %d+%d past %d", region, offset, count, length))
		}

		for i := offset; i < offset+count; i++ {
			used[region][i]++
			if used[region][i] > 1 {
				fail(fmt.Sprintf("%s byte %d used twice", region, i))
			}
		}
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
		}
		fieldUsed[fieldIndex]++
		if fieldUsed[fieldIndex] > 1 {
			fail(fmt.Sprintf("field %d used twice", fieldIndex))
		}

		entry := fieldOffset + gffFieldSize*fieldIndex
		fieldType := u32(data, entry)
		labelIndex := int(u32(data, entry+4))
		raw := u32(data, entry+8)
		if labelIndex >= labelCount {
			fail(fmt.Sprintf("label index %d >= %d", labelIndex, labelCount))
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

		case GffU32:
			value.Number = float64(raw)

		case GffI32:
			value.Number = float64(int32(raw))

		case GffF32:
			value.Number = readFloat(entry + 8)

		case GffString:
			mark("data", int(raw), 4, fieldDataLength)
			start := fieldDataOffset + int(raw) + 4
			length := int(u32(data, fieldDataOffset+int(raw)))
			mark("data", int(raw)+4, length, fieldDataLength)
			value.Text = Latin1(data[start : start+length])

		case GffResref:
			mark("data", int(raw), 1, fieldDataLength)
			start := fieldDataOffset + int(raw) + 1
			length := int(data[fieldDataOffset+int(raw)])
			if length > ResrefLength {
				fail(fmt.Sprintf("resref length %d", length))
			}
			mark("data", int(raw)+1, length, fieldDataLength)
			value.Text = Latin1(data[start : start+length])

		case GffStruct:
			value.Struct = readStruct(int(raw))

		case GffList:
			mark("list", int(raw), 4, listIndicesLength)
			list := listIndicesOffset + int(raw)
			count := int(u32(data, list))
			mark("list", int(raw)+4, 4*count, listIndicesLength)
			value.List = make([]*GffStructValue, 0, count)
			for i := range count {
				value.List = append(value.List, readStruct(int(u32(data, list+4+4*i))))
			}

		case GffVector:
			mark("data", int(raw), 12, fieldDataLength)
			for component := range value.Vector {
				value.Vector[component] = readFloat(fieldDataOffset + int(raw) + 4*component)
			}

		default:
			fail(fmt.Sprintf("unknown field type %d (field %d)", fieldType, fieldIndex))
		}

		return GffField{Label: labels[labelIndex], Type: fieldType, Value: value}
	}

	readStruct = func(structIndex int) *GffStructValue {
		if structIndex >= structCount {
			fail(fmt.Sprintf("struct index %d >= %d", structIndex, structCount))
		}
		structUsed[structIndex]++
		if structUsed[structIndex] > 1 {
			fail(fmt.Sprintf("struct %d used twice", structIndex))
		}

		entry := structOffset + gffStructSize*structIndex
		structType := u32(data, entry)
		first := int(u32(data, entry+4))
		count := int(u32(data, entry+8))

		var ids []int
		if count == 1 {
			ids = []int{first}
		} else if count > 1 {
			mark("idx", first, 4*count, fieldIndicesLength)
			for i := range count {
				ids = append(ids, int(u32(data, fieldIndicesOffset+first+4*i)))
			}
		}

		fields := make([]GffField, 0, len(ids))
		for _, id := range ids {
			fields = append(fields, readField(id))
		}
		return &GffStructValue{StructType: structType, Fields: fields}
	}

	root := readStruct(0)

	for _, region := range []string{"data", "idx", "list"} {
		for i, count := range used[region] {
			if count == 0 {
				fail(fmt.Sprintf("%s byte %d never used", region, i))
			}
		}
	}
	for i, count := range fieldUsed {
		if count == 0 {
			fail(fmt.Sprintf("field %d never used", i))
		}
	}
	for i, count := range structUsed {
		if count == 0 {
			fail(fmt.Sprintf("struct %d never used", i))
		}
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
			Types:        types,
		},
	}
}
