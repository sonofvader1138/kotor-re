// LYT: the room layout of one area, resource type 3000, in data/layouts.bif. It is ASCII text with CRLF line
// breaks, exported from 3ds Max. It places the room models of the area, the track and obstacle models of the
// minigames, and the door hooks, the places where doors stand. The lines come in a fixed order: a header of
// three lines, four sections that each begin with a count line, and a closing line. Fields are separated by
// white space, as the scan formats of swkotor.exe ("%s %f%f%f", "%*s%d") read them.
package kotor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	lytLineBreak  = "\r\n"
	LytSignature  = "#MAXLAYOUT ASCII"
	LytDependency = "filedependancy"
	LytBegin      = "beginlayout"
	LytDone       = "donelayout"
)

// Canonical indentation of count lines and of entry lines.
const (
	LytCountIndent = "   "
	LytEntryIndent = "      "
)

// LytSections names the count keyword of each section, in file order.
var LytSections = []string{"roomcount", "trackcount", "obstaclecount", "doorhookcount"}

const (
	lytDoorHookSection = 3
	lytPlacementFields = 4  // model, x, y, z
	lytDoorHookFields  = 10 // room, door, unknown integer, x, y, z, quaternion w, x, y, z
)

// lytNumber matches the decimal numbers of the files, such as "0.0", "-1.31576" and "1.26441e-007".
var lytNumber = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?(e[-+][0-9]+)?$`)

// LytPlacement places one model: a room, a track or an obstacle.
type LytPlacement struct {
	Model    string
	Position [3]float64
}

// LytDoorHook places one door. Orientation is a unit quaternion in the order w, x, y, z.
type LytDoorHook struct {
	Room        string
	Door        string
	Unknown     int
	Position    [3]float64
	Orientation [4]float64
}

type Lyt struct {
	Dependency string
	Rooms      []LytPlacement
	Tracks     []LytPlacement
	Obstacles  []LytPlacement
	DoorHooks  []LytDoorHook

	// Trailer holds the bytes after the line break of the closing line.
	Trailer string

	// Irregularities that the parser accepts. Indents counts the lines whose leading white space differs
	// from the canonical indentation, such as a tab.
	Indents      int
	NoFinalBreak bool

	Errors []string
}

// ParseLyt reads a room layout. It requires printable ASCII, tabs and CRLF line breaks, the lines in their
// fixed order, the number of entries that each count gives, and names of at most ResrefLength characters.
func ParseLyt(data []byte) *Lyt {
	lyt := &Lyt{}
	for offset, value := range data {
		if (value < ' ' || value > '~') && value != '\t' && value != '\r' && value != '\n' {
			lyt.Errors = append(lyt.Errors, fmt.Sprintf("byte 0x%02x at %d", value, offset))
			return lyt
		}
	}

	text := string(data)
	lines := strings.Split(text, lytLineBreak)
	number := 0
	fail := func(problem string) {
		lyt.Errors = append(lyt.Errors, fmt.Sprintf("line %d: %s", number, problem))
	}

	// next returns the fields of the next line and counts its indentation when it is not canonical.
	next := func(indent string) []string {
		if number >= len(lines) {
			fail("unexpected end of file")
			return nil
		}

		line := lines[number]
		if strings.ContainsAny(line, "\r\n") {
			fail("line break other than CRLF")
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && line[:strings.Index(line, fields[0])] != indent {
			lyt.Indents++
		}
		number++
		return fields
	}

	expect := func(indent string, words ...string) {
		fields := next(indent)
		if strings.Join(fields, " ") != strings.Join(words, " ") {
			fail(fmt.Sprintf("expected %q, found %q", strings.Join(words, " "), strings.Join(fields, " ")))
		}
	}

	numbers := func(fields []string) []float64 {
		values := make([]float64, len(fields))
		for i, field := range fields {
			value, err := strconv.ParseFloat(field, 64)
			if err != nil || !lytNumber.MatchString(field) {
				fail(fmt.Sprintf("bad number %q", field))
			}
			values[i] = value
		}
		return values
	}

	name := func(field string) string {
		if len(field) > ResrefLength {
			fail(fmt.Sprintf("name %q longer than %d characters", field, ResrefLength))
		}
		return field
	}

	expect("", strings.Fields(LytSignature)...)
	fields := next("")
	if len(fields) != 2 || fields[0] != LytDependency {
		fail(fmt.Sprintf("bad file dependency line %q", strings.Join(fields, " ")))
	} else {
		lyt.Dependency = fields[1]
	}
	expect("", LytBegin)

	for section, keyword := range LytSections {
		fields := next(LytCountIndent)
		if len(fields) != 2 || fields[0] != keyword {
			fail(fmt.Sprintf("expected %s, found %q", keyword, strings.Join(fields, " ")))
			return lyt
		}

		count, err := strconv.ParseUint(fields[1], 10, 16)
		if err != nil {
			fail(fmt.Sprintf("bad count %q", fields[1]))
			return lyt
		}

		for range count {
			fields := next(LytEntryIndent)
			if section == lytDoorHookSection {
				if len(fields) != lytDoorHookFields {
					fail(fmt.Sprintf("door hook with %d fields", len(fields)))
					return lyt
				}

				unknown, err := strconv.Atoi(fields[2])
				if err != nil {
					fail(fmt.Sprintf("bad integer %q", fields[2]))
				}
				values := numbers(fields[3:])
				hook := LytDoorHook{Room: name(fields[0]), Door: name(fields[1]), Unknown: unknown}
				copy(hook.Position[:], values[:3])
				copy(hook.Orientation[:], values[3:])
				lyt.DoorHooks = append(lyt.DoorHooks, hook)
				continue
			}

			if len(fields) != lytPlacementFields {
				fail(fmt.Sprintf("%s entry with %d fields", keyword, len(fields)))
				return lyt
			}

			placement := LytPlacement{Model: name(fields[0])}
			copy(placement.Position[:], numbers(fields[1:]))
			switch section {
			case 0:
				lyt.Rooms = append(lyt.Rooms, placement)
			case 1:
				lyt.Tracks = append(lyt.Tracks, placement)
			default:
				lyt.Obstacles = append(lyt.Obstacles, placement)
			}
		}
	}
	expect("", LytDone)

	// The closing line ends at its line break, or at the end of the file when it has none.
	consumed := 0
	for _, line := range lines[:number] {
		consumed += len(line) + len(lytLineBreak)
	}
	if consumed > len(text) {
		lyt.NoFinalBreak = true
		consumed = len(text)
	}
	lyt.Trailer = text[consumed:]
	return lyt
}

func ReportLyt(label string, lyt *Lyt) string {
	return fmt.Sprintf("%s rooms=%d tracks=%d obstacles=%d doorhooks=%d trailer_bytes=%d odd_indents=%d no_final_break=%t %s",
		label, len(lyt.Rooms), len(lyt.Tracks), len(lyt.Obstacles), len(lyt.DoorHooks), len(lyt.Trailer), lyt.Indents,
		lyt.NoFinalBreak, Verdict(lyt.Errors...))
}
