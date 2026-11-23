package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"re/kotor"
)

// A GUI file becomes JSON the way JSON.stringify(value, null, 1) wrote it in the JS converter. The rules
// that matter here: an object lists keys that are array indices first, in numeric order, and then the
// other keys in insertion order; assigning an existing key keeps its position; numbers use
// Number.prototype.toString, and NaN and infinities become null.

const (
	jsonNumber = iota
	jsonString
	jsonObject
	jsonArray
)

// jsonValue is a number, a string, an object (keys and items in parallel) or an array (items).
type jsonValue struct {
	kind   int
	number float64
	text   string
	keys   []string
	items  []jsonValue
}

// set assigns a key of an object, keeping the position of a key that already exists.
func (object *jsonValue) set(key string, value jsonValue) {
	for i, existing := range object.keys {
		if existing == key {
			object.items[i] = value
			return
		}
	}
	object.keys = append(object.keys, key)
	object.items = append(object.items, value)
}

// The largest array index is 2^32 - 2.
const maxArrayIndex = math.MaxUint32 - 1

// arrayIndex reports whether a key is the canonical decimal form of an array index, and its value.
func arrayIndex(key string) (uint64, bool) {
	index, err := strconv.ParseUint(key, 10, 64)
	if err != nil || index > maxArrayIndex || strconv.FormatUint(index, 10) != key {
		return 0, false
	}
	return index, true
}

// quote writes a string as a JSON string literal: quotes, backslashes and control characters are escaped.
func quote(out *strings.Builder, text string) {
	out.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
}

func writeJson(out *strings.Builder, value jsonValue, depth int) {
	switch value.kind {
	case jsonNumber:
		if math.IsNaN(value.number) || math.IsInf(value.number, 0) {
			out.WriteString("null")
		} else {
			out.WriteString(kotor.NumberString(value.number))
		}
		return

	case jsonString:
		quote(out, value.text)
		return
	}

	opening, closing := "[", "]"
	if value.kind == jsonObject {
		opening, closing = "{", "}"
	}
	if len(value.items) == 0 {
		out.WriteString(opening + closing)
		return
	}

	order := make([]int, len(value.items))
	for i := range order {
		order[i] = i
	}
	if value.kind == jsonObject {
		sort.SliceStable(order, func(a, b int) bool {
			indexA, isIndexA := arrayIndex(value.keys[order[a]])
			indexB, isIndexB := arrayIndex(value.keys[order[b]])
			if isIndexA && isIndexB {
				return indexA < indexB
			}
			return isIndexA && !isIndexB
		})
	}

	inner := strings.Repeat(" ", depth+1)
	out.WriteString(opening + "\n")
	for n, i := range order {
		if n > 0 {
			out.WriteString(",\n")
		}
		out.WriteString(inner)
		if value.kind == jsonObject {
			quote(out, value.keys[i])
			out.WriteString(": ")
		}
		writeJson(out, value.items[i], depth+1)
	}
	out.WriteString("\n" + strings.Repeat(" ", depth) + closing)
}

// Nine significant digits always round-trip an f32.
const f32MaxDigits = 9

// shortestF32 returns the decimal with the fewest significant digits that rounds to the same f32.
func shortestF32(value float64) jsonValue {
	for digits := 1; digits <= f32MaxDigits; digits++ {
		shortest := kotor.RoundSignificant(value, digits)
		if float64(float32(shortest)) == value {
			return jsonValue{kind: jsonNumber, number: shortest}
		}
	}
	return jsonValue{kind: jsonNumber, number: value}
}

// guiJson turns a struct into an object keyed by field label, in file order. A struct type other than 0 is
// kept as "__struct_type". Lists become arrays of objects, vectors become 3-element arrays, and f32 values
// use the shortest decimal that round-trips.
func guiJson(structValue *kotor.GffStructValue) jsonValue {
	object := jsonValue{kind: jsonObject}
	if structValue.StructType != 0 {
		object.set("__struct_type", jsonValue{kind: jsonNumber, number: float64(structValue.StructType)})
	}

	for _, field := range structValue.Fields {
		var value jsonValue
		switch field.Type {
		case kotor.GffStruct:
			value = guiJson(field.Value.Struct)

		case kotor.GffList:
			value = jsonValue{kind: jsonArray}
			for _, item := range field.Value.List {
				value.items = append(value.items, guiJson(item))
			}

		case kotor.GffF32:
			value = shortestF32(field.Value.Number)

		case kotor.GffVector:
			value = jsonValue{kind: jsonArray}
			for _, component := range field.Value.Vector {
				value.items = append(value.items, shortestF32(component))
			}

		case kotor.GffString, kotor.GffResref:
			value = jsonValue{kind: jsonString, text: field.Value.Text}

		default:
			value = jsonValue{kind: jsonNumber, number: field.Value.Number}
		}
		object.set(field.Label, value)
	}
	return object
}
