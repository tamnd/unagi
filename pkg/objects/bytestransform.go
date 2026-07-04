package objects

// This file holds the transform and predicate methods shared by bytes and
// bytearray: ASCII case mapping, whitespace and set stripping, replace,
// prefix/suffix removal, padding, and the is* predicates. A transform returns
// the receiver's own type (bytes stays bytes, bytearray stays bytearray),
// which byteResult selects from typeName; the predicates return a bool.

// byteResult boxes a transform's output as the receiver's type.
func byteResult(typeName string, b []byte) Object {
	if typeName == "bytearray" {
		return NewByteArray(b)
	}
	return NewBytes(b)
}

// bytesTransformMethod handles the transform and predicate names for both
// bytes and bytearray. It reports handled=false when name is not one of them
// so the caller can fall through to the "no attribute" error.
func bytesTransformMethod(v []byte, typeName, name string, args []Object) (Object, bool, error) {
	switch name {
	case "upper":
		return byteResult(typeName, bytesMapASCII(v, asciiUpper)), true, nil
	case "lower":
		return byteResult(typeName, bytesMapASCII(v, asciiLower)), true, nil
	case "swapcase":
		return byteResult(typeName, bytesMapASCII(v, asciiSwap)), true, nil
	case "capitalize":
		return byteResult(typeName, bytesCapitalize(v)), true, nil
	case "title":
		return byteResult(typeName, bytesTitle(v)), true, nil
	case "strip", "lstrip", "rstrip":
		out, err := bytesStrip(name, v, args)
		if err != nil {
			return nil, true, err
		}
		return byteResult(typeName, out), true, nil
	case "replace":
		out, err := bytesReplace(v, args)
		if err != nil {
			return nil, true, err
		}
		return byteResult(typeName, out), true, nil
	case "removeprefix", "removesuffix":
		out, err := bytesRemoveFix(name, v, args)
		if err != nil {
			return nil, true, err
		}
		return byteResult(typeName, out), true, nil
	case "center", "ljust", "rjust":
		out, err := bytesPad(name, v, args)
		if err != nil {
			return nil, true, err
		}
		return byteResult(typeName, out), true, nil
	case "zfill":
		out, err := bytesZfill(v, args)
		if err != nil {
			return nil, true, err
		}
		return byteResult(typeName, out), true, nil
	case "isascii", "isalpha", "isalnum", "isdigit", "isspace",
		"islower", "isupper", "istitle":
		if len(args) != 0 {
			return nil, true, Raise(TypeError, "%s.%s() takes no arguments (%d given)", typeName, name, len(args))
		}
		return NewBool(bytesPredicate(name, v)), true, nil
	}
	return nil, false, nil
}

func asciiUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

func asciiLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func asciiSwap(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func asciiIsUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func asciiIsLower(c byte) bool { return c >= 'a' && c <= 'z' }
func asciiIsAlpha(c byte) bool { return asciiIsUpper(c) || asciiIsLower(c) }
func asciiIsDigit(c byte) bool { return c >= '0' && c <= '9' }
func asciiIsAlnum(c byte) bool { return asciiIsAlpha(c) || asciiIsDigit(c) }

func asciiIsSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// bytesMapASCII applies a per-byte mapping, the shape upper/lower/swapcase share.
func bytesMapASCII(v []byte, f func(byte) byte) []byte {
	out := make([]byte, len(v))
	for i, c := range v {
		out[i] = f(c)
	}
	return out
}

// bytesCapitalize uppercases the first byte and lowercases the rest, ASCII only.
func bytesCapitalize(v []byte) []byte {
	out := make([]byte, len(v))
	for i, c := range v {
		if i == 0 {
			out[i] = asciiUpper(c)
		} else {
			out[i] = asciiLower(c)
		}
	}
	return out
}

// bytesTitle title-cases ASCII words: the first letter of each maximal run of
// letters is uppercased and the rest lowercased, with any non-letter byte
// ending a word.
func bytesTitle(v []byte) []byte {
	out := make([]byte, len(v))
	prevCased := false
	for i, c := range v {
		switch {
		case asciiIsLower(c):
			if !prevCased {
				out[i] = asciiUpper(c)
			} else {
				out[i] = c
			}
			prevCased = true
		case asciiIsUpper(c):
			if prevCased {
				out[i] = asciiLower(c)
			} else {
				out[i] = c
			}
			prevCased = true
		default:
			out[i] = c
			prevCased = false
		}
	}
	return out
}

// bytesPredicate answers the is* predicates. Every predicate but isascii is
// false on an empty receiver, matching CPython.
func bytesPredicate(name string, v []byte) bool {
	switch name {
	case "isascii":
		for _, c := range v {
			if c >= 0x80 {
				return false
			}
		}
		return true
	case "isalpha":
		return allBytes(v, asciiIsAlpha)
	case "isalnum":
		return allBytes(v, asciiIsAlnum)
	case "isdigit":
		return allBytes(v, asciiIsDigit)
	case "isspace":
		return allBytes(v, asciiIsSpace)
	case "islower":
		return casedPredicate(v, false)
	case "isupper":
		return casedPredicate(v, true)
	case "istitle":
		return bytesIsTitle(v)
	}
	return false
}

// allBytes reports whether v is non-empty and every byte satisfies f.
func allBytes(v []byte, f func(byte) bool) bool {
	if len(v) == 0 {
		return false
	}
	for _, c := range v {
		if !f(c) {
			return false
		}
	}
	return true
}

// casedPredicate backs islower (upper=false) and isupper (upper=true): the
// receiver must hold a cased byte and no byte of the opposite case.
func casedPredicate(v []byte, upper bool) bool {
	cased := false
	for _, c := range v {
		if upper {
			if asciiIsLower(c) {
				return false
			}
			if asciiIsUpper(c) {
				cased = true
			}
		} else {
			if asciiIsUpper(c) {
				return false
			}
			if asciiIsLower(c) {
				cased = true
			}
		}
	}
	return cased
}

// bytesIsTitle reports whether v is title-cased: at least one cased byte, with
// each uppercase byte starting a word and each lowercase byte continuing one.
func bytesIsTitle(v []byte) bool {
	cased := false
	prevCased := false
	for _, c := range v {
		switch {
		case asciiIsUpper(c):
			if prevCased {
				return false
			}
			prevCased = true
			cased = true
		case asciiIsLower(c):
			if !prevCased {
				return false
			}
			prevCased = true
			cased = true
		default:
			prevCased = false
		}
	}
	return cased
}

// bytesStrip trims bytes from one or both ends. With no argument (or None) it
// trims ASCII whitespace; a bytes-like argument gives the set of bytes to trim.
func bytesStrip(name string, v []byte, args []Object) ([]byte, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError, "%s takes at most 1 argument (%d given)", name, len(args))
	}
	inSet := asciiIsSpace
	if len(args) == 1 && args[0] != None {
		set, ok := asBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		inSet = func(c byte) bool { return bytesContains(set, c) }
	}
	lo, hi := 0, len(v)
	if name != "rstrip" {
		for lo < hi && inSet(v[lo]) {
			lo++
		}
	}
	if name != "lstrip" {
		for hi > lo && inSet(v[hi-1]) {
			hi--
		}
	}
	return append([]byte(nil), v[lo:hi]...), nil
}

// bytesReplace replaces occurrences of old with new, up to count times
// (count < 0 replaces all).
func bytesReplace(v []byte, args []Object) ([]byte, error) {
	if len(args) < 2 {
		return nil, Raise(TypeError, "replace() takes at least 2 arguments (%d given)", len(args))
	}
	if len(args) > 3 {
		return nil, Raise(TypeError, "replace() takes at most 3 arguments (%d given)", len(args))
	}
	old, ok := asBytesLike(args[0])
	if !ok {
		return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
	}
	repl, ok := asBytesLike(args[1])
	if !ok {
		return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[1].TypeName())
	}
	count := -1
	if len(args) == 3 {
		n, ok := AsInt(args[2])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[2].TypeName())
		}
		count = int(n)
	}
	return replaceBytes(v, old, repl, count), nil
}

// replaceBytes performs the byte-slice replacement, reproducing CPython's
// handling of an empty old (a match at every gap including the ends).
func replaceBytes(v, old, repl []byte, count int) []byte {
	if count == 0 {
		return append([]byte(nil), v...)
	}
	var out []byte
	done := 0
	i := 0
	for i <= len(v) {
		if (count < 0 || done < count) && bytesEqualAt(v, i, old) {
			out = append(out, repl...)
			done++
			if len(old) == 0 {
				if i < len(v) {
					out = append(out, v[i])
				}
				i++
			} else {
				i += len(old)
			}
			continue
		}
		if i < len(v) {
			out = append(out, v[i])
		}
		i++
	}
	return out
}

// bytesRemoveFix implements removeprefix and removesuffix.
func bytesRemoveFix(name string, v []byte, args []Object) ([]byte, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "%s() takes exactly one argument (%d given)", name, len(args))
	}
	fix, ok := asBytesLike(args[0])
	if !ok {
		return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
	}
	if name == "removeprefix" {
		if bytesEqualAt(v, 0, fix) {
			return append([]byte(nil), v[len(fix):]...), nil
		}
		return append([]byte(nil), v...), nil
	}
	if len(fix) <= len(v) && bytesEqualAt(v, len(v)-len(fix), fix) {
		return append([]byte(nil), v[:len(v)-len(fix)]...), nil
	}
	return append([]byte(nil), v...), nil
}

// bytesPad implements center, ljust and rjust: pad v to width with a single
// fill byte (default space).
func bytesPad(name string, v []byte, args []Object) ([]byte, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "%s() takes at least 1 argument (%d given)", name, len(args))
	}
	if len(args) > 2 {
		return nil, Raise(TypeError, "%s() takes at most 2 arguments (%d given)", name, len(args))
	}
	width, ok := AsInt(args[0])
	if !ok {
		return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	fill := byte(' ')
	if len(args) == 2 {
		f, err := padFill(name, args[1])
		if err != nil {
			return nil, err
		}
		fill = f
	}
	w := int(width)
	pad := w - len(v)
	if pad <= 0 {
		return append([]byte(nil), v...), nil
	}
	switch name {
	case "ljust":
		return append(append([]byte(nil), v...), fillRun(fill, pad)...), nil
	case "rjust":
		return append(fillRun(fill, pad), v...), nil
	default: // center
		// CPython gives the odd byte to the left only when width and the
		// margin are both odd; otherwise the extra pads the right.
		left := pad/2 + (pad & w & 1)
		out := fillRun(fill, left)
		out = append(out, v...)
		return append(out, fillRun(fill, pad-left)...), nil
	}
}

// padFill validates the optional fill byte of center/ljust/rjust. A bytes-like
// of the wrong length and a non-bytes value each get CPython's distinct
// wording.
func padFill(name string, o Object) (byte, error) {
	if fill, ok := asBytesLike(o); ok {
		if len(fill) != 1 {
			return 0, Raise(TypeError, "%s(): argument 2 must be a byte string of length 1, not a %s object of length %d", name, o.TypeName(), len(fill))
		}
		return fill[0], nil
	}
	return 0, Raise(TypeError, "%s() argument 2 must be a byte string of length 1, not %s", name, o.TypeName())
}

// bytesZfill pads v on the left with '0' to width, keeping a leading sign in
// front of the zeros.
func bytesZfill(v []byte, args []Object) ([]byte, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "zfill() takes exactly one argument (%d given)", len(args))
	}
	width, ok := AsInt(args[0])
	if !ok {
		return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	pad := int(width) - len(v)
	if pad <= 0 {
		return append([]byte(nil), v...), nil
	}
	out := make([]byte, 0, int(width))
	i := 0
	if len(v) > 0 && (v[0] == '+' || v[0] == '-') {
		out = append(out, v[0])
		i = 1
	}
	out = append(out, fillRun('0', pad)...)
	return append(out, v[i:]...), nil
}

// fillRun returns n copies of c.
func fillRun(c byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = c
	}
	return out
}
