package objects

// Percent formatting: the % operator with a str left operand. The whole
// surface is probed on CPython 3.14, output shapes and error texts
// alike, never guessed. The float bodies reuse the verified helpers in
// format.go (fixedFloat, sciFloat, generalFloat), which already match
// the printf-style e/f/g rules CPython uses here.

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// percentState is the argument cursor. CPython treats a tuple right
// operand as the positional argument list and anything else as a single
// argument; a %(key)s lookup replaces the cursor with just the value,
// which is why "%(k)*d" runs out of arguments after * eats the value.
type percentState struct {
	args []Object
	idx  int
	// mapping is set when the right operand subscripts by string key.
	// CPython's check is PyMapping_Check minus tuple and str, so dict,
	// list and range all pass; probed: "" % [1] is '' with no leftover
	// complaint and "%(k)s" % [1] fails inside the list subscript.
	mapping Object
}

func (st *percentState) next() (Object, error) {
	if st.idx >= len(st.args) {
		// Probed on 3.14: "%s %s" % (1,) and "%s %s" % "ab".
		return nil, Raise(TypeError, "not enough arguments for format string")
	}
	v := st.args[st.idx]
	st.idx++
	return v, nil
}

// percentFormat implements format % right for a str format.
func percentFormat(format string, right Object) (Object, error) {
	st := &percentState{}
	if t, ok := right.(*tupleObject); ok {
		st.args = t.elts
	} else {
		st.args = []Object{right}
	}
	switch right.(type) {
	case *dictObject, *listObject, *rangeObject:
		st.mapping = right
	}
	rs := []rune(format)
	var out strings.Builder
	i := 0
	for i < len(rs) {
		if rs[i] != '%' {
			out.WriteRune(rs[i])
			i++
			continue
		}
		i++
		if i >= len(rs) {
			// Probed on 3.14: "%" % 5 and "abc%" % ().
			return nil, Raise(ValueError, "incomplete format")
		}
		// A doubled % right after the first is the literal escape. Any
		// flags or width in between kill it: "%5%" is an unsupported
		// conversion, not a padded percent. Probed on 3.14.
		if rs[i] == '%' {
			out.WriteByte('%')
			i++
			continue
		}
		if rs[i] == '(' {
			// The mapping check comes before the key scan; probed on
			// 3.14, "%(k" % 5 says mapping, not incomplete key.
			if st.mapping == nil {
				return nil, Raise(TypeError, "format requires a mapping")
			}
			depth := 1
			j := i + 1
			for j < len(rs) && depth > 0 {
				switch rs[j] {
				case '(':
					depth++
				case ')':
					depth--
				}
				j++
			}
			if depth > 0 {
				// Probed on 3.14: "%(k" % {"k": 1}.
				return nil, Raise(ValueError, "incomplete format key")
			}
			key := string(rs[i+1 : j-1])
			v, err := GetItem(st.mapping, NewStr(key))
			if err != nil {
				// KeyError for a dict, the subscript TypeError for a
				// list or range; both pass through untouched.
				return nil, err
			}
			st.args = []Object{v}
			st.idx = 0
			i = j
		}
		var left, plus, space, alt, zero bool
	flags:
		for i < len(rs) {
			switch rs[i] {
			case '-':
				left = true
			case '+':
				plus = true
			case ' ':
				space = true
			case '#':
				alt = true
			case '0':
				zero = true
			default:
				break flags
			}
			i++
		}
		width := 0
		if i < len(rs) && rs[i] == '*' {
			i++
			n, err := starArg(st, "Python int too large to convert to C ssize_t")
			if err != nil {
				return nil, err
			}
			width = n
			// A negative * width flips to left justification. Probed on
			// 3.14: "%*d" % (-5, 42) is '42   '.
			if width < 0 {
				left = true
				width = -width
			}
		} else {
			for i < len(rs) && rs[i] >= '0' && rs[i] <= '9' {
				width = width*10 + int(rs[i]-'0')
				i++
			}
		}
		prec := -1
		if i < len(rs) && rs[i] == '.' {
			i++
			// A bare '.' means precision zero: "%.f" % 3.7 is '4'.
			prec = 0
			if i < len(rs) && rs[i] == '*' {
				i++
				// Probed: a big * precision says C int, not ssize_t.
				n, err := starArg(st, "Python int too large to convert to C int")
				if err != nil {
					return nil, err
				}
				prec = n
				// A negative * precision clamps to zero, it does not go
				// back to the default. Probed on 3.14: "%.*f" % (-2,
				// 3.14159) is '3' and "%.*s" % (-2, "hello") is ''.
				if prec < 0 {
					prec = 0
				}
			} else {
				for i < len(rs) && rs[i] >= '0' && rs[i] <= '9' {
					prec = prec*10 + int(rs[i]-'0')
					i++
				}
			}
		}
		// Exactly one C length modifier is tolerated and ignored.
		// Probed on 3.14: "%ld" % 5 is '5' but "%lld" % 5 is the
		// unsupported-character error on the second 'l'.
		if i < len(rs) && (rs[i] == 'h' || rs[i] == 'l' || rs[i] == 'L') {
			i++
		}
		if i >= len(rs) {
			return nil, Raise(ValueError, "incomplete format")
		}
		c := rs[i]
		cIdx := i
		i++
		// The argument is fetched before the conversion character is
		// validated. Probed on 3.14: "%y" % () says not enough
		// arguments, "%y" % 5 says unsupported format character.
		v, err := st.next()
		if err != nil {
			return nil, err
		}
		var sign rune
		if plus {
			sign = '+'
		} else if space {
			sign = ' '
		}
		switch c {
		case 's', 'r', 'a':
			var text string
			var terr error
			switch c {
			case 's':
				text, terr = StrE(v)
			case 'r':
				text, terr = ReprE(v)
			default:
				text, terr = ReprE(v)
				text = asciiEscape(text)
			}
			if terr != nil {
				// Probed: "%s" % 2**20000 hits the 4300-digit limit.
				return nil, terr
			}
			// Precision truncates; sign, # and 0 flags are silently
			// ignored for the string conversions. Probed on 3.14:
			// "%.2r" % "hello" is "'h" and "%010s" % "ab" space-pads.
			if prec >= 0 {
				tr := []rune(text)
				if prec < len(tr) {
					text = string(tr[:prec])
				}
			}
			out.WriteString(padPercent("", "", text, width, left, false))
		case 'c':
			text, cerr := percentChar(v)
			if cerr != nil {
				return nil, cerr
			}
			// Precision is ignored and the 0 flag stays a space pad.
			// Probed on 3.14: "%.3c" % 65 is 'A', "%05c" % 65 is '    A'.
			out.WriteString(padPercent("", "", text, width, left, false))
		case 'd', 'i', 'u':
			neg, digits, derr := percentDecimal(v, c)
			if derr != nil {
				return nil, derr
			}
			out.WriteString(renderPercentInt(neg, digits, c, alt, prec, sign, width, left, zero))
		case 'o', 'x', 'X':
			neg, digits, ok := percentBaseDigits(v, c)
			if !ok {
				// Probed on 3.14: "%x" % 3.7, floats are not truncated
				// here the way %d does.
				return nil, Raise(TypeError, "%%%c format: an integer is required, not %s", c, v.TypeName())
			}
			out.WriteString(renderPercentInt(neg, digits, c, alt, prec, sign, width, left, zero))
		case 'e', 'E', 'f', 'F', 'g', 'G':
			f, ok := AsFloat(v)
			if !ok {
				// Probed on 3.14: "%f" % "x".
				return nil, Raise(TypeError, "must be real number, not %s", v.TypeName())
			}
			if prec < 0 {
				prec = 6
			}
			a := math.Abs(f)
			var body string
			switch c {
			case 'f', 'F':
				body = fixedFloat(a, prec, alt)
			case 'e', 'E':
				body = sciFloat(a, prec, alt)
			default:
				body = generalFloat(a, prec, alt, false)
			}
			if c == 'E' || c == 'F' || c == 'G' {
				body = strings.ToUpper(body)
			}
			// The 0 flag zero-pads even inf and nan. Probed on 3.14:
			// "%010f" % float('inf') is '0000000inf'.
			out.WriteString(padPercent(signStr(math.Signbit(f), sign), "", body, width, left, zero))
		default:
			disp := c
			if c <= 31 || c >= 128 {
				// Non-printable and non-ASCII characters show as '?'.
				// Probed on 3.14: "%é" % 5 says '?' (0xe9).
				disp = '?'
			}
			return nil, Raise(ValueError, "unsupported format character '%c' (0x%x) at index %d", disp, c, cIdx)
		}
	}
	// Leftover positional arguments only complain without a mapping.
	// Probed on 3.14: "" % 5 raises, "" % {"a": 1} is ''.
	if st.idx < len(st.args) && st.mapping == nil {
		return nil, Raise(TypeError, "not all arguments converted during string formatting")
	}
	return NewStr(out.String()), nil
}

// starArg fetches a * width or precision argument, which must be an int
// or bool. Probed on 3.14: "%*d" % (5.0, 42) is "* wants int" while
// True works as 1. overflowMsg is the wording for a spilled int, which
// differs between the width (ssize_t) and precision (int) positions.
func starArg(st *percentState, overflowMsg string) (int, error) {
	v, err := st.next()
	if err != nil {
		return 0, err
	}
	n, ok := AsInt(v)
	if !ok {
		if IsBigInt(v) {
			return 0, Raise(OverflowError, "%s", overflowMsg)
		}
		return 0, Raise(TypeError, "* wants int")
	}
	return int(n), nil
}

// percentDecimal pulls the digits for %d %i %u as sign and magnitude.
// Floats truncate toward zero; probed on 3.14, "%d" % 3.7 is '3' and
// "%d" % -3.7 is '-3'. A spilled int hits the 4300-digit limit here,
// "%d" % 2**20000 raises, and a huge float renders exactly, "%d" % 1e308
// is the full 309-digit value.
func percentDecimal(v Object, c rune) (bool, string, error) {
	if x, ok := v.(*intObject); ok && x.big != nil {
		s, err := intDecimal(x)
		if err != nil {
			return false, "", err
		}
		return x.big.Sign() < 0, strings.TrimPrefix(s, "-"), nil
	}
	if n, ok := AsInt(v); ok {
		neg := n < 0
		u := uint64(n)
		if neg {
			u = -u
		}
		return neg, strconv.FormatUint(u, 10), nil
	}
	if f, ok := v.(*floatObject); ok {
		if math.IsInf(f.v, 0) {
			return false, "", Raise(OverflowError, "cannot convert float infinity to integer")
		}
		if math.IsNaN(f.v) {
			return false, "", Raise(ValueError, "cannot convert float NaN to integer")
		}
		t := math.Trunc(f.v)
		if t >= -9.2e18 && t <= 9.2e18 {
			n := int64(t)
			neg := n < 0
			u := uint64(n)
			if neg {
				u = -u
			}
			return neg, strconv.FormatUint(u, 10), nil
		}
		b, _ := new(big.Float).SetFloat64(t).Int(nil)
		return b.Sign() < 0, new(big.Int).Abs(b).String(), nil
	}
	// The message names the conversion actually used: "%i format: ...".
	return false, "", Raise(TypeError, "%%%c format: a real number is required, not %s", c, v.TypeName())
}

// percentBaseDigits pulls sign and magnitude for %o %x %X, which take
// ints of any size; probed, "%x" % 2**20000 is exempt from the digit
// limit. ok is false for every non-int, floats included.
func percentBaseDigits(v Object, c rune) (bool, string, bool) {
	base := 8
	if c == 'x' || c == 'X' {
		base = 16
	}
	if x, ok := v.(*intObject); ok && x.big != nil {
		return x.big.Sign() < 0, new(big.Int).Abs(x.big).Text(base), true
	}
	n, ok := AsInt(v)
	if !ok {
		return false, "", false
	}
	neg := n < 0
	u := uint64(n)
	if neg {
		u = -u
	}
	return neg, strconv.FormatUint(u, base), true
}

// renderPercentInt renders d i u o x X with sign-magnitude negatives,
// precision zero-extension and the alternate prefixes. Unlike C printf
// the 0 flag still applies with a precision: probed on 3.14,
// "%08.5d" % 42 is '00000042'.
func renderPercentInt(neg bool, digits string, c rune, alt bool, prec int, sign rune, width int, left, zero bool) string {
	if c == 'X' {
		digits = strings.ToUpper(digits)
	}
	if prec > len(digits) {
		digits = strings.Repeat("0", prec-len(digits)) + digits
	}
	prefix := ""
	if alt {
		// Probed on 3.14: 0o, 0x, 0X, and '#' adds nothing to %d.
		switch c {
		case 'o':
			prefix = "0o"
		case 'x':
			prefix = "0x"
		case 'X':
			prefix = "0X"
		}
	}
	return padPercent(signStr(neg, sign), prefix, digits, width, left, zero)
}

// percentChar renders %c: a code point or a one-character string.
func percentChar(v Object) (string, error) {
	// Probed: "%c" % 2**100 gives the range error, not a type error.
	if IsBigInt(v) {
		return "", Raise(OverflowError, "%%c arg not in range(0x110000)")
	}
	if n, ok := AsInt(v); ok {
		if n < 0 || n > 0x10FFFF {
			// Probed on 3.14: "%c" % -1 and "%c" % 0x110000.
			return "", Raise(OverflowError, "%%c arg not in range(0x110000)")
		}
		return string(rune(n)), nil
	}
	if s, ok := AsStr(v); ok {
		rn := []rune(s)
		if len(rn) != 1 {
			// Probed on 3.14: "%c" % "AB" and "%c" % "".
			return "", Raise(TypeError, "%%c requires an int or a unicode character, not a string of length %d", len(rn))
		}
		return s, nil
	}
	// Floats are not accepted here. Probed on 3.14: "%c" % 3.5.
	return "", Raise(TypeError, "%%c requires an int or a unicode character, not %s", v.TypeName())
}

// asciiEscape turns a repr into its ascii() form: every non-ASCII rune
// becomes \xhh, \uhhhh or \Uhhhhhhhh with lowercase hex. Probed on
// 3.14: ascii('é') is \xe9 and ascii('\U0001F600') is \U0001f600.
func asciiEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	return b.String()
}

// padPercent pads sign+prefix+body to width in code points. The '-'
// flag wins over '0' and pads with spaces on the right; the 0 fill goes
// between the prefix and the body, "%#05x" % 255 is '0x0ff'.
func padPercent(sign, prefix, body string, width int, left, zero bool) string {
	pad := width - utf8.RuneCountInString(sign) - len(prefix) - utf8.RuneCountInString(body)
	if pad <= 0 {
		return sign + prefix + body
	}
	switch {
	case left:
		return sign + prefix + body + strings.Repeat(" ", pad)
	case zero:
		return sign + prefix + strings.Repeat("0", pad) + body
	}
	return strings.Repeat(" ", pad) + sign + prefix + body
}
