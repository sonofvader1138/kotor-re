package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"

	"re/kotor"
)

// A GFF file (GUI layouts and module resources) becomes JSON indented with tabs. Object keys keep the order of the fields in the file,
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

// gffJson turns a struct into an object keyed by field label, in file order. A struct type other than 0 is
// kept as "__struct_type". Lists become arrays of objects, vectors become 3-element arrays and orientations
// 4-element arrays, and f32 values use the shortest decimal that round-trips as an f32. Binary data becomes
// a lowercase hexadecimal string. A localized string becomes an object with "strref", the string number
// (null for 0xFFFFFFFF); "text", the string's text in dialog.tlk when the number names an entry with text;
// and "substrings", when there are any, as objects with "id" and "text". unresolved counts the string
// numbers that name no entry of dialog.tlk.
func gffJson(structValue *kotor.GffStructValue, tlk *kotor.Tlk, unresolved *int) *orderedObject {
	object := &orderedObject{}
	if structValue.StructType != 0 {
		object.set("__struct_type", number(float64(structValue.StructType), 64))
	}

	for _, field := range structValue.Fields {
		var value any
		switch field.Type {
		case kotor.GffStruct:
			value = gffJson(field.Value.Struct, tlk, unresolved)

		case kotor.GffList:
			items := []any{}
			for _, item := range field.Value.List {
				items = append(items, gffJson(item, tlk, unresolved))
			}
			value = items

		case kotor.GffF32:
			value = number(field.Value.Number, 32)

		case kotor.GffVector, kotor.GffOrientation:
			components := []any{}
			for _, component := range field.Value.Vector {
				components = append(components, number(component, 32))
			}
			value = components

		case kotor.GffString, kotor.GffResref:
			value = field.Value.Text

		case kotor.GffBinary:
			value = hex.EncodeToString(field.Value.Bytes)

		case kotor.GffLocString:
			value = locStringJson(field.Value, tlk, unresolved)

		default:
			value = number(field.Value.Number, 64)
		}
		object.set(field.Label, value)
	}
	return object
}

// noString is the string number of a localized string that has no entry in dialog.tlk.
const noString = 0xffffffff

func locStringJson(value kotor.GffFieldValue, tlk *kotor.Tlk, unresolved *int) *orderedObject {
	object := &orderedObject{}
	strref := int(value.Number)
	switch {
	case strref == noString:
		object.set("strref", nil)

	case strref >= len(tlk.Entries):
		object.set("strref", number(value.Number, 64))
		*unresolved++

	default:
		object.set("strref", number(value.Number, 64))
		if tlk.Entries[strref].Text != "" {
			object.set("text", tlk.Entries[strref].Text)
		}
	}

	if len(value.Substrings) > 0 {
		substrings := []any{}
		for _, substring := range value.Substrings {
			item := &orderedObject{}
			item.set("id", number(float64(substring.ID), 64))
			item.set("text", substring.Text)
			substrings = append(substrings, item)
		}
		object.set("substrings", substrings)
	}
	return object
}

// ssfJson turns a sound set into an object with "slots", an array with one object per slot in file order.
// Each slot has "strref", the string number (null for an empty slot), and, when the number names an entry
// of dialog.tlk, that entry's "text" and "sound" when they are not empty. unresolved counts the string
// numbers that name no entry of dialog.tlk.
func ssfJson(ssf *kotor.Ssf, tlk *kotor.Tlk, unresolved *int) *orderedObject {
	slots := []any{}
	for _, strref := range ssf.Slots {
		slot := &orderedObject{}
		switch {
		case strref == kotor.SsfEmpty:
			slot.set("strref", nil)

		case int(strref) >= len(tlk.Entries):
			slot.set("strref", number(float64(strref), 64))
			*unresolved++

		default:
			slot.set("strref", number(float64(strref), 64))
			entry := tlk.Entries[strref]
			if entry.Text != "" {
				slot.set("text", entry.Text)
			}
			if entry.Sound != "" {
				slot.set("sound", entry.Sound)
			}
		}
		slots = append(slots, slot)
	}

	object := &orderedObject{}
	object.set("slots", slots)
	return object
}

// ltrJson turns a letter table into an object with "letters", the alphabet in table order; "single", the
// tables for single letters; "double", an object keyed by the preceding letter; and "triple", an object
// keyed by the preceding pair of letters. Each set of tables is an object with "start", "middle" and
// "end", each an array of the values in file order, one per letter. The values are the stored cumulative
// probabilities, written with the shortest decimal that round-trips as an f32.
func ltrJson(ltr *kotor.Ltr) *orderedObject {
	setJson := func(set kotor.LtrSet) *orderedObject {
		object := &orderedObject{}
		for i, position := range kotor.LtrPositions {
			values := make([]any, len(set[i]))
			for j, value := range set[i] {
				values[j] = number(float64(value), 32)
			}
			object.set(position, values)
		}
		return object
	}

	double := &orderedObject{}
	for i, set := range ltr.Double {
		double.set(kotor.LtrLetters[i:i+1], setJson(set))
	}

	triple := &orderedObject{}
	for i, set := range ltr.Triple {
		first, second := i/ltr.Letters, i%ltr.Letters
		triple.set(kotor.LtrLetters[first:first+1]+kotor.LtrLetters[second:second+1], setJson(set))
	}

	object := &orderedObject{}
	object.set("letters", kotor.LtrLetters)
	object.set("single", setJson(ltr.Single))
	object.set("double", double)
	object.set("triple", triple)
	return object
}

// visJson turns a room visibility list into an object with "rooms", an array with one object per room line
// in file order. Each room has "room", its name; "count", the count as written (null when the line has
// none); and "visible", the names of the indented lines that follow it. Names keep their case. Spaces
// beyond the canonical form and a missing final line break are not kept.
func visJson(vis *kotor.Vis) *orderedObject {
	rooms := make([]any, len(vis.Rooms))
	for i, room := range vis.Rooms {
		object := &orderedObject{}
		object.set("room", room.Name)

		var count any
		if room.Count != kotor.VisNoCount {
			count = room.Count
		}
		object.set("count", count)

		visible := make([]any, len(room.Visible))
		for j, name := range room.Visible {
			visible[j] = name
		}
		object.set("visible", visible)
		rooms[i] = object
	}

	object := &orderedObject{}
	object.set("rooms", rooms)
	return object
}

// lytJson turns a room layout into an object with "filedependancy", the name of the 3ds Max scene, spelled as
// in the file; "rooms", "tracks" and "obstacles", one object per entry with "model" and "position" (x, y, z);
// "doorhooks", one object per door hook with "room", "door", "unknown" (the integer after the door name),
// "position" and "orientation" (a quaternion as w, x, y, z); and "trailer", the text after the closing line.
// The counts are the lengths of the arrays. Indentation and a missing final line break are not kept.
func lytJson(lyt *kotor.Lyt) *orderedObject {
	numbers := func(values []float64) []any {
		list := make([]any, len(values))
		for i, value := range values {
			list[i] = number(value, 64)
		}
		return list
	}

	placements := func(entries []kotor.LytPlacement) []any {
		list := make([]any, len(entries))
		for i, entry := range entries {
			object := &orderedObject{}
			object.set("model", entry.Model)
			object.set("position", numbers(entry.Position[:]))
			list[i] = object
		}
		return list
	}

	hooks := make([]any, len(lyt.DoorHooks))
	for i, hook := range lyt.DoorHooks {
		object := &orderedObject{}
		object.set("room", hook.Room)
		object.set("door", hook.Door)
		object.set("unknown", hook.Unknown)
		object.set("position", numbers(hook.Position[:]))
		object.set("orientation", numbers(hook.Orientation[:]))
		hooks[i] = object
	}

	object := &orderedObject{}
	object.set("filedependancy", lyt.Dependency)
	object.set("rooms", placements(lyt.Rooms))
	object.set("tracks", placements(lyt.Tracks))
	object.set("obstacles", placements(lyt.Obstacles))
	object.set("doorhooks", hooks)
	object.set("trailer", lyt.Trailer)
	return object
}
