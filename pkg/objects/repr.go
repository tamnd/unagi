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

// dictBodyRepr spells the {k: v, ...} body shared by a plain dict and the
// trailing part of a defaultdict repr.
func dictBodyRepr(d *dictObject, strict bool) (string, error) {
	var b strings.Builder
	b.WriteString("{")
	for i, e := range d.entries {
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

// Ascii is ascii(): the repr with every non-ASCII rune replaced by its
// backslash escape. CPython's ascii escapes only appear where the repr has a
// non-ASCII rune (a string's contents, an identifier), and any backslash the
// repr already produced for a control character is plain ASCII, so escaping
// each rune >= 0x80 in the repr string reproduces ascii() exactly: \xHH up to
// 0xff, \uHHHH up to 0xffff, else \UHHHHHHHH, lowercase hex like repr.
func Ascii(o Object) (string, error) {
	s, err := ReprE(o)
	if err != nil {
		return "", err
	}
	if isASCII(s) {
		return s, nil
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r <= 0xffff:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			fmt.Fprintf(&b, "\\U%08x", r)
		}
	}
	return b.String(), nil
}

// isASCII reports whether s is all ASCII, the common case that skips the
// escape rebuild.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func reprCore(o Object, strict bool) (string, error) {
	switch x := o.(type) {
	case *noneObject:
		return "None", nil
	case *ellipsisObject:
		return "Ellipsis", nil
	case *notImplementedObject:
		return "NotImplemented", nil
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
	case *complexObject:
		return complexRepr(x.re, x.im), nil
	case *strObject:
		return strRepr(x.v), nil
	case *bytesObject:
		return bytesRepr(x.v), nil
	case *bytearrayObject:
		return bytearrayRepr(x.snapshot()), nil
	case *memoryviewObject:
		return memoryviewRepr(x), nil
	case *quitterObject:
		return fmt.Sprintf("Use %s() or Ctrl-D (i.e. EOF) to exit", x.name), nil
	case *printerObject:
		return x.reprText, nil
	case *listObject:
		return reprSeqCore(x.elts, "[", "]", strict)
	case *dequeObject:
		return dequeRepr(x, strict)
	case *tupleObject:
		if x.named != nil {
			return namedTupleRepr(x, strict)
		}
		if len(x.elts) == 1 {
			s, err := reprCore(x.elts[0], strict)
			if err != nil {
				return "", err
			}
			return "(" + s + ",)", nil
		}
		return reprSeqCore(x.elts, "(", ")", strict)
	case *dictObject:
		switch x.kind {
		case defaultDict:
			return defaultDictRepr(x, strict)
		case counterDict:
			return counterRepr(x, strict)
		case orderedDict:
			return orderedDictRepr(x, strict)
		}
		return dictBodyRepr(x, strict)
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
	case *sliceObject:
		// Probed on 3.14: repr(slice(1, 10, 2)) is "slice(1, 10, 2)" and an
		// omitted part prints as None, so slice(5) is "slice(None, 5, None)".
		start, err := reprCore(x.start, strict)
		if err != nil {
			return "", err
		}
		stop, err := reprCore(x.stop, strict)
		if err != nil {
			return "", err
		}
		step, err := reprCore(x.step, strict)
		if err != nil {
			return "", err
		}
		return "slice(" + start + ", " + stop + ", " + step + ")", nil
	case *funcObject:
		// A builtin passed around as a value reprs the way CPython does: the
		// type constructors as classes, the plain builtins as built-in
		// functions. Internal helper funcObjects keep the generic form.
		if builtinTypeReprs[x.name] {
			return fmt.Sprintf("<class '%s'>", x.name), nil
		}
		if builtinFuncReprs[x.name] {
			return fmt.Sprintf("<built-in function %s>", x.name), nil
		}
		if objectSlotWrappers[x.name] {
			return fmt.Sprintf("<slot wrapper '%s' of 'object' objects>", x.name), nil
		}
		return fmt.Sprintf("<function %s at %p>", x.name, x), nil
	case *functionObject:
		// Probed: repr spells __qualname__, g.<locals>.<lambda> and all.
		return fmt.Sprintf("<function %s at %p>", x.qual, x), nil
	case *partialObject:
		return partialRepr(x, strict)
	case *patternObject:
		return patternRepr(x, strict)
	case *staticmethodObject, *classmethodObject, *propertyObject, *cachedPropertyObject:
		return descriptorRepr(x), nil
	case *memberDescriptor:
		return fmt.Sprintf("<member '%s' of '%s' objects>", x.name, x.owner.name), nil
	case *Module:
		return fmt.Sprintf("<module '%s' from '%s'>", x.name, x.file), nil
	case *dictKeysObject:
		return reprSeqCore(x.d.keySlice(), "dict_keys([", "])", strict)
	case *dictValuesObject:
		return reprSeqCore(x.d.valSlice(), "dict_values([", "])", strict)
	case *dictItemsObject:
		return reprSeqCore(x.d.itemSlice(), "dict_items([", "])", strict)
	case *Exception:
		// A user exception subclass may override __repr__; otherwise the default
		// is ClassName(args...) with repr-joined args, ValueError('boom').
		if s, ok, err := excSpecialStr(x, "__repr__"); ok {
			return s, err
		}
		s, err := reprSeqCore(x.Args, "(", ")", strict)
		if err != nil {
			return "", err
		}
		return x.Kind + s, nil
	case *classObject:
		return classRepr(x), nil
	case *typeObject:
		// A type value for a constructor-less kind, spelled the same as a class.
		return fmt.Sprintf("<class '%s'>", x.name), nil
	case *instanceObject:
		res, defined, err := instanceSpecial(x, "__repr__")
		if err != nil {
			return "", err
		}
		if !defined {
			return instanceRepr(x), nil
		}
		s, ok := res.(*strObject)
		if !ok {
			return "", Raise(TypeError, "__repr__ returned non-string (type %s)", res.TypeName())
		}
		return s.v, nil
	case *boundMethod:
		return boundMethodRepr(x), nil
	case *superObject:
		return superRepr(x), nil
	case *stringIOObject:
		return stringIORepr(x), nil
	case *bytesIOObject:
		return bytesIORepr(x), nil
	case *generatorObject:
		kind := "generator"
		switch {
		case x.isAsyncGen:
			kind = "async_generator"
		case x.isCoro:
			kind = "coroutine"
		}
		return fmt.Sprintf("<%s object %s at %p>", kind, x.qual, x), nil
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
		// A user exception subclass may override __str__; otherwise str is the
		// message rendered from args, BaseException.__str__.
		if s, ok, err := excSpecialStr(x, "__str__"); ok {
			return s, err
		}
		return x.Text(), nil
	case *instanceObject:
		res, defined, err := instanceSpecial(x, "__str__")
		if err != nil {
			return "", err
		}
		if !defined {
			// object.__str__ delegates to __repr__, which reprCore dispatches.
			return reprCore(x, strict)
		}
		s, ok := res.(*strObject)
		if !ok {
			return "", Raise(TypeError, "__str__ returned non-string (type %s)", res.TypeName())
		}
		return s.v, nil
	}
	return reprCore(o, strict)
}
