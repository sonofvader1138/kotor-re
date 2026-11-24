package kotor

import (
	"fmt"
)

// A saved game's GLOBALVARS.res is a GFF file of type "GVT " that holds the global script variables, which
// data/2da/globalcat.2da declares. The names are stored in four lists of structs with one string field
// "Name": CatBoolean, CatNumber, CatLocation and CatString. The values are stored apart from the names:
//
//   - ValBoolean (binary) packs one bit per boolean, eight per byte, in the order of CatBoolean. The
//     order of the bits within a byte is UNKNOWN, because every boolean of the only save is false. This
//     reader takes the high bit first.
//   - ValNumber (binary) holds one signed byte per number, in the order of CatNumber. nwscript.nss limits
//     global numbers to -128 to +127.
//   - ValString (list) holds one struct per string, in the order of CatString, with one string field
//     "String".
//   - ValLocation (binary) holds the locations. Its layout is UNKNOWN: it is 2,400 bytes for five
//     locations, and every byte is 0.
const globalsFileType = "GVT "

const bitsPerByte = 8

// GlobalValue is a boolean (0 or 1) or number variable.
type GlobalValue struct {
	Name  string
	Value int
}

// GlobalString is a string variable.
type GlobalString struct {
	Name  string
	Value string
}

// Globals holds the decoded variables of GLOBALVARS.res. LocationData is ValLocation, whose layout is
// UNKNOWN.
type Globals struct {
	Booleans     []GlobalValue
	Numbers      []GlobalValue
	Strings      []GlobalString
	Locations    []string
	LocationData []byte
}

// globalsLabels lists the root fields of GLOBALVARS.res with their types.
var globalsLabels = map[string]uint32{
	"CatBoolean": GffList, "CatNumber": GffList, "CatLocation": GffList, "CatString": GffList,
	"ValBoolean": GffBinary, "ValNumber": GffBinary, "ValLocation": GffBinary, "ValString": GffList,
}

// ReadGlobals decodes a parsed GLOBALVARS.res. It returns an error text when a root field is missing,
// repeated or of an unexpected type, when a list item is not a single string field of the expected label,
// when a value region does not match its count of names, or when a spare bit of ValBoolean is set.
func ReadGlobals(gff *Gff) (*Globals, string) {
	if gff.FileType != globalsFileType {
		return nil, "file type " + gff.FileType
	}

	fields := map[string]GffFieldValue{}
	for _, field := range gff.Root.Fields {
		wanted, known := globalsLabels[field.Label]
		if !known || field.Type != wanted {
			return nil, fmt.Sprintf("unexpected field %s of type %d", field.Label, field.Type)
		}
		if _, seen := fields[field.Label]; seen {
			return nil, "repeated field " + field.Label
		}
		fields[field.Label] = field.Value
	}
	if len(fields) != len(globalsLabels) {
		return nil, fmt.Sprintf("%d of %d root fields", len(fields), len(globalsLabels))
	}

	// texts reads the single string field of each item of a list.
	problem := ""
	texts := func(list, label string) []string {
		var values []string
		for i, item := range fields[list].List {
			if len(item.Fields) != 1 || item.Fields[0].Label != label || item.Fields[0].Type != GffString {
				problem = fmt.Sprintf("%s item %d is not a single %s string", list, i, label)
				return nil
			}
			values = append(values, item.Fields[0].Value.Text)
		}
		return values
	}

	booleanNames := texts("CatBoolean", "Name")
	numberNames := texts("CatNumber", "Name")
	stringNames := texts("CatString", "Name")
	locationNames := texts("CatLocation", "Name")
	stringValues := texts("ValString", "String")
	if problem != "" {
		return nil, problem
	}

	bits := fields["ValBoolean"].Bytes
	numbers := fields["ValNumber"].Bytes
	switch {
	case len(bits) != (len(booleanNames)+bitsPerByte-1)/bitsPerByte:
		return nil, fmt.Sprintf("ValBoolean has %d bytes for %d names", len(bits), len(booleanNames))

	case len(numbers) != len(numberNames):
		return nil, fmt.Sprintf("ValNumber has %d bytes for %d names", len(numbers), len(numberNames))

	case len(stringValues) != len(stringNames):
		return nil, fmt.Sprintf("ValString has %d items for %d names", len(stringValues), len(stringNames))
	}

	globals := &Globals{Locations: locationNames, LocationData: fields["ValLocation"].Bytes}
	for i := range len(bits) * bitsPerByte {
		bit := int(bits[i/bitsPerByte]>>(bitsPerByte-1-i%bitsPerByte)) & 1
		if i >= len(booleanNames) {
			if bit != 0 {
				return nil, fmt.Sprintf("spare bit %d of ValBoolean is set", i)
			}
			continue
		}
		globals.Booleans = append(globals.Booleans, GlobalValue{Name: booleanNames[i], Value: bit})
	}

	for i, name := range numberNames {
		globals.Numbers = append(globals.Numbers, GlobalValue{Name: name, Value: int(int8(numbers[i]))})
	}

	for i, name := range stringNames {
		globals.Strings = append(globals.Strings, GlobalString{Name: name, Value: stringValues[i]})
	}

	return globals, ""
}
