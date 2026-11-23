package kotor

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// The JavaScript scripts that came before these commands formatted numbers with Number.prototype.toFixed
// and Number.prototype.toString. The functions here follow those algorithms exactly, so that reports,
// OBJ files and JSON files keep the same bytes.

// toFixed switches to toString at and above this magnitude.
const fixedLimit = 1e21

// Bits of precision for the exact products in roundsHalfUp. A float64 has 53 bits and 10^d needs
// about 3.4 d bits, so 256 bits keeps every product exact for the digit counts used here.
const exactPrecision = 256

// ToFixed formats x with the given number of digits after the decimal point, like Number.prototype.toFixed.
// strconv rounds an exact tie to the even digit, but toFixed rounds it away from zero, so ties are corrected.
func ToFixed(x float64, digits int) string {
	if math.IsNaN(x) {
		return "NaN"
	}
	if math.Abs(x) >= fixedLimit {
		return NumberString(x)
	}

	text := strconv.FormatFloat(x, 'f', digits, 64)

	// An exact tie shows as a trailing 5 one digit further, and that longer form is then exact.
	longer := strconv.FormatFloat(x, 'f', digits+1, 64)
	if strings.HasSuffix(longer, "5") && isExactDecimal(x, digits+1) {
		text = incrementLastDigit(strings.TrimSuffix(strings.TrimSuffix(longer, "5"), "."))
	}

	// toFixed writes no sign for negative zero.
	if x == 0 {
		text = strings.TrimPrefix(text, "-")
	}
	return text
}

// isExactDecimal reports whether x has a finite decimal form with at most the given number of fraction digits.
func isExactDecimal(x float64, digits int) bool {
	scale := new(big.Float).SetPrec(exactPrecision).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil))
	scaled := new(big.Float).SetPrec(exactPrecision).Mul(new(big.Float).SetFloat64(x), scale)
	return scaled.IsInt()
}

// incrementLastDigit adds one unit in the last place to a decimal string such as "-12.34", carrying as needed.
func incrementLastDigit(text string) string {
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign = "-"
		text = text[1:]
	}

	digits := []byte(text)
	for i := len(digits) - 1; i >= 0; i-- {
		if digits[i] == '.' {
			continue
		}
		if digits[i] < '9' {
			digits[i]++
			return sign + string(digits)
		}
		digits[i] = '0'
	}
	return sign + "1" + string(digits)
}

// RoundSignificant rounds x to the given number of significant digits the way Number.prototype.toPrecision
// does, rounding an exact tie away from zero, and returns the value of the resulting decimal.
func RoundSignificant(x float64, digits int) float64 {
	if x == 0 || math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}

	// Every float64 has a finite decimal form of at most 767 significant digits, so this text is exact.
	const exactDigits = 1100
	text := new(big.Float).SetFloat64(math.Abs(x)).Text('e', exactDigits)
	mantissa, exponent, _ := strings.Cut(text, "e")
	mantissa = strings.Replace(mantissa, ".", "", 1)

	kept := mantissa[:digits]
	if mantissa[digits] >= '5' {
		kept = incrementLastDigit(kept)
	}

	value, _ := strconv.ParseFloat(kept[:1]+"."+kept[1:]+"e"+exponent, 64)
	if len(kept) > digits {
		// The carry added a digit, as in 9.99 -> 10.0. The exponent already accounts for the original
		// digits, so the longer mantissa is one power of ten too large.
		value, _ = strconv.ParseFloat("1e"+strconv.Itoa(mustAtoi(exponent)+1), 64)
	}
	if x < 0 {
		value = -value
	}
	return value
}

func mustAtoi(text string) int {
	value, _ := strconv.Atoi(text)
	return value
}

// NumberString formats x like Number.prototype.toString with radix 10: the shortest digits that round-trip,
// in plain notation from 1e-7 up to 1e21 and in exponent notation outside that range.
func NumberString(x float64) string {
	switch {
	case math.IsNaN(x):
		return "NaN"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	case x == 0:
		return "0"
	}

	sign := ""
	if x < 0 {
		sign = "-"
		x = -x
	}

	// The 'e' form is d.ddde±XX; n is the position of the decimal point relative to the digits.
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(x, 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	k := len(digits)
	n := mustAtoi(exponent) + 1

	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}

	exponentSign := "+"
	if n-1 < 0 {
		exponentSign = "-"
	}
	exponentText := exponentSign + strconv.Itoa(abs(n-1))
	if k == 1 {
		return sign + digits + "e" + exponentText
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + exponentText
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
