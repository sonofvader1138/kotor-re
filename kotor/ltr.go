// LTR V1.0: letter probabilities for a random name generator. Signature "LTR V1.0" (8 bytes), then a u8
// letter count n, then f32 tables of n values each. The tables are, in order: three for single letters
// (start, middle and end of a name); three for each letter, giving the letter after it; and three for each
// pair of letters, giving the letter after the pair. A nonzero value is the cumulative probability up to
// and including that letter, and 0 means that the letter cannot occur there.
package kotor

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	ltrSignature = "LTR V1.0"
	ltrValueSize = 4
)

// LtrLetters is the alphabet of the tables, in table order, for a letter count of 28.
const LtrLetters = "abcdefghijklmnopqrstuvwxyz'-"

// LtrPositions names the three tables of each set, in file order.
var LtrPositions = []string{"start", "middle", "end"}

// An LtrSet holds the start, middle and end tables of one context, each with one value per letter.
type LtrSet [3][]float32

type Ltr struct {
	Letters int
	Single  LtrSet
	Double  []LtrSet // one per preceding letter
	Triple  []LtrSet // one per preceding pair, the first letter varying slowest
	Errors  []string
}

// LtrSize returns the file size for a letter count.
func LtrSize(letters int) int {
	sets := 1 + letters + letters*letters
	return signatureLength + 1 + sets*len(LtrPositions)*letters*ltrValueSize
}

// ParseLtr reads a letter table. It requires the letter count to match the alphabet, the tables to end
// exactly at the end of the file, and every value to lie between 0 and 1.
func ParseLtr(data []byte) *Ltr {
	ltr := &Ltr{}
	if len(data) <= signatureLength || string(data[:signatureLength]) != ltrSignature {
		ltr.Errors = append(ltr.Errors, "not LTR V1.0")
		return ltr
	}

	ltr.Letters = int(data[signatureLength])
	if ltr.Letters != len(LtrLetters) {
		ltr.Errors = append(ltr.Errors, fmt.Sprintf("letter count %d", ltr.Letters))
		return ltr
	}
	if len(data) != LtrSize(ltr.Letters) {
		ltr.Errors = append(ltr.Errors, fmt.Sprintf("size %d, expected %d", len(data), LtrSize(ltr.Letters)))
		return ltr
	}

	position := signatureLength + 1
	readSet := func() LtrSet {
		var set LtrSet
		for table := range set {
			set[table] = make([]float32, ltr.Letters)
			for i := range set[table] {
				value := math.Float32frombits(binary.LittleEndian.Uint32(data[position:]))
				if !(value >= 0 && value <= ltrMaxValue) {
					ltr.Errors = append(ltr.Errors, fmt.Sprintf("value %v at %d", value, position))
				}
				set[table][i] = value
				position += ltrValueSize
			}
		}
		return set
	}

	ltr.Single = readSet()
	for range ltr.Letters {
		ltr.Double = append(ltr.Double, readSet())
	}
	for range ltr.Letters * ltr.Letters {
		ltr.Triple = append(ltr.Triple, readSet())
	}
	return ltr
}

// ltrMaxValue allows for the rounding of cumulative sums: the largest value in the game files is
// 1.0000001.
const ltrMaxValue = 1.000001

// ltrDrops counts the values that are smaller than the last nonzero value before them in the same table.
// In a cumulative table this never happens.
func ltrDrops(values []float32) int {
	drops := 0
	last := float32(0)
	for _, value := range values {
		if value != 0 && value < last {
			drops++
		}
		if value != 0 {
			last = value
		}
	}
	return drops
}

func ReportLtr(label string, ltr *Ltr) string {
	if len(ltr.Errors) > 0 {
		return label + " " + Verdict(ltr.Errors...)
	}

	used, drops := 0, 0
	sets := append(append([]LtrSet{ltr.Single}, ltr.Double...), ltr.Triple...)
	for _, set := range sets {
		for _, table := range set {
			if ltrDrops(table) > 0 {
				drops++
			}
			for _, value := range table {
				if value != 0 {
					used++
					break
				}
			}
		}
	}
	return fmt.Sprintf("%s letters=%d tables=%d used=%d non_cumulative=%d PASS", label, ltr.Letters, len(sets)*len(LtrPositions), used, drops)
}
