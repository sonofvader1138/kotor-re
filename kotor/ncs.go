// NCS V1.0: a compiled script for the stack machine that runs NWScript (the game's scripting language).
// The file is the 8-byte signature followed by instructions up to the end of the file. The first instruction
// is always opcode 0x42 with a big-endian u32 operand that equals the file size. Every instruction is a u8
// opcode, a u8 type and operands that depend on both; all operands are big-endian. Jump operands are byte
// offsets relative to the start of the jump instruction.
//
// nwscript.nss (type 2009 in data/scripts.bif) declares the engine routines that ACTION calls. Routine n is
// the n-th function declaration in that file.
package kotor

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	ncsSignature  = "NCS V1.0"
	ncsSizeOpcode = 0x42
	ncsHeaderSize = 13
	TypeNss       = 2009
)

// Opcodes whose operands are not just a fixed layout, and the types that change their operands or listing.
const (
	ncsConst      = 0x04
	ncsAction     = 0x05
	ncsEqual      = 0x0b
	ncsNotEqual   = 0x0c
	ncsStructType = 0x24
	ncsFloatType  = 0x04
	ncsStringType = 0x05
)

// Operand layouts. Each letter is one big-endian operand: i = i32, u = u32, h = i16, w = u16, b = u8,
// f = f32, j = i32 jump offset, s = u16 length followed by that many characters.
var NcsOperandSizes = map[rune]int{'i': 4, 'u': 4, 'j': 4, 'f': 4, 'h': 2, 'w': 2, 's': 2, 'b': 1}

// Type codes. I, F, S and O are integer, float, string and object; E0 to E3 are the engine structures that
// nwscript.nss defines (effect, event, location and talent); T is a structure whose size is the operand;
// V is a vector. Two letters name the types of the two operands of a binary operation.
var ncsTypeNames = map[byte]string{
	0x03: "I", 0x04: "F", 0x05: "S", 0x06: "O",
	0x10: "E0", 0x11: "E1", 0x12: "E2", 0x13: "E3",
	0x20: "II", 0x21: "FF", 0x22: "OO", 0x23: "SS", 0x24: "TT", 0x25: "IF", 0x26: "FI",
	0x30: "E0E0", 0x31: "E1E1", 0x32: "E2E2", 0x33: "E3E3",
	0x3a: "VV", 0x3b: "VF", 0x3c: "FV",
}

// Sets of type codes that an opcode accepts. An opcode with a single type has no type suffix in the
// listing. Every opcode uses either type 0 or type 1 when its type carries no meaning, never both.
var (
	ncsNone       = []byte{0x00}
	ncsStack      = []byte{0x01}
	ncsState      = []byte{0x10}
	ncsInteger    = []byte{0x03}
	ncsNumber     = []byte{0x03, 0x04}
	ncsConstants  = []byte{0x03, 0x04, 0x05, 0x06}
	ncsSingle     = []byte{0x03, 0x04, 0x05, 0x06, 0x10, 0x11, 0x12, 0x13}
	ncsIntPair    = []byte{0x20}
	ncsOrderPair  = []byte{0x20, 0x21}
	ncsMathPair   = []byte{0x20, 0x21, 0x23, 0x25, 0x26, 0x3a, 0x3b, 0x3c}
	ncsEqualPairs = []byte{0x20, 0x21, 0x22, 0x23, 0x24, 0x30, 0x31, 0x32, 0x33, 0x3a}
)

var NcsOpcodes = map[byte]struct {
	Name     string
	operands string
	types    []byte
}{
	0x01: {"CPDOWNSP", "iw", ncsStack},
	0x02: {"RSADD", "", ncsSingle},
	0x03: {"CPTOPSP", "iw", ncsStack},
	0x04: {"CONST", "", ncsConstants},
	0x05: {"ACTION", "wb", ncsNone},
	0x06: {"LOGAND", "", ncsIntPair},
	0x07: {"LOGOR", "", ncsIntPair},
	0x08: {"INCOR", "", ncsIntPair},
	0x09: {"EXCOR", "", ncsIntPair},
	0x0a: {"BOOLAND", "", ncsIntPair},
	0x0b: {"EQUAL", "", ncsEqualPairs},
	0x0c: {"NEQUAL", "", ncsEqualPairs},
	0x0d: {"GEQ", "", ncsOrderPair},
	0x0e: {"GT", "", ncsOrderPair},
	0x0f: {"LT", "", ncsOrderPair},
	0x10: {"LEQ", "", ncsOrderPair},
	0x11: {"SHLEFT", "", ncsIntPair},
	0x12: {"SHRIGHT", "", ncsIntPair},
	0x13: {"USHRIGHT", "", ncsIntPair},
	0x14: {"ADD", "", ncsMathPair},
	0x15: {"SUB", "", ncsMathPair},
	0x16: {"MUL", "", ncsMathPair},
	0x17: {"DIV", "", ncsMathPair},
	0x18: {"MOD", "", ncsIntPair},
	0x19: {"NEG", "", ncsNumber},
	0x1a: {"COMP", "", ncsInteger},
	0x1b: {"MOVSP", "i", ncsNone},
	0x1d: {"JMP", "j", ncsNone},
	0x1e: {"JSR", "j", ncsNone},
	0x1f: {"JZ", "j", ncsNone},
	0x20: {"RETN", "", ncsNone},
	0x21: {"DESTRUCT", "whw", ncsStack},
	0x22: {"NOT", "", ncsInteger},
	0x23: {"DECISP", "i", ncsInteger},
	0x24: {"INCISP", "i", ncsInteger},
	0x25: {"JNZ", "j", ncsNone},
	0x26: {"CPDOWNBP", "iw", ncsStack},
	0x27: {"CPTOPBP", "iw", ncsStack},
	0x28: {"DECIBP", "i", ncsInteger},
	0x29: {"INCIBP", "i", ncsInteger},
	0x2a: {"SAVEBP", "", ncsNone},
	0x2b: {"RESTOREBP", "", ncsNone},
	0x2c: {"STORESTATE", "uu", ncsState},
	0x2d: {"NOP", "", ncsNone},
	0x42: {"T", "u", ncsNone},
}

// Operand layout of CONST by type: integer, float, string and object.
var ncsConstOperands = map[byte]string{0x03: "i", 0x04: "f", 0x05: "s", 0x06: "i"}

// NcsOperands returns the operand layout of an opcode with a type, and whether the opcode accepts that
// type. The size instruction has type 0, the value that stands for its missing type byte.
func NcsOperands(opcode, valueType byte) (string, bool) {
	info, known := NcsOpcodes[opcode]
	switch {
	case !known || !slices.Contains(info.types, valueType):
		return "", false

	case opcode == ncsConst:
		return ncsConstOperands[valueType], true

	case (opcode == ncsEqual || opcode == ncsNotEqual) && valueType == ncsStructType:
		return "w", true
	}
	return info.operands, true
}

// NcsMnemonic returns the listing name of an opcode with a type: the opcode name, followed by the type
// name when the opcode accepts more than one type.
func NcsMnemonic(opcode, valueType byte) string {
	info := NcsOpcodes[opcode]
	if len(info.types) == 1 {
		return info.Name
	}
	return info.Name + ncsTypeNames[valueType]
}

// NcsRoutine is one engine routine declared in nwscript.nss. Params holds the parameter types, and
// Required counts the parameters without a default value.
type NcsRoutine struct {
	Name     string
	Return   string
	Params   []string
	Required int
}

type NcsInstruction struct {
	Offset   int
	Opcode   byte
	Type     byte
	Operands []int64
	Text     string
	Target   int
}

// Warnings are calls whose argument count the shipped nwscript.nss does not allow. They are not decoding
// errors: such scripts were probably compiled against another version of nwscript.nss.
type Ncs struct {
	Size         int
	Instructions []NcsInstruction
	Errors       []string
	Warnings     []string
}

var nwscriptDeclaration = regexp.MustCompile(`^\s*(void|int|float|string|object|vector|location|effect|event|talent|action)\s+(\w+)\s*\((.*)\)\s*;`)

// ParseNwscript returns the routines declared in nwscript.nss in declaration order.
func ParseNwscript(text string) []NcsRoutine {
	var routines []NcsRoutine
	for _, line := range strings.Split(text, "\n") {
		match := nwscriptDeclaration.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		routine := NcsRoutine{Name: match[2], Return: match[1]}
		for _, param := range splitParams(match[3]) {
			if param == "" {
				continue
			}
			routine.Params = append(routine.Params, strings.Fields(param)[0])
			if !strings.Contains(param, "=") {
				routine.Required++
			}
		}
		routines = append(routines, routine)
	}
	return routines
}

// splitParams splits a parameter list at the commas that are not inside brackets, such as the commas of a
// default vector value [0.0,0.0,0.0].
func splitParams(text string) []string {
	var params []string
	depth, start := 0, 0
	for i, char := range text {
		switch char {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ',':
			if depth == 0 {
				params = append(params, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	return append(params, strings.TrimSpace(text[start:]))
}

// ParseNcs decodes a compiled script. It reports an error when the header is wrong, when an opcode or type
// is unknown, when an instruction runs past the end of the file, when a jump does not land on an
// instruction, or when ACTION names a routine that nwscript.nss does not declare. An argument count that
// the declaration does not allow is a warning.
func ParseNcs(data []byte, routines []NcsRoutine) *Ncs {
	ncs := &Ncs{Size: len(data)}
	fail := func(format string, args ...any) {
		ncs.Errors = append(ncs.Errors, fmt.Sprintf(format, args...))
	}

	if len(data) < ncsHeaderSize || string(data[:signatureLength]) != ncsSignature {
		fail("not NCS V1.0")
		return ncs
	}
	if data[signatureLength] != ncsSizeOpcode || int(binary.BigEndian.Uint32(data[signatureLength+1:])) != len(data) {
		fail("size instruction does not match the file size")
	}

	starts := map[int]bool{}
	position := signatureLength
	for position < len(data) && len(ncs.Errors) == 0 {
		instruction := NcsInstruction{Offset: position, Opcode: data[position], Target: -1}
		if position+2 > len(data) {
			fail("unknown opcode %#02x at %#x", instruction.Opcode, position)
			break
		}

		// The size instruction has no type byte; every other instruction has one.
		cursor := position + 1
		if instruction.Opcode != ncsSizeOpcode {
			instruction.Type = data[cursor]
			cursor++
		}
		operands, valid := NcsOperands(instruction.Opcode, instruction.Type)
		if !valid {
			fail("opcode %#02x with type %#02x at %#x", instruction.Opcode, instruction.Type, position)
			break
		}

		for _, kind := range operands {
			size := NcsOperandSizes[kind]
			if cursor+size > len(data) {
				fail("instruction at %#x runs past the end of the file", position)
				break
			}

			var value int64
			switch kind {
			case 'i', 'j':
				value = int64(int32(binary.BigEndian.Uint32(data[cursor:])))
			case 'u':
				value = int64(binary.BigEndian.Uint32(data[cursor:]))
			case 'f':
				value = int64(binary.BigEndian.Uint32(data[cursor:]))
			case 'h':
				value = int64(int16(binary.BigEndian.Uint16(data[cursor:])))
			case 'w', 's':
				value = int64(binary.BigEndian.Uint16(data[cursor:]))
			case 'b':
				value = int64(data[cursor])
			}
			cursor += size

			if kind == 's' {
				if cursor+int(value) > len(data) {
					fail("string at %#x runs past the end of the file", position)
					break
				}
				instruction.Text = Latin1(data[cursor : cursor+int(value)])
				cursor += int(value)
			}
			if kind == 'j' {
				instruction.Target = position + int(value)
			}
			instruction.Operands = append(instruction.Operands, value)
		}
		if len(ncs.Errors) > 0 {
			break
		}

		if instruction.Opcode == ncsSizeOpcode && position != signatureLength {
			fail("size instruction at %#x", position)
		}
		if instruction.Opcode == ncsAction {
			routine, argc := int(instruction.Operands[0]), int(instruction.Operands[1])
			if routine >= len(routines) {
				fail("ACTION %d at %#x is not declared", routine, position)
			} else if argc < routines[routine].Required || argc > len(routines[routine].Params) {
				ncs.Warnings = append(ncs.Warnings, fmt.Sprintf("ACTION %s at %#x passes %d arguments", routines[routine].Name, position, argc))
			}
		}

		starts[position] = true
		ncs.Instructions = append(ncs.Instructions, instruction)
		position = cursor
	}

	for _, instruction := range ncs.Instructions {
		if instruction.Target >= 0 && !starts[instruction.Target] {
			fail("jump at %#x to %#x does not land on an instruction", instruction.Offset, instruction.Target)
		}
	}
	return ncs
}

// NcsListing returns the disassembly of a parsed script, one instruction per line: the offset, the
// mnemonic with its type suffix, the operands, and a comment with the jump target or the routine
// declaration. Warnings come first, as comment lines.
func NcsListing(ncs *Ncs, routines []NcsRoutine) []byte {
	var out strings.Builder
	for _, warning := range ncs.Warnings {
		out.WriteString("; warning: " + warning + "\n")
	}

	for _, instruction := range ncs.Instructions {
		mnemonic := NcsMnemonic(instruction.Opcode, instruction.Type)

		var operands []string
		for _, value := range instruction.Operands {
			operands = append(operands, strconv.FormatInt(value, 10))
		}

		comment := ""
		switch {
		case instruction.Opcode == ncsConst && instruction.Type == ncsFloatType:
			bits := uint32(instruction.Operands[0])
			operands = []string{strconv.FormatFloat(float64(math.Float32frombits(bits)), 'g', -1, 32)}

		case instruction.Opcode == ncsConst && instruction.Type == ncsStringType:
			operands = []string{strconv.Quote(instruction.Text)}

		case instruction.Opcode == ncsAction:
			routine := routines[instruction.Operands[0]]
			comment = fmt.Sprintf("%s %s(%s)", routine.Return, routine.Name, strings.Join(routine.Params, ", "))

		case instruction.Target >= 0:
			comment = fmt.Sprintf("-> %08x", instruction.Target)
		}

		text := fmt.Sprintf("%08x  %-12s %s", instruction.Offset, mnemonic, strings.Join(operands, " "))
		if comment != "" {
			text = fmt.Sprintf("%-40s ; %s", text, comment)
		}
		out.WriteString(strings.TrimRight(text, " ") + "\n")
	}
	return []byte(out.String())
}

// ReportNcs returns one report line for a script.
func ReportNcs(label string, ncs *Ncs) string {
	warnings := ""
	if len(ncs.Warnings) > 0 {
		warnings = "warning: " + strings.Join(ncs.Warnings, "; ") + " "
	}
	return fmt.Sprintf("%s %d bytes %d instructions %s%s", label, ncs.Size, len(ncs.Instructions), warnings, Verdict(ncs.Errors...))
}

// ReadNwscript returns the routines declared in nwscript.nss, which is in data/scripts.bif.
func ReadNwscript(key *Key) []NcsRoutine {
	for _, resource := range BifResources("data/scripts.bif", key) {
		if resource.Type == TypeNss && resource.Resref == "nwscript" {
			return ParseNwscript(string(resource.Data))
		}
	}

	Fatal("nwscript.nss not found in data/scripts.bif")
	return nil
}
