// VIS: the room visibility list of one area, resource type 3001, in data/lightmaps*.bif. It is ASCII text
// with CRLF line breaks; the last line may lack its line break. A room line holds a room name, spaces and
// a decimal count, and starts at the first column. Each following line indented by two spaces names a room
// visible from that room. The count is meant to equal the number of indented lines, but it does not
// always, so the parser takes the list from the indentation and keeps the count as written.
package kotor

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	visLineBreak = "\r\n"
	visIndent    = "  "
)

// VisNoCount is the count of a room line that has no count.
const VisNoCount = -1

type VisRoom struct {
	Name    string
	Count   int // as written, or VisNoCount
	Visible []string
}

type Vis struct {
	Rooms []VisRoom

	// Irregularities that the parser accepts. Mismatches counts the rooms whose count differs from the
	// number of indented lines. Spacing counts the lines with more spaces than the canonical form: two or
	// more spaces before a count, or spaces after a name.
	Mismatches   int
	NoCounts     int
	Spacing      int
	NoFinalBreak bool

	Errors []string
}

// ParseVis reads a room visibility list. It requires printable ASCII, CRLF line breaks, room names of at
// most ResrefLength characters, and an indented line to follow a room line.
func ParseVis(data []byte) *Vis {
	vis := &Vis{}
	for offset, value := range data {
		if (value < ' ' || value > '~') && value != '\r' && value != '\n' {
			vis.Errors = append(vis.Errors, fmt.Sprintf("byte 0x%02x at %d", value, offset))
			return vis
		}
	}

	text := string(data)
	vis.NoFinalBreak = !strings.HasSuffix(text, visLineBreak)
	lines := strings.Split(strings.TrimSuffix(text, visLineBreak), visLineBreak)
	for number, line := range lines {
		fail := func(problem string) {
			vis.Errors = append(vis.Errors, fmt.Sprintf("line %d: %s", number+1, problem))
		}

		if strings.ContainsAny(line, "\r\n") {
			fail("line break other than CRLF")
			continue
		}

		if strings.HasPrefix(line, visIndent) {
			name := strings.TrimRight(line[len(visIndent):], " ")
			if name != line[len(visIndent):] {
				vis.Spacing++
			}
			if len(vis.Rooms) == 0 {
				fail("indented line before the first room")
				continue
			}
			if name == "" || strings.Contains(name, " ") || len(name) > ResrefLength {
				fail(fmt.Sprintf("bad room name %q", name))
				continue
			}

			room := &vis.Rooms[len(vis.Rooms)-1]
			room.Visible = append(room.Visible, name)
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 || len(fields) > 2 || line[0] == ' ' || len(fields[0]) > ResrefLength {
			fail(fmt.Sprintf("bad room line %q", line))
			continue
		}

		room := VisRoom{Name: fields[0], Count: VisNoCount}
		if len(fields) == 2 {
			count, err := strconv.ParseUint(fields[1], 10, 16)
			if err != nil {
				fail(fmt.Sprintf("bad count %q", fields[1]))
				continue
			}

			room.Count = int(count)
			if line != fields[0]+" "+fields[1] {
				vis.Spacing++
			}
		}
		vis.Rooms = append(vis.Rooms, room)
	}

	for _, room := range vis.Rooms {
		if room.Count == VisNoCount {
			vis.NoCounts++
		} else if room.Count != len(room.Visible) {
			vis.Mismatches++
		}
	}
	return vis
}

func ReportVis(label string, vis *Vis) string {
	visible := 0
	for _, room := range vis.Rooms {
		visible += len(room.Visible)
	}
	return fmt.Sprintf("%s rooms=%d visible=%d count_mismatch=%d no_count=%d extra_spaces=%d no_final_break=%t %s",
		label, len(vis.Rooms), visible, vis.Mismatches, vis.NoCounts, vis.Spacing, vis.NoFinalBreak, Verdict(vis.Errors...))
}
