package runtime

import (
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _io.IncrementalNewlineDecoder wraps another incremental decoder and translates
// newlines as it decodes, so a TextIOWrapper reading in universal-newline mode
// turns "\r\n" and "\r" into "\n" while remembering which line endings it has
// seen. It holds a trailing "\r" back between chunks (pendingcr) so a "\r\n"
// split across two decode calls still collapses to one "\n", and records the
// endings seen in a three-bit set exposed through the newlines property. Unlike
// the stream classes it subclasses object directly, not _IOBase. This is
// sub-slice 5h (IncrementalNewlineDecoder, the first piece TextIOWrapper needs)
// of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io shim has none, so
// nothing runs in parallel.
var ioIncrementalNewlineDecoderClass objects.Object

// The three newline kinds are tracked as a bit set: seenCR|seenLF|seenCRLF.
const (
	nlSeenCR   = 1
	nlSeenLF   = 2
	nlSeenCRLF = 4
)

// nlNewlinesTable maps the seen-newline bit set (0..7) to the value the newlines
// property reports: None, a single ending, or a tuple of endings in CR, LF,
// CR-LF order.
func nlNewlinesValue(seen int) objects.Object {
	switch seen {
	case 0:
		return objects.None
	case nlSeenCR:
		return objects.NewStr("\r")
	case nlSeenLF:
		return objects.NewStr("\n")
	case nlSeenCR | nlSeenLF:
		return objects.NewTuple([]objects.Object{objects.NewStr("\r"), objects.NewStr("\n")})
	case nlSeenCRLF:
		return objects.NewStr("\r\n")
	case nlSeenCR | nlSeenCRLF:
		return objects.NewTuple([]objects.Object{objects.NewStr("\r"), objects.NewStr("\r\n")})
	case nlSeenLF | nlSeenCRLF:
		return objects.NewTuple([]objects.Object{objects.NewStr("\n"), objects.NewStr("\r\n")})
	default: // all three
		return objects.NewTuple([]objects.Object{objects.NewStr("\r"), objects.NewStr("\n"), objects.NewStr("\r\n")})
	}
}

// buildIOIncrementalNewlineDecoder constructs the _io.IncrementalNewlineDecoder
// classObject.
func buildIOIncrementalNewlineDecoder() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_decoder"), objects.NewStr("_translate"), objects.NewStr("_errors"),
		objects.NewStr("_pendingcr"), objects.NewStr("_seennl"),
	})
	names := []string{
		"__slots__", "__init__",
		"decode", "getstate", "setstate", "reset",
		"newlines",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioNLDInit),
		ioMethod("decode", -1, ioNLDDecode),
		ioMethod("getstate", 1, ioNLDGetstate),
		ioMethod("setstate", 2, ioNLDSetstate),
		ioMethod("reset", 1, ioNLDReset),
		objects.NewProperty(objects.NewFunc("newlines", 1, ioNLDNewlinesProp), nil, nil),
	}
	return objects.NewClass("IncrementalNewlineDecoder", "_io.IncrementalNewlineDecoder",
		nil, names, vals, nil, nil)
}

// ioNLDInit stores the wrapped decoder, the translate flag and the errors name.
// The signature is IncrementalNewlineDecoder(decoder, translate, errors='strict').
func ioNLDInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	rest := pos[1:]
	if len(rest) > 3 {
		return nil, objects.Raise(objects.TypeError, "IncrementalNewlineDecoder() takes at most 3 arguments (%d given)", len(rest))
	}
	// decoder, translate and errors may each arrive positionally or by keyword.
	var decoder, translate objects.Object
	errors := objects.Object(objects.NewStr("strict"))
	if len(rest) >= 1 {
		decoder = rest[0]
	}
	if len(rest) >= 2 {
		translate = rest[1]
	}
	if len(rest) >= 3 {
		errors = rest[2]
	}
	for i, name := range kwNames {
		switch name {
		case "decoder":
			decoder = kwVals[i]
		case "translate":
			translate = kwVals[i]
		case "errors":
			errors = kwVals[i]
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for IncrementalNewlineDecoder()", name)
		}
	}
	if decoder == nil {
		return nil, objects.Raise(objects.TypeError, "IncrementalNewlineDecoder() missing required argument 'decoder' (pos 1)")
	}
	if translate == nil {
		return nil, objects.Raise(objects.TypeError, "IncrementalNewlineDecoder() missing required argument 'translate' (pos 2)")
	}
	if err := objects.StoreAttr(self, "_decoder", decoder); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_translate", objects.NewBool(objects.Truth(translate))); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_errors", errors); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_pendingcr", objects.False); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_seennl", objects.NewInt(0)); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioNLDDecode decodes the input through the wrapped decoder, then holds a
// trailing carriage return between chunks, records the newline kinds seen and
// (when translate is set) rewrites "\r\n" and "\r" to "\n".
func ioNLDDecode(args []objects.Object) (objects.Object, error) {
	self := args[0]
	input := objects.NewBytes(nil)
	if len(args) >= 2 {
		input = args[1]
	}
	final := false
	if len(args) >= 3 {
		final = objects.Truth(args[2])
	}
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	var output string
	if decoder == objects.None {
		s, ok := objects.AsStr(input)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "a str object is required, not '%s'", input.TypeName())
		}
		output = s
	} else {
		res, err := objects.CallMethod(decoder, "decode", []objects.Object{input, objects.NewBool(final)})
		if err != nil {
			return nil, err
		}
		s, ok := objects.AsStr(res)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "decoder should return a str result, not '%s'", res.TypeName())
		}
		output = s
	}
	pendingcr := nlPendingCR(self)
	if pendingcr && (len(output) > 0 || final) {
		output = "\r" + output
		pendingcr = false
	}
	if !final && strings.HasSuffix(output, "\r") {
		output = output[:len(output)-1]
		pendingcr = true
	}
	if err := objects.StoreAttr(self, "_pendingcr", objects.NewBool(pendingcr)); err != nil {
		return nil, err
	}
	crlf := strings.Count(output, "\r\n")
	cr := strings.Count(output, "\r") - crlf
	lf := strings.Count(output, "\n") - crlf
	seen := nlSeen(self)
	if lf > 0 {
		seen |= nlSeenLF
	}
	if cr > 0 {
		seen |= nlSeenCR
	}
	if crlf > 0 {
		seen |= nlSeenCRLF
	}
	if err := objects.StoreAttr(self, "_seennl", objects.NewInt(int64(seen))); err != nil {
		return nil, err
	}
	if nlTranslate(self) {
		if crlf > 0 {
			output = strings.ReplaceAll(output, "\r\n", "\n")
		}
		if cr > 0 {
			output = strings.ReplaceAll(output, "\r", "\n")
		}
	}
	return objects.NewStr(output), nil
}

// ioNLDGetstate returns the wrapped decoder's byte buffer and a flag that folds
// the pending carriage return into the low bit above the decoder's own flag.
func ioNLDGetstate(args []objects.Object) (objects.Object, error) {
	self := args[0]
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	buf := objects.NewBytes(nil)
	flag := int64(0)
	if decoder != objects.None {
		st, err := objects.CallMethod(decoder, "getstate", nil)
		if err != nil {
			return nil, err
		}
		items, err := objects.Unpack(st, 2)
		if err != nil {
			return nil, err
		}
		buf = items[0]
		n, _ := objects.AsInt(items[1])
		flag = n
	}
	flag <<= 1
	if nlPendingCR(self) {
		flag |= 1
	}
	return objects.NewTuple([]objects.Object{buf, objects.NewInt(flag)}), nil
}

// ioNLDSetstate restores the pending carriage return and the wrapped decoder's
// state from a tuple produced by getstate.
func ioNLDSetstate(args []objects.Object) (objects.Object, error) {
	self, state := args[0], args[1]
	items, err := objects.Unpack(state, 2)
	if err != nil {
		return nil, err
	}
	buf := items[0]
	flag, _ := objects.AsInt(items[1])
	if err := objects.StoreAttr(self, "_pendingcr", objects.NewBool(flag&1 != 0)); err != nil {
		return nil, err
	}
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	if decoder != objects.None {
		inner := objects.NewTuple([]objects.Object{buf, objects.NewInt(flag >> 1)})
		if _, err := objects.CallMethod(decoder, "setstate", []objects.Object{inner}); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

// ioNLDReset clears the seen-newline set and pending carriage return and resets
// the wrapped decoder.
func ioNLDReset(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := objects.StoreAttr(self, "_seennl", objects.NewInt(0)); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_pendingcr", objects.False); err != nil {
		return nil, err
	}
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	if decoder != objects.None {
		if _, err := objects.CallMethod(decoder, "reset", nil); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

// ioNLDNewlinesProp reports the newline kinds seen so far.
func ioNLDNewlinesProp(args []objects.Object) (objects.Object, error) {
	return nlNewlinesValue(nlSeen(args[0])), nil
}

// nlPendingCR reports whether a trailing carriage return is held over.
func nlPendingCR(self objects.Object) bool {
	v, err := objects.LoadAttr(self, "_pendingcr")
	if err != nil {
		return false
	}
	return objects.Truth(v)
}

// nlTranslate reports whether newline translation is enabled.
func nlTranslate(self objects.Object) bool {
	v, err := objects.LoadAttr(self, "_translate")
	if err != nil {
		return false
	}
	return objects.Truth(v)
}

// nlSeen reads the seen-newline bit set.
func nlSeen(self objects.Object) int {
	v, err := objects.LoadAttr(self, "_seennl")
	if err != nil {
		return 0
	}
	n, _ := objects.AsInt(v)
	return int(n)
}
