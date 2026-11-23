package main

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"

	"re/kotor"
)

// A GUI file becomes JSON with one-space indentation. Object keys keep the order of the fields in the file,
// and assigning an existing key keeps its position. NaN and infinities have no JSON form and become null.

// orderedObject is a JSON object whose keys keep their insertion order.
type orderedObject struct {
	keys   []string
	values []any
}

// set assigns a key, keeping the position of a key that already exists.
func (object *orderedObject) set(key string, value any) {
	for i, existing := range object.keys {
		if existing == key {
			object.values[i] = value
			return
		}
	}
	object.keys = append(object.keys, key)
	object.values = append(object.values, value)
}

func (object *orderedObject) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for i, key := range object.keys {
		if i > 0 {
			out.WriteByte(',')
		}

		keyText, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		valueText, err := json.Marshal(object.values[i])
		if err != nil {
			return nil, err
		}

		out.Write(keyText)
		out.WriteByte(':')
		out.Write(valueText)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// number formats a value with the fewest digits that read back as the same value at the given bit size.
func number(value float64, bitSize int) any {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return json.Number(strconv.FormatFloat(value, 'f', -1, bitSize))
}

// guiJson turns a struct into an object keyed by field label, in file order. A struct type other than 0 is
// kept as "__struct_type". Lists become arrays of objects, vectors become 3-element arrays, and f32 values
// use the shortest decimal that round-trips as an f32.
func guiJson(structValue *kotor.GffStructValue) *orderedObject {
	object := &orderedObject{}
	if structValue.StructType != 0 {
		object.set("__struct_type", number(float64(structValue.StructType), 64))
	}

	for _, field := range structValue.Fields {
		var value any
		switch field.Type {
		case kotor.GffStruct:
			value = guiJson(field.Value.Struct)

		case kotor.GffList:
			items := []any{}
			for _, item := range field.Value.List {
				items = append(items, guiJson(item))
			}
			value = items

		case kotor.GffF32:
			value = number(field.Value.Number, 32)

		case kotor.GffVector:
			components := []any{}
			for _, component := range field.Value.Vector {
				components = append(components, number(component, 32))
			}
			value = components

		case kotor.GffString, kotor.GffResref:
			value = field.Value.Text

		default:
			value = number(field.Value.Number, 64)
		}
		object.set(field.Label, value)
	}
	return object
}
