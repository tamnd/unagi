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

func reprSeqCore(elts []Object, open, close string, strict bool) (string, error) {
	var b strings.Builder
	b.WriteString(open)
	for i, e := range elts {
		if i > 0 {
			b.WriteString(", ")
		}
		s, err := reprCore(e, strict)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	b.WriteString(close)
	return b.String(), nil
}

// Repr returns the Python repr of an object. This infallible form serves
// error messages and internal rendering; it ignores the 4300-digit int
// conversion limit, which only the user-visible boundaries enforce.
func Repr(o Object) string {
	s, _ := reprCore(o, false)
	return s
}

// ReprE is repr() as user code reaches it: identical to Repr except that
// an int (anywhere in a container tree) past the 4300-digit conversion
// limit raises the probed ValueError.
func ReprE(o Object) (string, error) {
	return reprCore(o, true)
}

func reprCore(o Object, strict bool) (string, error) {
	switch x := o.(type) {
	case *noneObject:
		return "None", nil
	case *boolObject:
		if x.v {
			return "True", nil
		}
		return "False", nil
	case *intObject:
		if strict {
			return intDecimal(x)
		}
		return intDecimalLoose(x), nil
	case *floatObject:
		return floatRepr(x.v), nil
	case *strObject:
		return strRepr(x.v), nil
	case *listObject:
		return reprSeqCore(x.elts, "[", "]", strict)
	case *tupleObject:
		if len(x.elts) == 1 {
			s, err := reprCore(x.elts[0], strict)
			if err != nil {
				return "", err
			}
			return "(" + s + ",)", nil
		}
		return reprSeqCore(x.elts, "(", ")", strict)
	case *dictObject:
		var b strings.Builder
		b.WriteString("{")
		for i, e := range x.entries {
			if i > 0 {
				b.WriteString(", ")
			}
			k, err := reprCore(e.key, strict)
			if err != nil {
				return "", err
			}
			v, err := reprCore(e.val, strict)
			if err != nil {
				return "", err
			}
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
		}
		b.WriteString("}")
		return b.String(), nil
	case *setObject:
		// Probed: repr(set()) is "set()", repr({1,2}) is "{1, 2}".
		if len(x.elts) == 0 {
			return "set()", nil
		}
		return reprSeqCore(x.elts, "{", "}", strict)
	case *frozensetObject:
		// Probed: "frozenset()" empty, "frozenset({1, 2})" otherwise.
		if len(x.elts) == 0 {
			return "frozenset()", nil
		}
		s, err := reprSeqCore(x.elts, "{", "}", strict)
		if err != nil {
			return "", err
		}
		return "frozenset(" + s + ")", nil
	case *rangeObject:
		if x.step == 1 {
			return fmt.Sprintf("range(%d, %d)", x.start, x.stop), nil
		}
		return fmt.Sprintf("range(%d, %d, %d)", x.start, x.stop, x.step), nil
	case *funcObject:
		return fmt.Sprintf("<function %s at %p>", x.name, x), nil
	case *functionObject:
		// Probed: repr spells __qualname__, g.<locals>.<lambda> and all.
		return fmt.Sprintf("<function %s at %p>", x.qual, x), nil
	case *dictKeysObject:
		return reprSeqCore(x.d.keySlice(), "dict_keys([", "])", strict)
	case *dictValuesObject:
		return reprSeqCore(x.d.valSlice(), "dict_values([", "])", strict)
	case *dictItemsObject:
		return reprSeqCore(x.d.itemSlice(), "dict_items([", "])", strict)
	case *Exception:
		// ClassName(args...) with repr-joined args, ValueError('boom').
		s, err := reprSeqCore(x.Args, "(", ")", strict)
		if err != nil {
			return "", err
		}
		return x.Kind + s, nil
	case *classObject:
		return classRepr(x), nil
	case *instanceObject:
		return instanceRepr(x), nil
	case *boundMethod:
		return boundMethodRepr(x), nil
	case *superObject:
		return superRepr(x), nil
	case *excTypeObject:
		return excTypeRepr(x), nil
	}
	return fmt.Sprintf("<%s object>", o.TypeName()), nil
}

// Str returns the Python str of an object. Strings come back raw,
// exceptions render their message, everything else falls through to Repr.
// Like Repr, this form skips the 4300-digit int conversion limit.
func Str(o Object) string {
	s, _ := strCore(o, false)
	return s
}

// StrE is str() as user code reaches it, enforcing the 4300-digit int
// conversion limit that print, str() and f-strings all hit.
func StrE(o Object) (string, error) {
	return strCore(o, true)
}

func strCore(o Object, strict bool) (string, error) {
	switch x := o.(type) {
	case *strObject:
		return x.v, nil
	case *Exception:
		return x.Text(), nil
	}
	return reprCore(o, strict)
}
