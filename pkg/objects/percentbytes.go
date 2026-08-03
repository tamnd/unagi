package objects

// Percent formatting for a bytes or bytearray left operand: b"%d" % 5. It
// mirrors the str version in percent.go but follows CPython's
// _PyBytes_FormatEx (Objects/bytesobject.c): %s and %b both want a bytes-like
// object or an __bytes__, %a and %r render the ascii() repr as bytes, %c takes
// an int in range(256) or a single byte, and the numeric conversions reuse the
// same verified helpers the str path does. The whole surface is probed on
// CPython 3.14, output shapes and error texts alike.

import (
	"bytes"
	"math"
	"strings"
)

// percentFormatBytes implements format % right for a bytes or bytearray format,
// returning the formatted bytes. The caller wraps them back in the left
// operand's type, so bytearray(b"%d") % 5 stays a bytearray.
func percentFormatBytes(format []byte, right Object) ([]byte, error) {
	st := &percentState{}
	if t, ok := asFormatTuple(right); ok {
		st.args = t.elts
	} else {
		st.args = []Object{right}
	}
	if isFormatMapping(right) {
		st.mapping = right
	}
	var out []byte
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			out = append(out, format[i])
			i++
			continue
		}
		i++
		if i >= len(format) {
			return nil, Raise(ValueError, "incomplete format")
		}
		if format[i] == '%' {
			out = append(out, '%')
			i++
			continue
		}
		if format[i] == '(' {
			if st.mapping == nil {
				return nil, Raise(TypeError, "format requires a mapping")
			}
			depth := 1
			j := i + 1
			for j < len(format) && depth > 0 {
				switch format[j] {
				case '(':
					depth++
				case ')':
					depth--
				}
				j++
			}
			if depth > 0 {
				return nil, Raise(ValueError, "incomplete format key")
			}
			// A bytes format subscripts its mapping by a bytes key.
			key := make([]byte, j-1-(i+1))
			copy(key, format[i+1:j-1])
			v, err := GetItem(st.mapping, NewBytes(key))
			if err != nil {
				return nil, err
			}
			st.args = []Object{v}
			st.idx = 0
			i = j
		}
		var left, plus, space, alt, zero bool
	flags:
		for i < len(format) {
			switch format[i] {
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
		if i < len(format) && format[i] == '*' {
			i++
			n, err := starArg(st, "Python int too large to convert to C ssize_t")
			if err != nil {
				return nil, err
			}
			width = n
			if width < 0 {
				left = true
				width = -width
			}
		} else {
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				width = width*10 + int(format[i]-'0')
				i++
			}
		}
		prec := -1
		if i < len(format) && format[i] == '.' {
			i++
			prec = 0
			if i < len(format) && format[i] == '*' {
				i++
				n, err := starArg(st, "Python int too large to convert to C int")
				if err != nil {
					return nil, err
				}
				prec = max(n, 0)
			} else {
				for i < len(format) && format[i] >= '0' && format[i] <= '9' {
					prec = prec*10 + int(format[i]-'0')
					i++
				}
			}
		}
		if i < len(format) && (format[i] == 'h' || format[i] == 'l' || format[i] == 'L') {
			i++
		}
		if i >= len(format) {
			return nil, Raise(ValueError, "incomplete format")
		}
		c := rune(format[i])
		cIdx := i
		i++
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
		case 'a', 'r':
			// Both render ascii(v) as bytes; precision truncates.
			text, terr := ReprE(v)
			if terr != nil {
				return nil, terr
			}
			text = asciiEscape(text)
			if prec >= 0 && prec < len(text) {
				text = text[:prec]
			}
			out = append(out, []byte(padPercent("", "", text, width, left, false))...)
		case 's', 'b':
			body, berr := bytesFormatArg(v)
			if berr != nil {
				return nil, berr
			}
			if prec >= 0 && prec < len(body) {
				body = body[:prec]
			}
			out = append(out, padPercentBytes("", "", body, width, left, false)...)
		case 'c':
			by, cerr := bytesByteConverter(v)
			if cerr != nil {
				return nil, cerr
			}
			out = append(out, padPercentBytes("", "", []byte{by}, width, left, false)...)
		case 'd', 'i', 'u':
			neg, digits, derr := percentDecimal(v, c)
			if derr != nil {
				return nil, derr
			}
			out = append(out, []byte(renderPercentInt(neg, digits, c, alt, prec, sign, width, left, zero))...)
		case 'o', 'x', 'X':
			neg, digits, ok := percentBaseDigits(v, c)
			if !ok {
				return nil, Raise(TypeError, "%%%c format: an integer is required, not %s", c, v.TypeName())
			}
			out = append(out, []byte(renderPercentInt(neg, digits, c, alt, prec, sign, width, left, zero))...)
		case 'e', 'E', 'f', 'F', 'g', 'G':
			f, ok := AsFloat(v)
			if !ok {
				return nil, Raise(TypeError, "float argument required, not %s", v.TypeName())
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
			out = append(out, []byte(padPercent(signStr(math.Signbit(f), sign), "", body, width, left, zero))...)
		default:
			disp := c
			if c <= 31 || c >= 128 {
				disp = '?'
			}
			return nil, Raise(ValueError, "unsupported format character '%c' (0x%x) at index %d", disp, c, cIdx)
		}
	}
	if st.idx < len(st.args) && st.mapping == nil {
		return nil, Raise(TypeError, "not all arguments converted during bytes formatting")
	}
	return out, nil
}

// bytesFormatArg pulls the bytes a %s or %b conversion contributes: a bytes-like
// object (bytes, bytearray, memoryview or array through the buffer protocol) or
// an object with __bytes__, matching CPython's format_obj. Anything else is the
// probed TypeError that names %b whichever of the two codes was written.
func bytesFormatArg(v Object) ([]byte, error) {
	if b, ok := AsBufferBytes(v); ok {
		return b, nil
	}
	if inst, ok := v.(*instanceObject); ok && classCallable(inst.cls, "__bytes__") {
		res, err := instanceCallMethod(inst, "__bytes__", nil)
		if err != nil {
			return nil, err
		}
		if bo, ok := res.(*bytesObject); ok {
			return bo.v, nil
		}
		return nil, Raise(TypeError, "__bytes__ returned non-bytes (type %s)", res.TypeName())
	}
	return nil, Raise(TypeError,
		"%%b requires a bytes-like object, or an object that implements __bytes__, not '%s'", v.TypeName())
}

// bytesByteConverter pulls the single byte a %c conversion contributes: a
// bytes or bytearray of length one, or an int in range(256), matching CPython's
// byte_converter down to the length and range wording.
func bytesByteConverter(v Object) (byte, error) {
	switch x := v.(type) {
	case *bytesObject:
		if len(x.v) != 1 {
			return 0, Raise(TypeError,
				"%%c requires an integer in range(256) or a single byte, not a bytes object of length %d", len(x.v))
		}
		return x.v[0], nil
	case *bytearrayObject:
		s := x.snapshot()
		if len(s) != 1 {
			return 0, Raise(TypeError,
				"%%c requires an integer in range(256) or a single byte, not a bytearray object of length %d", len(s))
		}
		return s[0], nil
	}
	if IsBigInt(v) {
		return 0, Raise(OverflowError, "%%c arg not in range(256)")
	}
	if n, ok := AsInt(v); ok {
		if n < 0 || n > 255 {
			return 0, Raise(OverflowError, "%%c arg not in range(256)")
		}
		return byte(n), nil
	}
	return 0, Raise(TypeError, "%%c requires an integer in range(256) or a single byte, not %s", v.TypeName())
}

// padPercentBytes pads sign+prefix+body to width in bytes, the byte-oriented
// twin of padPercent for a body that can carry arbitrary bytes (a %s payload or
// a %c byte). sign and prefix are always ASCII.
func padPercentBytes(sign, prefix string, body []byte, width int, left, zero bool) []byte {
	pad := width - len(sign) - len(prefix) - len(body)
	var out []byte
	out = append(out, sign...)
	if pad <= 0 {
		out = append(out, prefix...)
		out = append(out, body...)
		return out
	}
	switch {
	case left:
		out = append(out, prefix...)
		out = append(out, body...)
		out = append(out, bytes.Repeat([]byte{' '}, pad)...)
	case zero:
		out = append(out, prefix...)
		out = append(out, bytes.Repeat([]byte{'0'}, pad)...)
		out = append(out, body...)
	default:
		out = out[:0]
		out = append(out, bytes.Repeat([]byte{' '}, pad)...)
		out = append(out, sign...)
		out = append(out, prefix...)
		out = append(out, body...)
	}
	return out
}
