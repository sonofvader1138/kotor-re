package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"re/kotor"
)

// A GUI file becomes JSON with one-space indentation. Object keys keep the order of the fields in the file,
// and assigning an existing key keeps its position. NaN and infinities have no JSON form and become null.

const (
	jsonNumber = iota
	jsonString
	jsonObject
	jsonArray
)

// jsonValue is a number or a string (text), an object (keys and items in parallel) or an array (items).
// A number's text is already formatted.
type jsonValue struct {
	kind  int
	text  string
	keys  []string
	items []jsonValue
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

// number formats a value with the fewest digits that read back as the same value at the given bit size.
func number(value float64, bitSize int) jsonValue {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return jsonValue{kind: jsonNumber, text: "null"}
	}
	return jsonValue{kind: jsonNumber, text: strconv.FormatFloat(value, 'f', -1, bitSize)}
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
		out.WriteString(value.text)
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

	inner := strings.Repeat(" ", depth+1)
	out.WriteString(opening + "\n")
	for i, item := range value.items {
		if i > 0 {
			out.WriteString(",\n")
		}
		out.WriteString(inner)
		if value.kind == jsonObject {
			quote(out, value.keys[i])
			out.WriteString(": ")
		}
		writeJson(out, item, depth+1)
	}
	out.WriteString("\n" + strings.Repeat(" ", depth) + closing)
}

// guiJson turns a struct into an object keyed by field label, in file order. A struct type other than 0 is
// kept as "__struct_type". Lists become arrays of objects, vectors become 3-element arrays, and f32 values
// use the shortest decimal that round-trips as an f32.
func guiJson(structValue *kotor.GffStructValue) jsonValue {
	object := jsonValue{kind: jsonObject}
	if structValue.StructType != 0 {
		object.set("__struct_type", number(float64(structValue.StructType), 64))
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
			value = number(field.Value.Number, 32)

		case kotor.GffVector:
			value = jsonValue{kind: jsonArray}
			for _, component := range field.Value.Vector {
				value.items = append(value.items, number(component, 32))
			}

		case kotor.GffString, kotor.GffResref:
			value = jsonValue{kind: jsonString, text: field.Value.Text}

		default:
			value = number(field.Value.Number, 64)
		}
		object.set(field.Label, value)
	}
	return object
}
