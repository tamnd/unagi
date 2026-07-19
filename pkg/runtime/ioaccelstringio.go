package runtime

import (
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _io.StringIO is the in-memory text stream: a growable unicode buffer with a
// character cursor, subclassing _TextIOBase. Like BytesIO it holds its state in
// hidden __slots__ and inherits close/closed, the context manager, iteration and
// writelines from _IOBase, but it carries the text-specific newline model. The
// newline argument (default "\n") controls how "\n" is rewritten on write and
// which sequences count as line ends and get universally decoded into the
// buffer, and the newlines property reports the kinds seen. This is sub-slice 5e
// of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io.StringIO shim stays
// in place until the flip.
var ioStringIOClass objects.Object

// The newlines property reports which line-ending kinds the universal decoder
// has seen, tracked as a bitmask on the _seennl slot.
const (
	sioSeenCR   = 1
	sioSeenLF   = 2
	sioSeenCRLF = 4
)

// sioClosedMessage is StringIO's own closed-file message; note it lacks the
// trailing period the _IOBase.close path uses, matching the C StringIO exactly.
const sioClosedMessage = "I/O operation on closed file"

// buildIOStringIO constructs the _io.StringIO classObject. The buffer lives in
// the _buf slot as a str, the cursor in _pos, and the newline configuration in
// _readnl (the original argument), _writenl (the precomputed write replacement)
// and _seennl (the seen-newlines bitmask), all hidden from __dict__ by __slots__.
func buildIOStringIO() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_buf"), objects.NewStr("_pos"),
		objects.NewStr("_readnl"), objects.NewStr("_writenl"),
		objects.NewStr("_seennl"),
	})
	names := []string{
		"__slots__", "__init__",
		"read", "readline", "write",
		"seek", "tell", "truncate", "getvalue",
		"readable", "writable", "seekable",
		"newlines",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioStringIOInit),
		ioMethod("read", -1, ioStringIORead),
		ioMethod("readline", -1, ioStringIOReadline),
		ioMethod("write", 2, ioStringIOWrite),
		ioMethod("seek", -1, ioStringIOSeek),
		ioMethod("tell", 1, ioStringIOTell),
		ioMethod("truncate", -1, ioStringIOTruncate),
		ioMethod("getvalue", 1, ioStringIOGetvalue),
		ioStringIOPredicate("readable"),
		ioStringIOPredicate("writable"),
		ioStringIOPredicate("seekable"),
		objects.NewProperty(objects.NewFunc("newlines", 1, ioStringIONewlines), nil, nil),
	}
	return objects.NewClass("StringIO", "_io.StringIO",
		[]objects.Object{ioTextIOBase}, names, vals, nil, nil)
}

// ioStringIOInit configures the newline mode and stores the optional initial
// value, decoded through the same write path so the buffer holds already
// translated text; the cursor is then reset to the start. The signature is
// StringIO(initial_value="", newline="\n").
func ioStringIOInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	initialValue := objects.Object(objects.None)
	newline := objects.Object(objects.NewStr("\n"))
	rest := pos[1:]
	if len(rest) > 2 {
		return nil, objects.Raise(objects.TypeError, "StringIO() takes at most 2 arguments (%d given)", len(rest))
	}
	if len(rest) >= 1 {
		initialValue = rest[0]
	}
	if len(rest) >= 2 {
		newline = rest[1]
	}
	for i, name := range kwNames {
		switch name {
		case "initial_value":
			initialValue = kwVals[i]
		case "newline":
			newline = kwVals[i]
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for StringIO()", name)
		}
	}
	// Derive the write replacement from the newline argument. Only "" turns the
	// write translation off; None and "\n" both replace "\n" with "\n" (a no-op
	// that still runs), and "\r"/"\r\n" rewrite the line ending.
	readnl := objects.Object(objects.None)
	writenl := objects.Object(objects.NewStr("\n"))
	if newline != objects.None {
		s, ok := objects.AsStr(newline)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "newline must be str or None, not %s", newline.TypeName())
		}
		switch s {
		case "":
			writenl = objects.None
		case "\n", "\r", "\r\n":
			writenl = objects.NewStr(s)
		default:
			return nil, objects.Raise(objects.ValueError, "illegal newline value: %s", objects.Repr(newline))
		}
		readnl = objects.NewStr(s)
	}
	if err := objects.StoreAttr(self, "_readnl", readnl); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_writenl", writenl); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_seennl", objects.NewInt(0)); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_buf", objects.NewStr("")); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_pos", objects.NewInt(0)); err != nil {
		return nil, err
	}
	// The initial value runs through the write translation, then the cursor is
	// reset to the start. It must be str or None.
	if initialValue != objects.None {
		initial, ok := objects.AsStr(initialValue)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "initial_value must be str or None, not %s", initialValue.TypeName())
		}
		trans, err := sioTranslate(self, initial)
		if err != nil {
			return nil, err
		}
		if err := objects.StoreAttr(self, "_buf", objects.NewStr(trans)); err != nil {
			return nil, err
		}
		if err := objects.StoreAttr(self, "_pos", objects.NewInt(0)); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

// ioStringIORead returns up to size characters from the cursor, advancing it. A
// missing, None or negative size reads the whole remainder.
func ioStringIORead(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := sioCheckClosed(self); err != nil {
		return nil, err
	}
	buf, pos, err := sioState(self)
	if err != nil {
		return nil, err
	}
	avail := len(buf) - pos
	if avail < 0 {
		avail = 0
	}
	n := avail
	if len(args) >= 2 && args[1] != objects.None {
		size, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		if size >= 0 && int(size) < avail {
			n = int(size)
		}
	}
	start := pos
	if start > len(buf) {
		start = len(buf)
	}
	out := string(buf[start : start+n])
	if err := sioSetPos(self, pos+n); err != nil {
		return nil, err
	}
	return objects.NewStr(out), nil
}

// ioStringIOReadline returns the next line including its terminator, capped at
// size characters when size is given. The terminator depends on the newline
// mode: only "\n" for None (the buffer is already translated) and for "\n", any
// of "\r"/"\n"/"\r\n" for "" (universal), and the exact sequence for "\r"/"\r\n".
func ioStringIOReadline(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := sioCheckClosed(self); err != nil {
		return nil, err
	}
	buf, pos, err := sioState(self)
	if err != nil {
		return nil, err
	}
	limit := -1
	if len(args) >= 2 && args[1] != objects.None {
		sz, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		limit = int(sz)
	}
	readnl, isNone, err := sioReadnl(self)
	if err != nil {
		return nil, err
	}
	start := pos
	if start > len(buf) {
		start = len(buf)
	}
	// The scan strategy is fixed by the newline mode, so resolve it once: only
	// "\n" for None and for "\n", any ending for "" (universal), else the exact
	// sequence.
	lfOnly := isNone || readnl == "\n"
	universal := readnl == ""
	term := []rune(readnl)
	end := len(buf)
	for i := start; i < len(buf); i++ {
		if lfOnly {
			if buf[i] == '\n' {
				end = i + 1
				break
			}
			continue
		}
		if universal {
			if buf[i] == '\n' {
				end = i + 1
				break
			}
			if buf[i] == '\r' {
				if i+1 < len(buf) && buf[i+1] == '\n' {
					end = i + 2
				} else {
					end = i + 1
				}
				break
			}
			continue
		}
		if sioMatch(buf, i, term) {
			end = i + len(term)
			break
		}
	}
	stop := end
	if limit >= 0 && start+limit < stop {
		stop = start + limit
	}
	out := string(buf[start:stop])
	if err := sioSetPos(self, stop); err != nil {
		return nil, err
	}
	return objects.NewStr(out), nil
}

// ioStringIOWrite rewrites and decodes s per the newline mode, splices it into
// the buffer at the cursor (zero-padding a gap left by a seek beyond the end),
// then advances the cursor and returns the original character count.
func ioStringIOWrite(args []objects.Object) (objects.Object, error) {
	self, arg := args[0], args[1]
	if err := sioCheckClosed(self); err != nil {
		return nil, err
	}
	s, ok := objects.AsStr(arg)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "string argument expected, got '%s'", arg.TypeName())
	}
	n := len([]rune(s))
	trans, err := sioTranslate(self, s)
	if err != nil {
		return nil, err
	}
	buf, pos, err := sioState(self)
	if err != nil {
		return nil, err
	}
	data := []rune(trans)
	end := pos + len(data)
	newLen := len(buf)
	if end > newLen {
		newLen = end
	}
	nb := make([]rune, newLen)
	copy(nb, buf)
	// make zero-values the gap between the old end and pos to NUL runes, matching
	// CPython's zero-fill on a write past the end.
	copy(nb[pos:], data)
	if err := sioSetBuf(self, nb); err != nil {
		return nil, err
	}
	if err := sioSetPos(self, end); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(n)), nil
}

// ioStringIOSeek moves the character cursor. Whence 0 is an absolute position
// (negative raises), whence 1 and 2 accept only offset 0 (current position and
// end), matching the C StringIO which forbids nonzero relative text seeks.
func ioStringIOSeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := sioCheckClosed(self); err != nil {
		return nil, err
	}
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "seek() takes at least 1 argument (0 given)")
	}
	off, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
	}
	whence := int64(0)
	if len(args) >= 3 && args[2] != objects.None {
		w, ok := objects.AsInt(args[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[2].TypeName())
		}
		whence = w
	}
	buf, pos, err := sioState(self)
	if err != nil {
		return nil, err
	}
	switch whence {
	case 0:
		if off < 0 {
			return nil, objects.Raise(objects.ValueError, "Negative seek position %d", off)
		}
		if err := sioSetPos(self, int(off)); err != nil {
			return nil, err
		}
		return objects.NewInt(off), nil
	case 1:
		if off != 0 {
			return nil, objects.Raise("OSError", "Can't do nonzero cur-relative seeks")
		}
		return objects.NewInt(int64(pos)), nil
	case 2:
		if off != 0 {
			return nil, objects.Raise("OSError", "Can't do nonzero cur-relative seeks")
		}
		if err := sioSetPos(self, len(buf)); err != nil {
			return nil, err
		}
		return objects.NewInt(int64(len(buf))), nil
	default:
		return nil, objects.Raise(objects.ValueError, "Invalid whence (%d, should be 0, 1 or 2)", whence)
	}
}

// ioStringIOTell returns the current character position.
func ioStringIOTell(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := sioCheckClosed(self); err != nil {
		return nil, err
	}
	_, pos, err := sioState(self)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(pos)), nil
}

// ioStringIOTruncate shrinks the buffer to size, or to the cursor when size is
// missing, leaving the cursor put and returning size.
func ioStringIOTruncate(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := sioCheckClosed(self); err != nil {
		return nil, err
	}
	buf, pos, err := sioState(self)
	if err != nil {
		return nil, err
	}
	size := pos
	if len(args) >= 2 && args[1] != objects.None {
		n, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		if n < 0 {
			return nil, objects.Raise(objects.ValueError, "Negative size value %d", n)
		}
		size = int(n)
	}
	if size < len(buf) {
		if err := sioSetBuf(self, buf[:size]); err != nil {
			return nil, err
		}
	}
	return objects.NewInt(int64(size)), nil
}

// ioStringIOGetvalue returns the whole buffer as str, independent of the cursor.
func ioStringIOGetvalue(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := sioCheckClosed(self); err != nil {
		return nil, err
	}
	buf, _, err := sioState(self)
	if err != nil {
		return nil, err
	}
	return objects.NewStr(string(buf)), nil
}

// ioStringIONewlines reports the line-ending kinds the universal decoder has
// seen: None, a single string, or a tuple in CR/LF/CRLF order.
func ioStringIONewlines(args []objects.Object) (objects.Object, error) {
	seen, err := sioSeen(args[0])
	if err != nil {
		return nil, err
	}
	var parts []objects.Object
	if seen&sioSeenCR != 0 {
		parts = append(parts, objects.NewStr("\r"))
	}
	if seen&sioSeenLF != 0 {
		parts = append(parts, objects.NewStr("\n"))
	}
	if seen&sioSeenCRLF != 0 {
		parts = append(parts, objects.NewStr("\r\n"))
	}
	switch len(parts) {
	case 0:
		return objects.None, nil
	case 1:
		return parts[0], nil
	default:
		return objects.NewTuple(parts), nil
	}
}

// ioStringIOPredicate builds readable/writable/seekable: each raises on a closed
// stream and otherwise reports true.
func ioStringIOPredicate(name string) objects.Object {
	return objects.NewMethod(name, 1, func(args []objects.Object) (objects.Object, error) {
		if err := sioCheckClosed(args[0]); err != nil {
			return nil, err
		}
		return objects.True, nil
	})
}

// sioTranslate applies the write-side newline rewrite and universal decode to s,
// updating the seen-newlines mark, and returns the text to store. Each call
// stands alone (a final decode), so a "\r" at the end is emitted immediately and
// a "\r\n" split across two writes is not recombined, matching the C StringIO.
func sioTranslate(self objects.Object, s string) (string, error) {
	writenl, hasWritenl, readuniversal, readtranslate, err := sioConfig(self)
	if err != nil {
		return "", err
	}
	if hasWritenl {
		s = strings.ReplaceAll(s, "\n", writenl)
	}
	if readuniversal {
		out, seen := sioDecode(s, readtranslate)
		s = out
		if seen != 0 {
			cur, err := sioSeen(self)
			if err != nil {
				return "", err
			}
			if err := objects.StoreAttr(self, "_seennl", objects.NewInt(int64(cur|seen))); err != nil {
				return "", err
			}
		}
	}
	return s, nil
}

// sioDecode runs one universal-newline decode pass over s, returning the decoded
// text and the bitmask of line endings seen. When translate is true, "\r" and
// "\r\n" collapse to "\n"; otherwise the text is unchanged and only the seen
// mark is tracked (the newline="" mode).
func sioDecode(s string, translate bool) (string, int) {
	runes := []rune(s)
	var b strings.Builder
	seen := 0
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '\r':
			if i+1 < len(runes) && runes[i+1] == '\n' {
				seen |= sioSeenCRLF
				i++
				if translate {
					b.WriteByte('\n')
				} else {
					b.WriteString("\r\n")
				}
			} else {
				seen |= sioSeenCR
				if translate {
					b.WriteByte('\n')
				} else {
					b.WriteByte('\r')
				}
			}
		case '\n':
			seen |= sioSeenLF
			b.WriteByte('\n')
		default:
			b.WriteRune(runes[i])
		}
	}
	return b.String(), seen
}

// sioMatch reports whether term appears in runes starting at index i.
func sioMatch(runes []rune, i int, term []rune) bool {
	if i+len(term) > len(runes) {
		return false
	}
	for j, c := range term {
		if runes[i+j] != c {
			return false
		}
	}
	return true
}

// sioConfig reads the newline configuration off the instance slots. readuniversal
// (None or "") drives the decoder; readtranslate (None only) makes it collapse
// endings to "\n"; hasWritenl is false only for the newline="" mode.
func sioConfig(self objects.Object) (writenl string, hasWritenl, readuniversal, readtranslate bool, err error) {
	rn, err := objects.LoadAttr(self, "_readnl")
	if err != nil {
		return "", false, false, false, err
	}
	if rn == objects.None {
		readuniversal, readtranslate = true, true
	} else if s, _ := objects.AsStr(rn); s == "" {
		readuniversal = true
	}
	wn, err := objects.LoadAttr(self, "_writenl")
	if err != nil {
		return "", false, false, false, err
	}
	if wn != objects.None {
		writenl, _ = objects.AsStr(wn)
		hasWritenl = true
	}
	return writenl, hasWritenl, readuniversal, readtranslate, nil
}

// sioReadnl returns the readline terminator: the empty string with isNone true
// for newline=None (the buffer is already "\n"-translated, so readline scans for
// "\n"), else the original newline argument.
func sioReadnl(self objects.Object) (readnl string, isNone bool, err error) {
	rn, err := objects.LoadAttr(self, "_readnl")
	if err != nil {
		return "", false, err
	}
	if rn == objects.None {
		return "", true, nil
	}
	s, _ := objects.AsStr(rn)
	return s, false, nil
}

// sioState reads the buffer (as runes for character indexing) and cursor slots.
func sioState(self objects.Object) ([]rune, int, error) {
	v, err := objects.LoadAttr(self, "_buf")
	if err != nil {
		return nil, 0, err
	}
	s, _ := objects.AsStr(v)
	p, err := objects.LoadAttr(self, "_pos")
	if err != nil {
		return nil, 0, err
	}
	pos, _ := objects.AsInt(p)
	return []rune(s), int(pos), nil
}

// sioSeen reads the seen-newlines bitmask slot.
func sioSeen(self objects.Object) (int, error) {
	v, err := objects.LoadAttr(self, "_seennl")
	if err != nil {
		return 0, err
	}
	n, _ := objects.AsInt(v)
	return int(n), nil
}

// sioSetBuf writes the buffer slot.
func sioSetBuf(self objects.Object, runes []rune) error {
	return objects.StoreAttr(self, "_buf", objects.NewStr(string(runes)))
}

// sioSetPos writes the cursor slot.
func sioSetPos(self objects.Object, pos int) error {
	return objects.StoreAttr(self, "_pos", objects.NewInt(int64(pos)))
}

// sioCheckClosed raises StringIO's closed-file ValueError when the stream is
// closed, using the same closed mark _IOBase.close sets.
func sioCheckClosed(self objects.Object) error {
	if ioIsClosed(self) {
		return objects.Raise(objects.ValueError, "%s", sioClosedMessage)
	}
	return nil
}
