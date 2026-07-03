package objects

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// floatRepr formats a float the way CPython repr does: shortest digits
// that round-trip, fixed notation for exponents in [-4, 16), scientific
// otherwise, and always a decimal point or an exponent.
func floatRepr(v float64) string {
	if math.IsInf(v, 1) {
		return "inf"
	}
	if math.IsInf(v, -1) {
		return "-inf"
	}
	if math.IsNaN(v) {
		return "nan"
	}
	s := strconv.FormatFloat(v, 'e', -1, 64)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	ePos := strings.IndexByte(s, 'e')
	digits := strings.Replace(s[:ePos], ".", "", 1)
	exp, _ := strconv.Atoi(s[ePos+1:])
	var out string
	switch {
	case exp >= 16 || exp < -4:
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		sign := "+"
		if exp < 0 {
			sign = "-"
			exp = -exp
		}
		out = fmt.Sprintf("%se%s%02d", m, sign, exp)
	case exp >= 0:
		if len(digits) > exp+1 {
			out = digits[:exp+1] + "." + digits[exp+1:]
		} else {
			out = digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
		}
	default:
		out = "0." + strings.Repeat("0", -exp-1) + digits
	}
	if neg {
		out = "-" + out
	}
	return out
}

// strRepr quotes a string like CPython repr: single quotes normally,
// double quotes when the value has a single quote but no double quote.
func strRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func reprSeq(elts []Object, open, close string) string {
	var b strings.Builder
	b.WriteString(open)
	for i, e := range elts {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(Repr(e))
	}
	b.WriteString(close)
	return b.String()
}

// Repr returns the Python repr of an object.
func Repr(o Object) string {
	switch x := o.(type) {
	case *noneObject:
		return "None"
	case *boolObject:
		if x.v {
			return "True"
		}
		return "False"
	case *intObject:
		return strconv.FormatInt(x.v, 10)
	case *floatObject:
		return floatRepr(x.v)
	case *strObject:
		return strRepr(x.v)
	case *listObject:
		return reprSeq(x.elts, "[", "]")
	case *tupleObject:
		if len(x.elts) == 1 {
			return "(" + Repr(x.elts[0]) + ",)"
		}
		return reprSeq(x.elts, "(", ")")
	case *dictObject:
		var b strings.Builder
		b.WriteString("{")
		for i, e := range x.entries {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(Repr(e.key))
			b.WriteString(": ")
			b.WriteString(Repr(e.val))
		}
		b.WriteString("}")
		return b.String()
	case *setObject:
		// Probed: repr(set()) is "set()", repr({1,2}) is "{1, 2}".
		if len(x.elts) == 0 {
			return "set()"
		}
		return reprSeq(x.elts, "{", "}")
	case *frozensetObject:
		// Probed: "frozenset()" empty, "frozenset({1, 2})" otherwise.
		if len(x.elts) == 0 {
			return "frozenset()"
		}
		return "frozenset(" + reprSeq(x.elts, "{", "}") + ")"
	case *rangeObject:
		if x.step == 1 {
			return fmt.Sprintf("range(%d, %d)", x.start, x.stop)
		}
		return fmt.Sprintf("range(%d, %d, %d)", x.start, x.stop, x.step)
	case *funcObject:
		return fmt.Sprintf("<function %s at %p>", x.name, x)
	case *dictKeysObject:
		return "dict_keys(" + reprSeq(x.d.keySlice(), "[", "]") + ")"
	case *dictValuesObject:
		return "dict_values(" + reprSeq(x.d.valSlice(), "[", "]") + ")"
	case *dictItemsObject:
		return "dict_items(" + reprSeq(x.d.itemSlice(), "[", "]") + ")"
	case *Exception:
		// ClassName(args...) with repr-joined args, ValueError('boom').
		return x.Kind + reprSeq(x.Args, "(", ")")
	}
	return fmt.Sprintf("<%s object>", o.TypeName())
}

// Str returns the Python str of an object. Strings come back raw,
// exceptions render their message, everything else falls through to Repr.
func Str(o Object) string {
	switch x := o.(type) {
	case *strObject:
		return x.v
	case *Exception:
		return x.Text()
	}
	return Repr(o)
}
