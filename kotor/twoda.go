// 2DA V2.b: a binary table with labelled columns and rows, called a two-dimensional array.
// Signature "2DA V2.b\n" (9 bytes). Column labels, each followed by a separator byte, then one NUL byte.
// u32 row count n. n row labels, each followed by the separator. n * column count u16 cell offsets in row
// order. u16 size of the string data. The string data: NUL-terminated strings, each stored once.
// The separator is a tab in most tables and a NUL byte in some; the byte after the first column label
// gives it.
package kotor

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

const (
	twoDaSignature  = "2DA V2.b\n"
	twoDaCountSize  = 4
	twoDaOffsetSize = 2
)

const (
	tab = '\t'
	nul = 0
)

type TwoDa struct {
	Separator byte
	Columns   []string
	Rows      []string
	Cells     [][]string
	Errors    []string
}

// ParseTwoDa reads a binary 2DA table. It requires every cell offset to point at the start of a string,
// every string to be referenced by a cell, and the string data to end exactly at the end of the file.
func ParseTwoDa(data []byte) *TwoDa {
	table := &TwoDa{}
	fail := func(format string, args ...any) *TwoDa {
		table.Errors = append(table.Errors, fmt.Sprintf(format, args...))
		return table
	}
	if !bytes.HasPrefix(data, []byte(twoDaSignature)) {
		return fail("not 2DA V2.b")
	}

	position := len(twoDaSignature)
	end := bytes.IndexAny(data[position:], "\t\x00")
	if end <= 0 {
		return fail("no column labels")
	}
	table.Separator = data[position+end]

	// label reads one string that ends with the separator and moves past it.
	label := func() (string, bool) {
		end := bytes.IndexByte(data[position:], table.Separator)
		if end < 0 {
			return "", false
		}
		text := Latin1(data[position : position+end])
		position += end + 1
		return text, true
	}

	for position < len(data) && data[position] != nul {
		text, ok := label()
		if !ok {
			return fail("column label runs past end of file")
		}
		table.Columns = append(table.Columns, text)
	}
	position++

	if position+twoDaCountSize > len(data) {
		return fail("row count past end of file")
	}
	rowCount := int(u32(data, position))
	position += twoDaCountSize

	for i := range rowCount {
		text, ok := label()
		if !ok {
			return fail("row label %d runs past end of file", i)
		}
		table.Rows = append(table.Rows, text)
	}

	cellCount := rowCount * len(table.Columns)
	if position+twoDaOffsetSize*(cellCount+1) > len(data) {
		return fail("%d cell offsets run past end of file", cellCount)
	}
	offsets := make([]int, cellCount)
	for i := range offsets {
		offsets[i] = int(u16(data, position+twoDaOffsetSize*i))
	}
	position += twoDaOffsetSize * cellCount

	size := int(u16(data, position))
	position += twoDaOffsetSize
	stringData := data[position:]
	if len(stringData) != size {
		return fail("string data is %d bytes, header says %d", len(stringData), size)
	}
	if size > 0 && stringData[size-1] != nul {
		return fail("string data does not end with NUL")
	}

	referenced := map[int]bool{}
	for r := range rowCount {
		row := make([]string, len(table.Columns))
		for c := range row {
			offset := offsets[r*len(table.Columns)+c]
			if offset >= size || (offset > 0 && stringData[offset-1] != nul) {
				return fail("cell %d,%d has offset %d, which is not the start of a string", r, c, offset)
			}

			row[c] = CString(stringData[offset:])
			referenced[offset] = true
		}
		table.Cells = append(table.Cells, row)
	}

	for offset := 0; offset < size; offset += bytes.IndexByte(stringData[offset:], nul) + 1 {
		if !referenced[offset] {
			return fail("string at %d is not referenced by any cell", offset)
		}
	}
	return table
}

// TwoDaCsv writes the table as CSV. The first line holds the column labels after an empty cell, and each
// further line holds a row label followed by the row's cells. An empty cell stays empty.
func TwoDaCsv(table *TwoDa) []byte {
	records := [][]string{append([]string{""}, table.Columns...)}
	for r, label := range table.Rows {
		records = append(records, append([]string{label}, table.Cells[r]...))
	}

	// Writing to a buffer with the default delimiter cannot fail.
	var text bytes.Buffer
	_ = csv.NewWriter(&text).WriteAll(records)
	return text.Bytes()
}

func ReportTwoDa(label string, table *TwoDa) string {
	separator := "tab"
	if table.Separator == nul {
		separator = "nul"
	}
	return fmt.Sprintf("%s columns=%d rows=%d separator=%s %s", label, len(table.Columns), len(table.Rows),
		separator, Verdict(table.Errors...))
}
