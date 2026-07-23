package objects

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func wantStrArg(method string, pos int, o Object) (string, error) {
	s, ok := AsStr(o)
	if !ok {
		return "", Raise(TypeError, "%s() argument %d must be str, not %s", method, pos, o.TypeName())
	}
	return s, nil
}

// strMethod dispatches the no-kwargs str method surface. Every result
// and error text is probed on CPython 3.14; the helpers below the
// switch carry the mechanics.
func strMethod(x *strObject, name string, args []Object) (Object, error) {
	s := x.v
	switch name {
	case "upper":
		if err := strNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewStr(strings.ToUpper(s)), nil
	case "lower":
		if err := strNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewStr(strings.ToLower(s)), nil
	case "capitalize":
		if err := strNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewStr(strCapitalize(s)), nil
	case "title":
		if err := strNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewStr(strTitle(s)), nil
	case "swapcase":
		if err := strNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewStr(strSwapcase(s)), nil
	case "isalnum", "isalpha", "isascii", "isdecimal", "isdigit", "isidentifier",
		"islower", "isnumeric", "isprintable", "isspace", "istitle", "isupper":
		if err := strNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewBool(strPredicate(name, s)), nil
	case "strip", "lstrip", "rstrip":
		if len(args) > 1 {
			return nil, Raise(TypeError, "%s expected at most 1 argument, got %d", name, len(args))
		}
		cut := ""
		haveCut := false
		if len(args) == 1 && args[0] != None {
			c, ok := AsStr(args[0])
			if !ok {
				// Probed on 3.14: " hi ".strip(1). No type name in the text.
				return nil, Raise(TypeError, "%s arg must be None or str", name)
			}
			cut, haveCut = c, true
		}
		switch name {
		case "strip":
			if haveCut {
				return NewStr(strings.Trim(s, cut)), nil
			}
			return NewStr(strings.TrimFunc(s, strIsSpace)), nil
		case "lstrip":
			if haveCut {
				return NewStr(strings.TrimLeft(s, cut)), nil
			}
			return NewStr(strings.TrimLeftFunc(s, strIsSpace)), nil
		default:
			if haveCut {
				return NewStr(strings.TrimRight(s, cut)), nil
			}
			return NewStr(strings.TrimRightFunc(s, strIsSpace)), nil
		}
	case "split", "rsplit":
		if len(args) > 2 {
			// Probed on 3.14: "a,b".split(",", 1, 2). Note the shape differs
			// from the find family.
			return nil, Raise(TypeError, "%s() takes at most 2 arguments (%d given)", name, len(args))
		}
		maxsplit := int64(-1)
		if len(args) == 2 {
			m, err := strIntArg(args[1])
			if err != nil {
				return nil, err
			}
			maxsplit = m
		}
		if maxsplit < 0 || maxsplit > int64(len(s)) {
			maxsplit = -1
		}
		if len(args) == 0 || args[0] == None {
			if name == "split" {
				return strSplitWhitespace(s, int(maxsplit)), nil
			}
			return strRsplitWhitespace(s, int(maxsplit)), nil
		}
		sep, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "must be str or None, not %s", args[0].TypeName())
		}
		if sep == "" {
			return nil, Raise(ValueError, "empty separator")
		}
		if name == "split" {
			var parts []string
			if maxsplit < 0 {
				parts = strings.Split(s, sep)
			} else {
				parts = strings.SplitN(s, sep, int(maxsplit)+1)
			}
			return strList(parts), nil
		}
		return strList(strRsplitSep(s, sep, int(maxsplit))), nil
	case "splitlines":
		if len(args) > 1 {
			return nil, Raise(TypeError, "splitlines() takes at most 1 argument (%d given)", len(args))
		}
		keepends := false
		if len(args) == 1 {
			t, err := TruthOf(args[0])
			if err != nil {
				return nil, err
			}
			keepends = t
		}
		return strSplitLines(s, keepends), nil
	case "join":
		if len(args) != 1 {
			return nil, Raise(TypeError, "str.join() takes exactly one argument (%d given)", len(args))
		}
		it, err := Iter(args[0])
		if err != nil {
			return nil, Raise(TypeError, "can only join an iterable")
		}
		var b strings.Builder
		i := 0
		for {
			v, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			part, isStr := AsStr(v)
			if !isStr {
				return nil, Raise(TypeError, "sequence item %d: expected str instance, %s found",
					i, v.TypeName())
			}
			if i > 0 {
				b.WriteString(s)
			}
			b.WriteString(part)
			i++
		}
		return NewStr(b.String()), nil
	case "startswith", "endswith":
		return strStartsEndsWith(name, s, args)
	case "replace":
		if len(args) < 2 {
			return nil, Raise(TypeError, "replace() takes at least 2 positional arguments (%d given)", len(args))
		}
		if len(args) > 3 {
			return nil, Raise(TypeError, "replace() takes at most 3 arguments (%d given)", len(args))
		}
		old, err := wantStrArg("replace", 1, args[0])
		if err != nil {
			return nil, err
		}
		repl, err := wantStrArg("replace", 2, args[1])
		if err != nil {
			return nil, err
		}
		count := int64(-1)
		if len(args) == 3 {
			count, err = strIntArg(args[2])
			if err != nil {
				return nil, err
			}
		}
		if count < 0 {
			return NewStr(strings.ReplaceAll(s, old, repl)), nil
		}
		if count > int64(len(s))+1 {
			count = int64(len(s)) + 1
		}
		return NewStr(strings.Replace(s, old, repl, int(count))), nil
	case "find", "rfind", "index", "rindex":
		pos, err := strFindCommon(name, s, args)
		if err != nil {
			return nil, err
		}
		if pos < 0 && (name == "index" || name == "rindex") {
			return nil, Raise(ValueError, "substring not found")
		}
		return NewInt(int64(pos)), nil
	case "count":
		sub, start, end, err := strSubRangeArgs(name, s, args)
		if err != nil {
			return nil, err
		}
		return NewInt(int64(strRuneCount(sub.hay, sub.needle, start, end))), nil
	case "center", "ljust", "rjust":
		if len(args) == 0 {
			return nil, Raise(TypeError, "%s expected at least 1 argument, got 0", name)
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "%s expected at most 2 arguments, got %d", name, len(args))
		}
		width, err := strIntArg(args[0])
		if err != nil {
			return nil, err
		}
		fill := ' '
		if len(args) == 2 {
			fill, err = strFillChar(args[1])
			if err != nil {
				return nil, err
			}
		}
		return NewStr(strJustify(name, s, width, fill)), nil
	case "zfill":
		if len(args) != 1 {
			return nil, Raise(TypeError, "str.zfill() takes exactly one argument (%d given)", len(args))
		}
		width, err := strIntArg(args[0])
		if err != nil {
			return nil, err
		}
		return NewStr(strZfill(s, width)), nil
	case "partition", "rpartition":
		if len(args) != 1 {
			return nil, Raise(TypeError, "str.%s() takes exactly one argument (%d given)", name, len(args))
		}
		sep, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "must be str, not %s", args[0].TypeName())
		}
		if sep == "" {
			return nil, Raise(ValueError, "empty separator")
		}
		var head, mid, tail string
		if name == "partition" {
			if k := strings.Index(s, sep); k >= 0 {
				head, mid, tail = s[:k], sep, s[k+len(sep):]
			} else {
				head = s
			}
		} else {
			if k := strings.LastIndex(s, sep); k >= 0 {
				head, mid, tail = s[:k], sep, s[k+len(sep):]
			} else {
				tail = s
			}
		}
		return NewTuple([]Object{NewStr(head), NewStr(mid), NewStr(tail)}), nil
	case "removeprefix", "removesuffix":
		if len(args) != 1 {
			return nil, Raise(TypeError, "str.%s() takes exactly one argument (%d given)", name, len(args))
		}
		fix, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "%s() argument must be str, not %s", name, args[0].TypeName())
		}
		if name == "removeprefix" {
			return NewStr(strings.TrimPrefix(s, fix)), nil
		}
		return NewStr(strings.TrimSuffix(s, fix)), nil
	case "expandtabs":
		if len(args) > 1 {
			return nil, Raise(TypeError, "expandtabs() takes at most 1 argument (%d given)", len(args))
		}
		tabsize := int64(8)
		if len(args) == 1 {
			t, err := strIntArg(args[0])
			if err != nil {
				return nil, err
			}
			tabsize = t
		}
		return NewStr(strExpandTabs(s, tabsize)), nil
	case "format":
		return strFormat(s, args)
	case "translate":
		return strTranslate(s, args)
	case "encode":
		return strEncode(s, args)
	}
	return nil, noAttr(x, name)
}

// strEncode encodes a str to bytes under the named codec, the inverse of
// bytes.decode. It shares the encodeStr codec switch the two-argument bytes
// constructor uses, so the utf-8, ascii and latin-1 families and their error
// wording stay in one place. encoding defaults to utf-8 and errors defaults to
// strict. The handler is looked up lazily, only when a character cannot be
// encoded, so an all-encodable string under an unknown handler does not raise,
// matching CPython; os.fsencode reaches this with utf-8 and surrogateescape.
func strEncode(s string, args []Object) (Object, error) {
	enc := "utf-8"
	if len(args) >= 1 && args[0] != None {
		e, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "encode() argument 'encoding' must be str, not %s", args[0].TypeName())
		}
		enc = e
	}
	errh := "strict"
	if len(args) >= 2 && args[1] != None {
		eh, ok := AsStr(args[1])
		if !ok {
			return nil, Raise(TypeError, "encode() argument 'errors' must be str, not %s", args[1].TypeName())
		}
		errh = eh
	}
	b, err := encodeStr(s, enc, errh)
	if err != nil {
		return nil, err
	}
	return NewBytes(b), nil
}

// strTranslate maps each character through a table keyed by code point, the way
// re.escape shifts the special characters to their backslash-escaped forms. A
// key the table does not carry (KeyError) keeps the original character, a None
// result deletes it, an int result becomes that code point, and a str result is
// spliced in.
func strTranslate(s string, args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "translate() takes exactly one argument (%d given)", len(args))
	}
	table := args[0]
	var b strings.Builder
	for _, ch := range s {
		v, err := GetItem(table, NewInt(int64(ch)))
		if err != nil {
			if e, ok := err.(*Exception); ok && e.Kind == KeyError {
				b.WriteRune(ch)
				continue
			}
			return nil, err
		}
		if v == None {
			continue
		}
		if code, ok := AsInt(v); ok {
			if code < 0 || code > 0x10FFFF {
				return nil, Raise(ValueError, "character mapping must be in range(0x110000)")
			}
			b.WriteRune(rune(code))
			continue
		}
		if rep, ok := AsStr(v); ok {
			b.WriteString(rep)
			continue
		}
		return nil, Raise(TypeError, "character mapping must return integer, None or str")
	}
	return NewStr(b.String()), nil
}

// strNoArgs is the zero-arity check shared by the case and predicate
// methods. Probed on 3.14: "a".upper(1) and "a".isalpha(1).
func strNoArgs(name string, args []Object) error {
	if len(args) != 0 {
		return Raise(TypeError, "str.%s() takes no arguments (%d given)", name, len(args))
	}
	return nil
}

// strIntArg converts an int-like argument the way CPython's index
// converter does. Probed on 3.14: "a,b".split(",", "x") and
// "12".zfill("a") both say the object cannot be interpreted as an
// integer, and True is accepted as 1.
func strIntArg(o Object) (int64, error) {
	v, ok := AsInt(o)
	if !ok {
		// Probed: "12".zfill(2**100) overflows the ssize_t converter.
		if IsBigInt(o) {
			return 0, Raise(OverflowError, "Python int too large to convert to C ssize_t")
		}
		return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
	}
	return v, nil
}

// strSliceArg reads an optional start or end argument. None keeps the
// default. Probed on 3.14: "hello".find("a", "b") and find("l", 1.5)
// both raise the slice-indices TypeError.
func strSliceArg(args []Object, i int, def int64) (int64, error) {
	if i >= len(args) || args[i] == None {
		return def, nil
	}
	v, ok := AsInt(args[i])
	if !ok {
		// Probed: "abc".find("b", -2**100, 2**100) clamps like a slice.
		if b, isBig := args[i].(*intObject); isBig && b.big != nil {
			if b.big.Sign() > 0 {
				return 1 << 62, nil
			}
			return -(1 << 62), nil
		}
		return 0, Raise(TypeError, "slice indices must be integers or None or have an __index__ method")
	}
	return v, nil
}

// strAdjustIndices mirrors CPython's ADJUST_INDICES: negatives count
// from the end and clamp at 0, the end clamps at n, and a start past n
// is capped just above it so every caller sees an empty range.
func strAdjustIndices(start, end int64, n int) (int, int) {
	if end > int64(n) {
		end = int64(n)
	} else if end < 0 {
		end += int64(n)
		if end < 0 {
			end = 0
		}
	}
	if start < 0 {
		start += int64(n)
		if start < 0 {
			start = 0
		}
	}
	if start > int64(n) {
		start = int64(n) + 1
	}
	return int(start), int(end)
}

// strSub bundles the rune views a substring search works on. All str
// indices are code points, never bytes.
type strSub struct {
	hay    []rune
	needle []rune
}

// strSubRangeArgs parses the (sub, start, end) positional shape used by
// find, rfind, index, rindex and count, returning adjusted bounds.
func strSubRangeArgs(name, s string, args []Object) (strSub, int, int, error) {
	if len(args) == 0 {
		return strSub{}, 0, 0, Raise(TypeError, "%s expected at least 1 argument, got 0", name)
	}
	if len(args) > 3 {
		return strSub{}, 0, 0, Raise(TypeError, "%s expected at most 3 arguments, got %d", name, len(args))
	}
	sub, err := wantStrArg(name, 1, args[0])
	if err != nil {
		return strSub{}, 0, 0, err
	}
	hay := []rune(s)
	start, err := strSliceArg(args, 1, 0)
	if err != nil {
		return strSub{}, 0, 0, err
	}
	end, err := strSliceArg(args, 2, int64(len(hay)))
	if err != nil {
		return strSub{}, 0, 0, err
	}
	lo, hi := strAdjustIndices(start, end, len(hay))
	return strSub{hay: hay, needle: []rune(sub)}, lo, hi, nil
}

func strFindCommon(name, s string, args []Object) (int, error) {
	sub, start, end, err := strSubRangeArgs(name, s, args)
	if err != nil {
		return 0, err
	}
	if name == "rfind" || name == "rindex" {
		return strRuneRfind(sub.hay, sub.needle, start, end), nil
	}
	return strRuneFind(sub.hay, sub.needle, start, end), nil
}

func strRuneMatch(hay, needle []rune, at int) bool {
	for i, r := range needle {
		if hay[at+i] != r {
			return false
		}
	}
	return true
}

// strRuneFind returns the leftmost match in [start, end), or -1.
// Probed on 3.14: "hello".find("", 4) is 4 and find("", 10) is -1, so
// an empty needle matches at start whenever start <= end.
func strRuneFind(hay, needle []rune, start, end int) int {
	m := len(needle)
	if m == 0 {
		if start <= end {
			return start
		}
		return -1
	}
	for j := start; j+m <= end; j++ {
		if strRuneMatch(hay, needle, j) {
			return j
		}
	}
	return -1
}

// strRuneRfind is the rightmost variant. Probed on 3.14:
// "hello".rfind("", 5) is 5 and rfind("", 6) is -1.
func strRuneRfind(hay, needle []rune, start, end int) int {
	m := len(needle)
	if m == 0 {
		if start <= end {
			return end
		}
		return -1
	}
	for j := end - m; j >= start; j-- {
		if strRuneMatch(hay, needle, j) {
			return j
		}
	}
	return -1
}

// strRuneCount counts non-overlapping matches. Probed on 3.14:
// "abc".count("") is 4, "abc".count("", 1, 2) is 2, "abc".count("", 5)
// is 0.
func strRuneCount(hay, needle []rune, start, end int) int {
	m := len(needle)
	if m == 0 {
		if start <= end {
			return end - start + 1
		}
		return 0
	}
	n := 0
	for j := start; j+m <= end; {
		if strRuneMatch(hay, needle, j) {
			n++
			j += m
		} else {
			j++
		}
	}
	return n
}

// strStartsEndsWith implements startswith and endswith with the tuple
// form and slice-style start and end. The bounds logic mirrors
// CPython's tailmatch. Probed on 3.14: "abc".startswith("", 5) is
// False and "hello".endswith("ll", 0, -1) is True.
func strStartsEndsWith(name, s string, args []Object) (Object, error) {
	if len(args) == 0 {
		return nil, Raise(TypeError, "%s expected at least 1 argument, got 0", name)
	}
	if len(args) > 3 {
		return nil, Raise(TypeError, "%s expected at most 3 arguments, got %d", name, len(args))
	}
	hay := []rune(s)
	start, err := strSliceArg(args, 1, 0)
	if err != nil {
		return nil, err
	}
	end, err := strSliceArg(args, 2, int64(len(hay)))
	if err != nil {
		return nil, err
	}
	lo, hi := strAdjustIndices(start, end, len(hay))
	if t, ok := args[0].(*tupleObject); ok {
		// Elements are checked lazily, so a match short-circuits ahead
		// of a bad later element. Probed on 3.14: "a".endswith(("a", 1))
		// is True, "a".endswith(("x", 1)) raises.
		for _, e := range t.elts {
			p, isStr := AsStr(e)
			if !isStr {
				return nil, Raise(TypeError, "tuple for %s must only contain str, not %s", name, e.TypeName())
			}
			if strTailMatch(hay, []rune(p), lo, hi, name == "endswith") {
				return True, nil
			}
		}
		return False, nil
	}
	p, ok := AsStr(args[0])
	if !ok {
		return nil, Raise(TypeError, "%s first arg must be str or a tuple of str, not %s",
			name, args[0].TypeName())
	}
	return NewBool(strTailMatch(hay, []rune(p), lo, hi, name == "endswith")), nil
}

func strTailMatch(hay, needle []rune, start, end int, atEnd bool) bool {
	end -= len(needle)
	if end < start {
		return false
	}
	pos := start
	if atEnd {
		pos = end
	}
	return strRuneMatch(hay, needle, pos)
}

// strIsSpace matches CPython's Py_UNICODE_ISSPACE, which takes in the
// file/group/record separators 1c..1f that Unicode White_Space lacks.
// Probed on 3.14: "\x1c".isspace() is True and "a\x1cb".split() splits
// on it.
func strIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func strList(parts []string) Object {
	out := make([]Object, len(parts))
	for i, p := range parts {
		out[i] = NewStr(p)
	}
	return NewList(out)
}

// strSplitWhitespace is split(None, maxsplit): runs of whitespace
// separate words, leading whitespace is dropped, and once the split
// budget runs out the rest, trailing whitespace included, is one
// piece. Probed on 3.14: " a b c ".split(None, 1) == ['a', 'b c '].
func strSplitWhitespace(s string, maxsplit int) Object {
	rs := []rune(s)
	var parts []string
	i, n := 0, len(rs)
	for {
		for i < n && strIsSpace(rs[i]) {
			i++
		}
		if i >= n {
			break
		}
		if maxsplit == 0 {
			parts = append(parts, string(rs[i:]))
			break
		}
		j := i
		for j < n && !strIsSpace(rs[j]) {
			j++
		}
		parts = append(parts, string(rs[i:j]))
		i = j
		if maxsplit > 0 {
			maxsplit--
		}
	}
	return strList(parts)
}

// strRsplitWhitespace mirrors strSplitWhitespace from the right.
// Probed on 3.14: " a b ".rsplit(None, 0) == [' a b'].
func strRsplitWhitespace(s string, maxsplit int) Object {
	rs := []rune(s)
	var parts []string
	i := len(rs) - 1
	for {
		for i >= 0 && strIsSpace(rs[i]) {
			i--
		}
		if i < 0 {
			break
		}
		if maxsplit == 0 {
			parts = append(parts, string(rs[:i+1]))
			break
		}
		j := i
		for j >= 0 && !strIsSpace(rs[j]) {
			j--
		}
		parts = append(parts, string(rs[j+1:i+1]))
		i = j
		if maxsplit > 0 {
			maxsplit--
		}
	}
	for l, r := 0, len(parts)-1; l < r; l, r = l+1, r-1 {
		parts[l], parts[r] = parts[r], parts[l]
	}
	return strList(parts)
}

// strRsplitSep cuts at most maxsplit separators counting from the
// right. Probed on 3.14: "a,b,c".rsplit(",", 1) == ['a,b', 'c'].
func strRsplitSep(s, sep string, maxsplit int) []string {
	if maxsplit < 0 {
		return strings.Split(s, sep)
	}
	var tail []string
	rest := s
	for maxsplit > 0 {
		k := strings.LastIndex(rest, sep)
		if k < 0 {
			break
		}
		tail = append(tail, rest[k+len(sep):])
		rest = rest[:k]
		maxsplit--
	}
	parts := make([]string, 0, len(tail)+1)
	parts = append(parts, rest)
	for i := len(tail) - 1; i >= 0; i-- {
		parts = append(parts, tail[i])
	}
	return parts
}

// strLineBreak is the splitlines boundary set, probed on 3.14 with
// "a\vb\fc\x1cd\x1de\x1ef\x85g h i".splitlines().
func strLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

func strSplitLines(s string, keepends bool) Object {
	rs := []rune(s)
	var parts []string
	start, i, n := 0, 0, len(rs)
	for i < n {
		if !strLineBreak(rs[i]) {
			i++
			continue
		}
		eol := i + 1
		if rs[i] == '\r' && eol < n && rs[eol] == '\n' {
			eol++
		}
		if keepends {
			parts = append(parts, string(rs[start:eol]))
		} else {
			parts = append(parts, string(rs[start:i]))
		}
		i, start = eol, eol
	}
	if start < n {
		parts = append(parts, string(rs[start:]))
	}
	return strList(parts)
}

// strFillChar validates the center/ljust/rjust fill argument. Probed
// on 3.14: "abc".center(6, "ab") and "abc".center(6, 1).
func strFillChar(o Object) (rune, error) {
	f, ok := AsStr(o)
	if !ok {
		return 0, Raise(TypeError, "The fill character must be a unicode character, not %s", o.TypeName())
	}
	rs := []rune(f)
	if len(rs) != 1 {
		return 0, Raise(TypeError, "The fill character must be exactly one character long")
	}
	return rs[0], nil
}

// strJustify pads to width code points. The center split comes from
// CPython: left margin is marg/2 plus (marg & width & 1), probed on
// 3.14 with "ab".center(5) == '  ab '.
func strJustify(name, s string, width int64, fill rune) string {
	n := len([]rune(s))
	if width <= int64(n) {
		return s
	}
	marg := int(width) - n
	switch name {
	case "ljust":
		return s + strings.Repeat(string(fill), marg)
	case "rjust":
		return strings.Repeat(string(fill), marg) + s
	}
	left := marg/2 + (marg & int(width) & 1)
	return strings.Repeat(string(fill), left) + s + strings.Repeat(string(fill), marg-left)
}

// strZfill zero-pads to width, keeping a leading ASCII sign in front.
// Probed on 3.14: "+12".zfill(5) == '+0012' and the Unicode minus in
// "−12" does not count as a sign.
func strZfill(s string, width int64) string {
	rs := []rune(s)
	if width <= int64(len(rs)) {
		return s
	}
	pad := strings.Repeat("0", int(width)-len(rs))
	if len(rs) > 0 && (rs[0] == '+' || rs[0] == '-') {
		return string(rs[0]) + pad + string(rs[1:])
	}
	return pad + s
}

// strExpandTabs advances a column counter per code point, resetting on
// \n and \r. Probed on 3.14: "ab\rcd\tx".expandtabs(4) == 'ab\rcd  x'
// and a non-positive tab size deletes tabs.
func strExpandTabs(s string, tabsize int64) string {
	var b strings.Builder
	col := int64(0)
	for _, r := range s {
		switch r {
		case '\t':
			if tabsize > 0 {
				pad := tabsize - col%tabsize
				b.WriteString(strings.Repeat(" ", int(pad)))
				col += pad
			}
		case '\n', '\r':
			b.WriteRune(r)
			col = 0
		default:
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// The Lowercase and Uppercase derived properties CPython classifies
// with, not just the Ll and Lu categories.
func strLowerRune(r rune) bool {
	return unicode.IsLower(r) || unicode.Is(unicode.Other_Lowercase, r)
}

func strUpperRune(r rune) bool {
	return unicode.IsUpper(r) || unicode.Is(unicode.Other_Uppercase, r)
}

func strCasedRune(r rune) bool {
	return strLowerRune(r) || strUpperRune(r) || unicode.IsTitle(r)
}

// strCapitalize titlecases the first code point and lowercases the
// rest, the 3.14 behavior. Probed: "HELLO World".capitalize() ==
// 'Hello world' and "ǆab".capitalize() == 'ǅab'. Multi-char special
// casings (the German sharp s expanding to Ss) are not reproduced;
// see the divergence notes in this package's tests.
func strCapitalize(s string) string {
	rs := []rune(s)
	if len(rs) == 0 {
		return s
	}
	var b strings.Builder
	b.WriteRune(unicode.ToTitle(rs[0]))
	for _, r := range rs[1:] {
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// strTitle titlecases the first cased character after every uncased
// one and lowercases the rest, exactly CPython's loop. Probed on 3.14:
// "it's a test".title() == "It'S A Test" and "3g ab".title() ==
// '3G Ab'.
func strTitle(s string) string {
	var b strings.Builder
	prevCased := false
	for _, r := range s {
		if prevCased {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(unicode.ToTitle(r))
		}
		prevCased = strCasedRune(r)
	}
	return b.String()
}

// strSwapcase lowercases uppercase characters and vice versa, leaving
// titlecase ones alone. Probed on 3.14: "ǅ".swapcase() == 'ǅ' and
// "µ".swapcase() == 'Μ'.
func strSwapcase(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case strUpperRune(r):
			b.WriteRune(unicode.ToLower(r))
		case strLowerRune(r):
			b.WriteRune(unicode.ToUpper(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// strDigitOnly holds the code points where isdigit() is true but
// isdecimal() is false, meaning Numeric_Type=Digit outside category
// Nd. Generated by probing python3.14 (Unicode 16.0) over the full
// code point range: superscripts, subscripts, circled digits and a
// few script-specific digit sets.
var strDigitOnly = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x00b2, Hi: 0x00b3, Stride: 1},
		{Lo: 0x00b9, Hi: 0x00b9, Stride: 1},
		{Lo: 0x1369, Hi: 0x1371, Stride: 1},
		{Lo: 0x19da, Hi: 0x19da, Stride: 1},
		{Lo: 0x2070, Hi: 0x2070, Stride: 1},
		{Lo: 0x2074, Hi: 0x2079, Stride: 1},
		{Lo: 0x2080, Hi: 0x2089, Stride: 1},
		{Lo: 0x2460, Hi: 0x2468, Stride: 1},
		{Lo: 0x2474, Hi: 0x247c, Stride: 1},
		{Lo: 0x2488, Hi: 0x2490, Stride: 1},
		{Lo: 0x24ea, Hi: 0x24ea, Stride: 1},
		{Lo: 0x24f5, Hi: 0x24fd, Stride: 1},
		{Lo: 0x24ff, Hi: 0x24ff, Stride: 1},
		{Lo: 0x2776, Hi: 0x277e, Stride: 1},
		{Lo: 0x2780, Hi: 0x2788, Stride: 1},
		{Lo: 0x278a, Hi: 0x2792, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x10a40, Hi: 0x10a43, Stride: 1},
		{Lo: 0x10e60, Hi: 0x10e68, Stride: 1},
		{Lo: 0x11052, Hi: 0x1105a, Stride: 1},
		{Lo: 0x1f100, Hi: 0x1f10a, Stride: 1},
	},
	LatinOffset: 2,
}

// strNumericIdeographs holds the code points where isnumeric() is true
// but the Unicode category is not N: the CJK ideographs that carry a
// numeric value. Generated by probing python3.14 (Unicode 16.0) over
// the full code point range.
var strNumericIdeographs = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x3405, Hi: 0x3405, Stride: 1},
		{Lo: 0x3483, Hi: 0x3483, Stride: 1},
		{Lo: 0x382a, Hi: 0x382a, Stride: 1},
		{Lo: 0x3b4d, Hi: 0x3b4d, Stride: 1},
		{Lo: 0x4e00, Hi: 0x4e00, Stride: 1},
		{Lo: 0x4e03, Hi: 0x4e03, Stride: 1},
		{Lo: 0x4e07, Hi: 0x4e07, Stride: 1},
		{Lo: 0x4e09, Hi: 0x4e09, Stride: 1},
		{Lo: 0x4e24, Hi: 0x4e24, Stride: 1},
		{Lo: 0x4e5d, Hi: 0x4e5d, Stride: 1},
		{Lo: 0x4e8c, Hi: 0x4e8c, Stride: 1},
		{Lo: 0x4e94, Hi: 0x4e94, Stride: 1},
		{Lo: 0x4e96, Hi: 0x4e96, Stride: 1},
		{Lo: 0x4eac, Hi: 0x4eac, Stride: 1},
		{Lo: 0x4ebf, Hi: 0x4ec0, Stride: 1},
		{Lo: 0x4edf, Hi: 0x4edf, Stride: 1},
		{Lo: 0x4ee8, Hi: 0x4ee8, Stride: 1},
		{Lo: 0x4f0d, Hi: 0x4f0d, Stride: 1},
		{Lo: 0x4f70, Hi: 0x4f70, Stride: 1},
		{Lo: 0x4fe9, Hi: 0x4fe9, Stride: 1},
		{Lo: 0x5006, Hi: 0x5006, Stride: 1},
		{Lo: 0x5104, Hi: 0x5104, Stride: 1},
		{Lo: 0x5146, Hi: 0x5146, Stride: 1},
		{Lo: 0x5169, Hi: 0x5169, Stride: 1},
		{Lo: 0x516b, Hi: 0x516b, Stride: 1},
		{Lo: 0x516d, Hi: 0x516d, Stride: 1},
		{Lo: 0x5341, Hi: 0x5341, Stride: 1},
		{Lo: 0x5343, Hi: 0x5345, Stride: 1},
		{Lo: 0x534c, Hi: 0x534c, Stride: 1},
		{Lo: 0x53c1, Hi: 0x53c4, Stride: 1},
		{Lo: 0x56db, Hi: 0x56db, Stride: 1},
		{Lo: 0x58f1, Hi: 0x58f1, Stride: 1},
		{Lo: 0x58f9, Hi: 0x58f9, Stride: 1},
		{Lo: 0x5e7a, Hi: 0x5e7a, Stride: 1},
		{Lo: 0x5efe, Hi: 0x5eff, Stride: 1},
		{Lo: 0x5f0c, Hi: 0x5f0e, Stride: 1},
		{Lo: 0x5f10, Hi: 0x5f10, Stride: 1},
		{Lo: 0x62d0, Hi: 0x62d0, Stride: 1},
		{Lo: 0x62fe, Hi: 0x62fe, Stride: 1},
		{Lo: 0x634c, Hi: 0x634c, Stride: 1},
		{Lo: 0x67d2, Hi: 0x67d2, Stride: 1},
		{Lo: 0x6d1e, Hi: 0x6d1e, Stride: 1},
		{Lo: 0x6f06, Hi: 0x6f06, Stride: 1},
		{Lo: 0x7396, Hi: 0x7396, Stride: 1},
		{Lo: 0x767e, Hi: 0x767e, Stride: 1},
		{Lo: 0x7695, Hi: 0x7695, Stride: 1},
		{Lo: 0x79ed, Hi: 0x79ed, Stride: 1},
		{Lo: 0x8086, Hi: 0x8086, Stride: 1},
		{Lo: 0x842c, Hi: 0x842c, Stride: 1},
		{Lo: 0x8cae, Hi: 0x8cae, Stride: 1},
		{Lo: 0x8cb3, Hi: 0x8cb3, Stride: 1},
		{Lo: 0x8d30, Hi: 0x8d30, Stride: 1},
		{Lo: 0x920e, Hi: 0x920e, Stride: 1},
		{Lo: 0x94a9, Hi: 0x94a9, Stride: 1},
		{Lo: 0x9621, Hi: 0x9621, Stride: 1},
		{Lo: 0x9646, Hi: 0x9646, Stride: 1},
		{Lo: 0x964c, Hi: 0x964c, Stride: 1},
		{Lo: 0x9678, Hi: 0x9678, Stride: 1},
		{Lo: 0x96f6, Hi: 0x96f6, Stride: 1},
		{Lo: 0xf96b, Hi: 0xf96b, Stride: 1},
		{Lo: 0xf973, Hi: 0xf973, Stride: 1},
		{Lo: 0xf978, Hi: 0xf978, Stride: 1},
		{Lo: 0xf9b2, Hi: 0xf9b2, Stride: 1},
		{Lo: 0xf9d1, Hi: 0xf9d1, Stride: 1},
		{Lo: 0xf9d3, Hi: 0xf9d3, Stride: 1},
		{Lo: 0xf9fd, Hi: 0xf9fd, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x20001, Hi: 0x20001, Stride: 1},
		{Lo: 0x20064, Hi: 0x20064, Stride: 1},
		{Lo: 0x200e2, Hi: 0x200e2, Stride: 1},
		{Lo: 0x20121, Hi: 0x20121, Stride: 1},
		{Lo: 0x2092a, Hi: 0x2092a, Stride: 1},
		{Lo: 0x20983, Hi: 0x20983, Stride: 1},
		{Lo: 0x2098c, Hi: 0x2098c, Stride: 1},
		{Lo: 0x2099c, Hi: 0x2099c, Stride: 1},
		{Lo: 0x20aea, Hi: 0x20aea, Stride: 1},
		{Lo: 0x20afd, Hi: 0x20afd, Stride: 1},
		{Lo: 0x20b19, Hi: 0x20b19, Stride: 1},
		{Lo: 0x22390, Hi: 0x22390, Stride: 1},
		{Lo: 0x22998, Hi: 0x22998, Stride: 1},
		{Lo: 0x23b1b, Hi: 0x23b1b, Stride: 1},
		{Lo: 0x2626d, Hi: 0x2626d, Stride: 1},
		{Lo: 0x2f890, Hi: 0x2f890, Stride: 1},
	},
}

func strIsDigitRune(r rune) bool {
	return unicode.IsDigit(r) || unicode.Is(strDigitOnly, r)
}

func strIsNumericRune(r rune) bool {
	return unicode.IsNumber(r) || unicode.Is(strNumericIdeographs, r)
}

func strIDStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) ||
		unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_ID_Start, r)
}

func strIDContinue(r rune) bool {
	return strIDStart(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) ||
		unicode.Is(unicode.Nd, r) || unicode.Is(unicode.Pc, r) ||
		unicode.Is(unicode.Other_ID_Continue, r)
}

func strAllRunes(s string, pred func(rune) bool) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !pred(r) {
			return false
		}
	}
	return true
}

// strPredicate evaluates one is* method. Empty-string results and the
// digit/decimal/numeric splits are all probed on 3.14; see the tests.
func strPredicate(name, s string) bool {
	switch name {
	case "isalnum":
		return strAllRunes(s, func(r rune) bool { return unicode.IsLetter(r) || strIsNumericRune(r) })
	case "isalpha":
		return strAllRunes(s, unicode.IsLetter)
	case "isascii":
		// The one predicate that is True for the empty string.
		for _, r := range s {
			if r >= 0x80 {
				return false
			}
		}
		return true
	case "isdecimal":
		return strAllRunes(s, unicode.IsDigit)
	case "isdigit":
		return strAllRunes(s, strIsDigitRune)
	case "isidentifier":
		first := true
		for _, r := range s {
			if first {
				if !strIDStart(r) {
					return false
				}
				first = false
			} else if !strIDContinue(r) {
				return false
			}
		}
		return !first
	case "islower":
		cased := false
		for _, r := range s {
			if strUpperRune(r) || unicode.IsTitle(r) {
				return false
			}
			if strLowerRune(r) {
				cased = true
			}
		}
		return cased
	case "isnumeric":
		return strAllRunes(s, strIsNumericRune)
	case "isprintable":
		// Go's IsPrint is L, M, N, P, S plus the ASCII space, the same
		// set CPython keeps. True for the empty string.
		for _, r := range s {
			if !unicode.IsPrint(r) {
				return false
			}
		}
		return true
	case "isspace":
		return strAllRunes(s, strIsSpace)
	case "istitle":
		cased, prevCased := false, false
		for _, r := range s {
			switch {
			case strUpperRune(r) || unicode.IsTitle(r):
				if prevCased {
					return false
				}
				prevCased, cased = true, true
			case strLowerRune(r):
				if !prevCased {
					return false
				}
				cased = true
			default:
				prevCased = false
			}
		}
		return cased
	case "isupper":
		cased := false
		for _, r := range s {
			if strLowerRune(r) || unicode.IsTitle(r) {
				return false
			}
			if strUpperRune(r) {
				cased = true
			}
		}
		return cased
	}
	return false
}

// str.format support. Positional fields only: the method dispatcher
// has no kwargs, so named fields raise KeyError just as CPython does
// when format() receives no keyword arguments.

// strFmtNumbering carries the auto/manual field numbering state across
// a whole template, nested spec fields included. Probed on 3.14:
// "{0:{}}" and "{:{1}}" both raise the switching ValueError.
type strFmtNumbering struct {
	auto   bool
	manual bool
	next   int
}

func strFormat(tmpl string, args []Object) (Object, error) {
	return strFormatKw(tmpl, args, nil)
}

// strFormatKw renders a template with positional args and an optional map of
// named fields, the keyword form '{name}'.format(name=value) takes. kw is nil
// for a plain positional format, so the two callers share one renderer.
func strFormatKw(tmpl string, args []Object, kw map[string]Object) (Object, error) {
	var num strFmtNumbering
	out, err := strFormatMarkup(tmpl, args, kw, &num, 0)
	if err != nil {
		return nil, err
	}
	return NewStr(out), nil
}

// strFormatMarkup renders one template. depth guards nested format
// specs: CPython allows a replacement field inside a spec once, then
// raises. Probed on 3.14: "{0:{1:{2}}}".format("x", 5, 3).
func strFormatMarkup(tmpl string, args []Object, kw map[string]Object, num *strFmtNumbering, depth int) (string, error) {
	rs := []rune(tmpl)
	var b strings.Builder
	i, n := 0, len(rs)
	for i < n {
		switch rs[i] {
		case '}':
			if i+1 < n && rs[i+1] == '}' {
				b.WriteByte('}')
				i += 2
				continue
			}
			return "", Raise(ValueError, "Single '}' encountered in format string")
		case '{':
			if i+1 < n && rs[i+1] == '{' {
				b.WriteByte('{')
				i += 2
				continue
			}
			if i+1 >= n {
				return "", Raise(ValueError, "Single '{' encountered in format string")
			}
			rendered, next, err := strFormatField(rs, i+1, args, kw, num, depth)
			if err != nil {
				return "", err
			}
			b.WriteString(rendered)
			i = next
		default:
			b.WriteRune(rs[i])
			i++
		}
	}
	return b.String(), nil
}

// strFormatField parses and renders one replacement field starting
// just past its '{'. It returns the rendered text and the index after
// the closing '}'. The parse order and every error text mirror
// CPython's unicode_format.h, all probed on 3.14.
func strFormatField(rs []rune, i int, args []Object, kw map[string]Object, num *strFmtNumbering, depth int) (string, int, error) {
	n := len(rs)
	var name []rune
	term := rune(0)
scan:
	for i < n {
		switch c := rs[i]; c {
		case '{':
			return "", 0, Raise(ValueError, "unexpected '{' in field name")
		case '[':
			// An index chunk: everything through the closing bracket is
			// part of the name, brackets included.
			name = append(name, c)
			i++
			for i < n && rs[i] != ']' {
				name = append(name, rs[i])
				i++
			}
		case '}', ':', '!':
			term = c
			i++
			break scan
		default:
			name = append(name, c)
			i++
		}
	}
	if term == 0 {
		return "", 0, Raise(ValueError, "expected '}' before end of string")
	}

	conv := rune(0)
	if term == '!' {
		if i >= n {
			return "", 0, Raise(ValueError, "end of string while looking for conversion specifier")
		}
		conv = rs[i]
		i++
		if i < n {
			switch rs[i] {
			case '}':
				term = '}'
			case ':':
				term = ':'
			default:
				return "", 0, Raise(ValueError, "expected ':' after conversion specifier")
			}
			i++
		} else {
			// EOF right after the conversion char: fall into the spec
			// scan, which reports the unmatched brace like CPython.
			term = ':'
		}
	}

	var spec []rune
	if term == ':' {
		count := 1
		for i < n {
			c := rs[i]
			i++
			if c == '{' {
				count++
			} else if c == '}' {
				count--
				if count == 0 {
					break
				}
			}
			spec = append(spec, c)
		}
		if count != 0 {
			return "", 0, Raise(ValueError, "unmatched '{' in format spec")
		}
	}

	value, err := strFormatLookup(string(name), args, kw, num)
	if err != nil {
		return "", 0, err
	}

	if conv != 0 {
		switch conv {
		case 'r':
			value = NewStr(Repr(value))
		case 's':
			value = NewStr(Str(value))
		case 'a':
			value = NewStr(strAsciiEscape(Repr(value)))
		default:
			return "", 0, Raise(ValueError, "Unknown conversion specifier %c", conv)
		}
	}

	specStr := string(spec)
	if strings.ContainsRune(specStr, '{') {
		if depth >= 1 {
			return "", 0, Raise(ValueError, "Max string recursion exceeded")
		}
		specStr, err = strFormatMarkup(specStr, args, kw, num, depth+1)
		if err != nil {
			return "", 0, err
		}
	}

	out, err := Format(value, specStr)
	if err != nil {
		return "", 0, err
	}
	res, _ := AsStr(out)
	return res, i, nil
}

// strFormatLookup resolves a field name to an argument. Empty names
// auto-number, all-digit names index the args, and anything else is a
// named field, which without kwargs is a KeyError on the part before
// any '.' or '[' path. Probed on 3.14: "{a.b}".format() is
// KeyError: 'a' and "{0.real}" style paths are out of scope here.
func strFormatLookup(name string, args []Object, kw map[string]Object, num *strFmtNumbering) (Object, error) {
	base, path := name, ""
	for k, r := range name {
		if r == '.' || r == '[' {
			base, path = name[:k], name[k:]
			break
		}
	}
	var idx int
	switch {
	case base != "" && !strAllDigits(base):
		// A named field reads from the keyword map; a miss is the KeyError
		// CPython raises for an absent keyword. base64 reaches this with
		// '{encoding}'.format(encoding='base32') at import.
		v, ok := kw[base]
		if !ok {
			return nil, NewException(KeyError, []Object{NewStr(base)})
		}
		if path != "" {
			return nil, Raise(ValueError, "unagi does not support attribute or index paths in format fields")
		}
		return v, nil
	case base == "":
		if num.manual {
			return nil, Raise(ValueError,
				"cannot switch from manual field specification to automatic field numbering")
		}
		num.auto = true
		idx = num.next
		num.next++
	case strAllDigits(base):
		if num.auto {
			return nil, Raise(ValueError,
				"cannot switch from automatic field numbering to manual field specification")
		}
		num.manual = true
		v, err := strconv.Atoi(base)
		if err != nil {
			// Probed on 3.14 with a 20-digit index.
			return nil, Raise(ValueError, "Too many decimal digits in format string")
		}
		idx = v
	}
	if idx >= len(args) {
		return nil, Raise(IndexError, "Replacement index %d out of range for positional args tuple", idx)
	}
	if path != "" {
		// Deliberate rejection: attribute and index paths need
		// getattr/getitem plumbing the compiler does not have yet.
		return nil, Raise(ValueError, "unagi does not support attribute or index paths in format fields")
	}
	return args[idx], nil
}

func strAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// strAsciiEscape rewrites a repr into ascii() form, escaping every
// non-ASCII code point. Probed on 3.14: ascii('héllo') is 'h\xe9llo'
// in quotes, ascii('嗨') uses \u and astral chars use \U.
func strAsciiEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x10000:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	return b.String()
}
