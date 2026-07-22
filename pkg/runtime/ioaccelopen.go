package runtime

import (
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _io.open is the factory the builtin open() and io.open resolve to: it parses a
// mode string, opens a raw FileIO, wraps it in the matching buffered layer, and
// for a text mode wraps that in a TextIOWrapper. open_code is open(path, "rb"),
// the hook the import system uses to read source. Both live here because they are
// the top of the FileIO stack this sub-slice (5f) stands up.

// ioBuiltinOpen is the single open function object: _io exports it, io.py
// re-exports that, and the builtin namespace binds the very same object, so
// `io.open is open` holds the way it does in CPython. Keeping one object avoids
// the two-distinct-functions identity mismatch.
var ioBuiltinOpen = objects.NewFuncKw("open", ioOpen)

// ioOpen implements
// open(file, mode='r', buffering=-1, encoding=None, errors=None,
//
//	newline=None, closefd=True, opener=None).
func ioOpen(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	var (
		file                            objects.Object
		mode                            = "r"
		buffering                       = int64(-1)
		haveBuffering                   bool
		encoding, errs, newline, opener = objects.None, objects.None, objects.None, objects.None
		closefd                         = true
	)
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "open() missing required argument 'file' (pos 1)")
	}
	if len(pos) > 8 {
		return nil, objects.Raise(objects.TypeError, "open() takes at most 8 arguments (%d given)", len(pos))
	}
	file = pos[0]
	setMode := func(o objects.Object) error {
		s, ok := objects.AsStr(o)
		if !ok {
			return objects.Raise(objects.TypeError, "open() argument 'mode' must be str, not %s", o.TypeName())
		}
		mode = s
		return nil
	}
	setBuffering := func(o objects.Object) error {
		n, ok := objects.AsInt(o)
		if !ok {
			return objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
		}
		buffering, haveBuffering = n, true
		return nil
	}
	if len(pos) >= 2 {
		if err := setMode(pos[1]); err != nil {
			return nil, err
		}
	}
	if len(pos) >= 3 {
		if err := setBuffering(pos[2]); err != nil {
			return nil, err
		}
	}
	if len(pos) >= 4 {
		encoding = pos[3]
	}
	if len(pos) >= 5 {
		errs = pos[4]
	}
	if len(pos) >= 6 {
		newline = pos[5]
	}
	if len(pos) >= 7 {
		closefd = objects.Truth(pos[6])
	}
	if len(pos) >= 8 {
		opener = pos[7]
	}
	for i, name := range kwNames {
		v := kwVals[i]
		switch name {
		case "mode":
			if err := setMode(v); err != nil {
				return nil, err
			}
		case "buffering":
			if err := setBuffering(v); err != nil {
				return nil, err
			}
		case "encoding":
			encoding = v
		case "errors":
			errs = v
		case "newline":
			newline = v
		case "closefd":
			closefd = objects.Truth(v)
		case "opener":
			opener = v
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for open()", name)
		}
	}

	reading, writing, appending, creating, updating, text, binary, err := ioParseOpenMode(mode)
	if err != nil {
		return nil, err
	}
	if text && binary {
		return nil, objects.Raise(objects.ValueError, "can't have text and binary mode at once")
	}
	if binary && encoding != objects.None {
		return nil, objects.Raise(objects.ValueError, "binary mode doesn't take an encoding argument")
	}
	if binary && errs != objects.None {
		return nil, objects.Raise(objects.ValueError, "binary mode doesn't take an errors argument")
	}
	if binary && newline != objects.None {
		return nil, objects.Raise(objects.ValueError, "binary mode doesn't take a newline argument")
	}

	// The raw stream carries only the create/read/write/append part of the mode;
	// the +/text/binary decisions are made above the raw layer.
	rawMode := ioRawModeString(reading, writing, appending, creating, updating)
	rawArgs := []objects.Object{file, objects.NewStr(rawMode), objects.NewBool(closefd)}
	var rawKwNames []string
	var rawKwVals []objects.Object
	if opener != objects.None {
		rawKwNames = []string{"opener"}
		rawKwVals = []objects.Object{opener}
	}
	raw, err := objects.CallKw(ioFileIOClass, rawArgs, rawKwNames, rawKwVals)
	if err != nil {
		return nil, err
	}

	// buffering == 0 asks for an unbuffered stream, only legal in binary mode; it
	// hands back the raw FileIO directly.
	if haveBuffering && buffering == 0 {
		if !binary {
			return nil, objects.Raise(objects.ValueError, "can't have unbuffered text I/O")
		}
		return raw, nil
	}

	bufsize := int64(131072)
	if haveBuffering && buffering > 1 {
		bufsize = buffering
	}

	var buffer objects.Object
	switch {
	case updating:
		buffer, err = objects.Call(ioBufferedRandomClass, []objects.Object{raw, objects.NewInt(bufsize)})
	case writing || appending || creating:
		buffer, err = objects.Call(ioBufferedWriterClass, []objects.Object{raw, objects.NewInt(bufsize)})
	default:
		buffer, err = objects.Call(ioBufferedReaderClass, []objects.Object{raw, objects.NewInt(bufsize)})
	}
	if err != nil {
		closeOnError(raw)
		return nil, err
	}
	if binary {
		return buffer, nil
	}

	// Text mode layers a TextIOWrapper over the buffered stream, threading the
	// settled encoding, errors and newline through.
	enc := encoding
	if enc == objects.None {
		enc = objects.NewStr("utf-8")
	}
	textErrs := errs
	if textErrs == objects.None {
		textErrs = objects.NewStr("strict")
	}
	wrapper, err := objects.Call(ioTextIOWrapperClass, []objects.Object{buffer, enc, textErrs, newline})
	if err != nil {
		closeOnError(buffer)
		return nil, err
	}
	return wrapper, nil
}

// ioOpenCode implements open_code(path) == open(path, "rb"): the import system's
// hook for reading a code file, with no audit or override in this build.
func ioOpenCode(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "open_code() takes exactly 1 argument (%d given)", len(args))
	}
	return ioOpen([]objects.Object{args[0], objects.NewStr("rb")}, nil, nil)
}

// ioParseOpenMode decomposes an open() mode string into its flags. It enforces
// exactly one of r/w/x/a, at most one '+', and at most one of 't'/'b', matching
// CPython's validation and error text.
func ioParseOpenMode(mode string) (reading, writing, appending, creating, updating, text, binary bool, err error) {
	seen := map[byte]bool{}
	for i := 0; i < len(mode); i++ {
		c := mode[i]
		if !strings.ContainsRune("xrwab+t", rune(c)) {
			return false, false, false, false, false, false, false,
				objects.Raise(objects.ValueError, "invalid mode: '%s'", mode)
		}
		if seen[c] {
			return false, false, false, false, false, false, false,
				objects.Raise(objects.ValueError, "invalid mode: '%s'", mode)
		}
		seen[c] = true
	}
	reading, writing, creating, appending = seen['r'], seen['w'], seen['x'], seen['a']
	updating, text, binary = seen['+'], seen['t'], seen['b']
	n := 0
	for _, c := range []byte{'r', 'w', 'x', 'a'} {
		if seen[c] {
			n++
		}
	}
	if n != 1 {
		return false, false, false, false, false, false, false,
			objects.Raise(objects.ValueError, "must have exactly one of create/read/write/append mode")
	}
	if !text && !binary {
		text = true
	}
	return reading, writing, appending, creating, updating, text, binary, nil
}

// ioRawModeString builds the FileIO mode for the raw layer from the decoded open
// flags: the single create/read/write/append letter plus a '+' when the mode
// updates.
func ioRawModeString(reading, writing, appending, creating, updating bool) string {
	var b strings.Builder
	switch {
	case creating:
		b.WriteByte('x')
	case writing:
		b.WriteByte('w')
	case appending:
		b.WriteByte('a')
	default:
		b.WriteByte('r')
	}
	if updating {
		b.WriteByte('+')
	}
	return b.String()
}

// closeOnError closes a stream while unwinding from a failed open(), ignoring any
// secondary error so the original one propagates.
func closeOnError(o objects.Object) {
	_, _ = objects.CallMethod(o, "close", nil)
}
