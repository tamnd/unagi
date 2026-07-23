package objects

// This file holds the static and class methods read off a builtin type object
// rather than an instance: str.maketrans, bytes.maketrans / bytearray.maketrans
// and bytes.fromhex / bytearray.fromhex. base64 reaches bytes.maketrans at
// import to build its URL-safe and z85 translation tables, so the type object
// has to answer these names the way CPython does. builtinTypeClassmethod routes
// a read of one of these names on the constructor here.

// strMaketrans builds the translate table str.translate consumes. The one-dict
// form copies the mapping keyed by code point; the two- or three-string form
// pairs equal-length x and y into code-point-to-code-point entries and maps each
// character of an optional z to None for deletion. The messages, including the
// two CPython spells with a missing space, match 3.14.
func strMaketrans(args []Object) (Object, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, Raise(TypeError, "maketrans expected at most 3 arguments, got %d", len(args))
	}
	out := &dictObject{index: map[string]int{}}
	if len(args) == 1 {
		d, ok := args[0].(*dictObject)
		if !ok {
			return nil, Raise(TypeError, "if you give only one argument to maketrans it must be a dict")
		}
		for _, e := range d.entries {
			key, err := strMaketransKey(e.key)
			if err != nil {
				return nil, err
			}
			if err := out.set(NewInt(key), e.val); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	x, ok := AsStr(args[0])
	if !ok {
		return nil, Raise(TypeError, "first maketrans argument must be a string if there is a second argument")
	}
	y, ok := AsStr(args[1])
	if !ok {
		return nil, Raise(TypeError, "maketrans() argument 2 must be str, not %s", args[1].TypeName())
	}
	xr, yr := []rune(x), []rune(y)
	if len(xr) != len(yr) {
		return nil, Raise(ValueError, "the first two maketrans arguments must have equal length")
	}
	for i := range xr {
		if err := out.set(NewInt(int64(xr[i])), NewInt(int64(yr[i]))); err != nil {
			return nil, err
		}
	}
	if len(args) == 3 {
		z, ok := AsStr(args[2])
		if !ok {
			return nil, Raise(TypeError, "maketrans() argument 3 must be str, not %s", args[2].TypeName())
		}
		for _, ch := range z {
			if err := out.set(NewInt(int64(ch)), None); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// strMaketransKey resolves a one-argument maketrans dict key to its code point:
// a single-character string maps to its rune, an int stays itself, and anything
// else is the CPython type error (which carries its own missing-space typo).
func strMaketransKey(k Object) (int64, error) {
	if s, ok := k.(*strObject); ok {
		r := []rune(s.v)
		if len(r) != 1 {
			return 0, Raise(ValueError, "string keys in translatetable must be of length 1")
		}
		return int64(r[0]), nil
	}
	if n, ok := AsInt(k); ok {
		return n, nil
	}
	return 0, Raise(TypeError, "keys in translate table mustbe strings or integers")
}

// bytesMaketrans builds the 256-byte translation table bytes.translate consumes:
// the identity table with each byte of from remapped to the matching byte of to.
// Both arguments are bytes-like of equal length; the result is always bytes,
// which is what bytearray.maketrans returns too.
func bytesMaketrans(args []Object) (Object, error) {
	if len(args) != 2 {
		return nil, Raise(TypeError, "maketrans expected 2 arguments, got %d", len(args))
	}
	frm, ok := asBytesLike(args[0])
	if !ok {
		return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
	}
	to, ok := asBytesLike(args[1])
	if !ok {
		return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[1].TypeName())
	}
	if len(frm) != len(to) {
		return nil, Raise(ValueError, "maketrans arguments must have same length")
	}
	table := make([]byte, 256)
	for i := range table {
		table[i] = byte(i)
	}
	for i := range frm {
		table[frm[i]] = to[i]
	}
	return NewBytes(table), nil
}

// bytesFromhex parses a hex string into bytes (or bytearray for the bytearray
// type), a direct port of CPython's loop: ASCII whitespace is skipped only at a
// byte boundary, the two nibbles of a byte must be adjacent, an odd digit count
// is the "even number" error and a non-hex digit reports its position in the
// original string. The argument is a str or a bytes-like buffer.
func bytesFromhex(typeName string, args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "fromhex expected 1 argument, got %d", len(args))
	}
	var s []byte
	if str, ok := AsStr(args[0]); ok {
		s = []byte(str)
	} else if bl, ok := asBytesLike(args[0]); ok {
		s = bl
	} else {
		return nil, Raise(TypeError, "fromhex() argument must be str, not %s", args[0].TypeName())
	}
	out := make([]byte, 0, len(s)/2)
	i := 0
	for i < len(s) {
		if isHexSpace(s[i]) {
			i++
			continue
		}
		hi, ok := hexNibble(s[i])
		if !ok {
			return nil, Raise(ValueError, "non-hexadecimal number found in fromhex() arg at position %d", i)
		}
		if i+1 >= len(s) {
			return nil, Raise(ValueError, "fromhex() arg must contain an even number of hexadecimal digits")
		}
		lo, ok := hexNibble(s[i+1])
		if !ok {
			return nil, Raise(ValueError, "non-hexadecimal number found in fromhex() arg at position %d", i+1)
		}
		out = append(out, hi<<4|lo)
		i += 2
	}
	return byteResult(typeName, out), nil
}

// bytesTranslateKw handles the one bytes/bytearray method that takes a keyword,
// translate(table, /, delete=b”), by folding the delete keyword into a second
// positional argument and dispatching through the positional path. Any other
// bytes method, or any other keyword, is the take-no-keyword TypeError CPython
// gives. base64.b16decode reaches this with translate(None, delete=b'0123...').
func bytesTranslateKw(o Object, typeName, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name != "translate" {
		return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", typeName, name)
	}
	args := append([]Object(nil), pos...)
	for i, kn := range kwNames {
		if kn != "delete" {
			return nil, Raise(TypeError, "translate() got an unexpected keyword argument '%s'", kn)
		}
		args = append(args, kwVals[i])
	}
	return CallMethod(o, name, args)
}

// isHexSpace reports the ASCII whitespace fromhex skips between bytes, matching
// CPython's Py_ISSPACE over the C locale.
func isHexSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// hexNibble decodes one hex digit to its 0-15 value.
func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
