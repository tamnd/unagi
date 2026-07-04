package objects

// This file holds the splitting and joining methods shared by bytes and
// bytearray: split, rsplit, splitlines, join, partition, rpartition and
// translate. Every method that yields byte data returns the receiver's own
// type through byteResult, so a bytearray receiver gets bytearray pieces and a
// bytes receiver gets bytes.

// bytesSplitMethod handles the split-family names for both types, reporting
// handled=false when name is not one of them.
func bytesSplitMethod(v []byte, typeName, name string, args []Object) (Object, bool, error) {
	switch name {
	case "split", "rsplit":
		out, err := bytesSplit(v, typeName, name == "rsplit", args)
		return out, true, err
	case "splitlines":
		out, err := bytesSplitLines(v, typeName, args)
		return out, true, err
	case "join":
		out, err := bytesJoin(v, typeName, args)
		return out, true, err
	case "partition", "rpartition":
		out, err := bytesPartition(v, typeName, name == "rpartition", args)
		return out, true, err
	case "translate":
		out, err := bytesTranslate(v, typeName, args)
		return out, true, err
	}
	return nil, false, nil
}

// byteListResult boxes split pieces as a list of the receiver's own type.
func byteListResult(typeName string, parts [][]byte) Object {
	out := make([]Object, len(parts))
	for i, p := range parts {
		out[i] = byteResult(typeName, p)
	}
	return NewList(out)
}

// splitMaxArgs reads the optional (sep, maxsplit) arguments the split methods
// share. sep is None (whitespace mode) or a non-empty bytes-like separator;
// maxsplit is an integer defaulting to -1 (unlimited).
func splitMaxArgs(name string, args []Object) (sep []byte, whitespace bool, maxsplit int, err error) {
	if len(args) > 2 {
		return nil, false, 0, Raise(TypeError, "%s() takes at most 2 arguments (%d given)", name, len(args))
	}
	whitespace = true
	if len(args) >= 1 && args[0] != None {
		s, ok := asBytesLike(args[0])
		if !ok {
			return nil, false, 0, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		if len(s) == 0 {
			return nil, false, 0, Raise(ValueError, "empty separator")
		}
		sep = s
		whitespace = false
	}
	maxsplit = -1
	if len(args) == 2 {
		m, ok := AsInt(args[1])
		if !ok {
			return nil, false, 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		maxsplit = int(m)
	}
	return sep, whitespace, maxsplit, nil
}

// bytesSplit implements split (reverse=false) and rsplit (reverse=true).
func bytesSplit(v []byte, typeName string, reverse bool, args []Object) (Object, error) {
	name := "split"
	if reverse {
		name = "rsplit"
	}
	sep, whitespace, maxsplit, err := splitMaxArgs(name, args)
	if err != nil {
		return nil, err
	}
	var parts [][]byte
	switch {
	case whitespace && reverse:
		parts = splitWhitespaceRight(v, maxsplit)
	case whitespace:
		parts = splitWhitespaceLeft(v, maxsplit)
	case reverse:
		parts = splitSepRight(v, sep, maxsplit)
	default:
		parts = splitSepLeft(v, sep, maxsplit)
	}
	return byteListResult(typeName, parts), nil
}

// splitWhitespaceLeft splits on runs of ASCII whitespace from the left,
// dropping empty fields and honoring maxsplit; the final field keeps any inner
// and trailing whitespace once the split budget runs out.
func splitWhitespaceLeft(v []byte, maxsplit int) [][]byte {
	var parts [][]byte
	i, n, count := 0, len(v), 0
	for i < n {
		for i < n && asciiIsSpace(v[i]) {
			i++
		}
		if i >= n {
			break
		}
		if maxsplit >= 0 && count >= maxsplit {
			parts = append(parts, clone(v[i:n]))
			break
		}
		start := i
		for i < n && !asciiIsSpace(v[i]) {
			i++
		}
		parts = append(parts, clone(v[start:i]))
		count++
	}
	return parts
}

// splitWhitespaceRight is splitWhitespaceLeft scanning from the right, so
// maxsplit counts the rightmost separators.
func splitWhitespaceRight(v []byte, maxsplit int) [][]byte {
	var parts [][]byte
	j, count := len(v), 0
	for j > 0 {
		for j > 0 && asciiIsSpace(v[j-1]) {
			j--
		}
		if j <= 0 {
			break
		}
		if maxsplit >= 0 && count >= maxsplit {
			parts = append(parts, clone(v[:j]))
			break
		}
		end := j
		for j > 0 && !asciiIsSpace(v[j-1]) {
			j--
		}
		parts = append(parts, clone(v[j:end]))
		count++
	}
	reverseParts(parts)
	return parts
}

// splitSepLeft splits on exact occurrences of sep from the left, keeping empty
// fields and honoring maxsplit.
func splitSepLeft(v, sep []byte, maxsplit int) [][]byte {
	var parts [][]byte
	start, count := 0, 0
	for maxsplit < 0 || count < maxsplit {
		idx := byteFind(v, sep, start, len(v), false)
		if idx < 0 {
			break
		}
		parts = append(parts, clone(v[start:idx]))
		start = idx + len(sep)
		count++
	}
	parts = append(parts, clone(v[start:]))
	return parts
}

// splitSepRight splits on sep scanning from the right, so maxsplit counts the
// rightmost separators; the result is returned in left-to-right order.
func splitSepRight(v, sep []byte, maxsplit int) [][]byte {
	var parts [][]byte
	end, count := len(v), 0
	for maxsplit < 0 || count < maxsplit {
		idx := byteFind(v, sep, 0, end, true)
		if idx < 0 {
			break
		}
		parts = append(parts, clone(v[idx+len(sep):end]))
		end = idx
		count++
	}
	parts = append(parts, clone(v[:end]))
	reverseParts(parts)
	return parts
}

// bytesSplitLines splits on the byte line boundaries \n, \r and \r\n, the only
// terminators bytes recognizes (unlike str, which also splits the Unicode
// line-boundary set). A trailing terminator does not yield a final empty line.
func bytesSplitLines(v []byte, typeName string, args []Object) (Object, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError, "splitlines() takes at most 1 argument (%d given)", len(args))
	}
	keepends := false
	if len(args) == 1 {
		keepends = Truth(args[0])
	}
	var parts [][]byte
	i, n := 0, len(v)
	for i < n {
		start := i
		for i < n && v[i] != '\n' && v[i] != '\r' {
			i++
		}
		eol := i
		if i < n {
			if v[i] == '\r' && i+1 < n && v[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
		}
		if keepends {
			parts = append(parts, clone(v[start:i]))
		} else {
			parts = append(parts, clone(v[start:eol]))
		}
	}
	return byteListResult(typeName, parts), nil
}

// bytesJoin joins the bytes-like items of an iterable with the receiver as the
// separator, returning the receiver's type.
func bytesJoin(v []byte, typeName string, args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "join() takes exactly one argument (%d given)", len(args))
	}
	it, err := Iter(args[0])
	if err != nil {
		return nil, Raise(TypeError, "can only join an iterable")
	}
	var out []byte
	i := 0
	for {
		item, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		piece, ok := asBytesLike(item)
		if !ok {
			return nil, Raise(TypeError, "sequence item %d: expected a bytes-like object, %s found", i, item.TypeName())
		}
		if i > 0 {
			out = append(out, v...)
		}
		out = append(out, piece...)
		i++
	}
	return byteResult(typeName, out), nil
}

// bytesPartition implements partition (reverse=false) and rpartition
// (reverse=true), returning a 3-tuple of the receiver's own type: the part
// before the separator, the separator, and the part after. When the separator
// is absent, partition returns (whole, empty, empty) and rpartition returns
// (empty, empty, whole).
func bytesPartition(v []byte, typeName string, reverse bool, args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "%s takes exactly one argument (%d given)", partitionName(reverse), len(args))
	}
	sep, ok := asBytesLike(args[0])
	if !ok {
		return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
	}
	if len(sep) == 0 {
		return nil, Raise(ValueError, "empty separator")
	}
	idx := byteFind(v, sep, 0, len(v), reverse)
	if idx < 0 {
		if reverse {
			return partitionTuple(typeName, nil, nil, v), nil
		}
		return partitionTuple(typeName, v, nil, nil), nil
	}
	return partitionTuple(typeName, v[:idx], sep, v[idx+len(sep):]), nil
}

func partitionName(reverse bool) string {
	if reverse {
		return "rpartition"
	}
	return "partition"
}

// partitionTuple boxes the three partition pieces as a tuple of the receiver's
// type.
func partitionTuple(typeName string, before, sep, after []byte) Object {
	return NewTuple([]Object{
		byteResult(typeName, clone(before)),
		byteResult(typeName, clone(sep)),
		byteResult(typeName, clone(after)),
	})
}

// bytesTranslate maps each byte through a 256-byte table (or None for the
// identity map) after dropping every byte in the optional delete set.
func bytesTranslate(v []byte, typeName string, args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "translate() takes at least 1 argument (%d given)", len(args))
	}
	if len(args) > 2 {
		return nil, Raise(TypeError, "translate() takes at most 2 arguments (%d given)", len(args))
	}
	var table []byte
	if args[0] != None {
		t, ok := asBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		if len(t) != 256 {
			return nil, Raise(ValueError, "translation table must be 256 characters long")
		}
		table = t
	}
	var deleted [256]bool
	if len(args) == 2 {
		del, ok := asBytesLike(args[1])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[1].TypeName())
		}
		for _, c := range del {
			deleted[c] = true
		}
	}
	out := make([]byte, 0, len(v))
	for _, c := range v {
		if deleted[c] {
			continue
		}
		if table != nil {
			out = append(out, table[c])
		} else {
			out = append(out, c)
		}
	}
	return byteResult(typeName, out), nil
}

// clone returns a fresh copy so a returned piece never aliases the receiver's
// backing array.
func clone(b []byte) []byte {
	return append([]byte(nil), b...)
}

// reverseParts flips a slice of split pieces in place.
func reverseParts(parts [][]byte) {
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
}
