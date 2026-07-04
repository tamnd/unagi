package runtime

// The int() string parser. CPython accepts optional sign, base prefixes,
// single underscores between digits, and any Unicode Nd digit, then
// enforces the 4300-digit conversion limit for the non-binary bases.
// Every accepted and rejected shape here is probed on 3.14.

import (
	"math/big"
	"strings"
	"unicode"

	"github.com/tamnd/unagi/pkg/objects"
)

// digitValue maps a rune to its numeric value, or -1. ASCII letters
// cover bases up to 36; any Unicode decimal digit counts too, so
// int("１２３") is 123.
func digitValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'z':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'Z':
		return int(r-'A') + 10
	}
	return ndValue(r)
}

// ndValue resolves a Unicode Nd digit to 0..9. The Nd table's runs
// start at each script's zero, so the offset inside a run mod 10 is the
// digit value even when adjacent runs merge into one range.
func ndValue(r rune) int {
	if r < 0x80 || !unicode.Is(unicode.Nd, r) {
		return -1
	}
	for _, rng := range unicode.Nd.R16 {
		if r <= rune(rng.Hi) && r >= rune(rng.Lo) {
			return (int(r-rune(rng.Lo)) / int(rng.Stride)) % 10
		}
	}
	for _, rng := range unicode.Nd.R32 {
		if r <= rune(rng.Hi) && r >= rune(rng.Lo) {
			return (int(r-rune(rng.Lo)) / int(rng.Stride)) % 10
		}
	}
	return -1
}

// asciiDigits rewrites every Unicode Nd digit to its ASCII form so the
// float parser sees plain digits.
func asciiDigits(s string) string {
	needs := false
	for _, r := range s {
		if r >= 0x80 && ndValue(r) >= 0 {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if v := ndValue(r); v >= 0 && r >= 0x80 {
			b.WriteByte(byte('0' + v))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// intFromStr parses an int literal for int(). orig is the argument
// object for the error repr, givenBase the base as passed (0 stays 0 in
// the error text) and digitBase the effective base before any prefix.
func intFromStr(orig objects.Object, s string, givenBase, digitBase int64) (objects.Object, error) {
	invalid := func() error {
		return objects.Raise(objects.ValueError,
			"invalid literal for int() with base %d: %s", givenBase, objects.Repr(orig))
	}
	rs := []rune(strings.TrimFunc(s, unicode.IsSpace))
	i := 0
	neg := false
	if i < len(rs) && (rs[i] == '+' || rs[i] == '-') {
		neg = rs[i] == '-'
		i++
	}
	base := digitBase
	prefixed := false
	if i+1 < len(rs) && rs[i] == '0' {
		var want int64
		switch rs[i+1] {
		case 'x', 'X':
			want = 16
		case 'o', 'O':
			want = 8
		case 'b', 'B':
			want = 2
		}
		if want != 0 && (givenBase == 0 || givenBase == want) {
			base = want
			prefixed = true
			i += 2
		}
	}
	// One underscore may follow the prefix: int("0x_1f", 0) is 31, but a
	// bare "_12" or "1__2" is invalid.
	var digits []byte
	lastDigit := false
	afterPrefix := prefixed
	for ; i < len(rs); i++ {
		r := rs[i]
		if r == '_' {
			if !lastDigit && !afterPrefix {
				return nil, invalid()
			}
			lastDigit = false
			afterPrefix = false
			continue
		}
		v := digitValue(r)
		if v < 0 || int64(v) >= base {
			return nil, invalid()
		}
		digits = append(digits, "0123456789abcdefghijklmnopqrstuvwxyz"[v])
		lastDigit = true
		afterPrefix = false
	}
	if !lastDigit {
		// Empty, sign-only, prefix-only or trailing-underscore input.
		return nil, invalid()
	}
	if givenBase == 0 && !prefixed && digits[0] == '0' {
		// Base 0 keeps the source rule: no leading zeros on a nonzero
		// number, so int("019", 0) fails while int("00", 0) is 0.
		for _, d := range digits {
			if d != '0' {
				return nil, invalid()
			}
		}
	}
	// The conversion limit applies to the quadratic-time bases only:
	// int("1"*4301) raises while int("f"*5000, 16) parses.
	if base&(base-1) != 0 && len(digits) > 4300 {
		return nil, objects.Raise(objects.ValueError,
			"Exceeds the limit (4300 digits) for integer string conversion: value has %d digits; use sys.set_int_max_str_digits() to increase the limit",
			len(digits))
	}
	b, ok := new(big.Int).SetString(string(digits), int(base))
	if !ok {
		return nil, invalid()
	}
	if neg {
		b.Neg(b)
	}
	return objects.NewIntFromBig(b), nil
}
