package runtime

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/objects"
)

// escape_encode and readbuffer_encode are two _codecs C accelerators the codecs
// module binds through `from _codecs import *`. Neither is a real codec entry in
// the registry; both are small buffer helpers pickle and a few callers reach for.
//
// escape_encode(data, errors=None) renders a bytes object with the C bytes-repr
// escapes (backslash, single quote, the \t \n \r names and \xNN for every byte
// outside printable ASCII) and returns the (escaped bytes, input length) pair the
// other _codecs functions use. It takes an exact bytes object, not a bytearray or
// str, the way the C O! format with &PyBytes_Type does.
//
// readbuffer_encode(data, errors=None) hands back the raw bytes behind a
// bytes-like object, or a str encoded as UTF-8, paired with the byte length. It
// is the buffer-protocol passthrough the C s* format implements.

// bytesEscapeEncode renders v with the C escape_encode catalog: the quote is
// always a single quote (so a double quote is left raw and a single quote is
// escaped), and every byte below 0x20 or at/above 0x7f prints as \xNN.
func bytesEscapeEncode(v []byte) []byte {
	out := make([]byte, 0, len(v))
	for _, c := range v {
		switch {
		case c == '\'' || c == '\\':
			out = append(out, '\\', c)
		case c == '\t':
			out = append(out, '\\', 't')
		case c == '\n':
			out = append(out, '\\', 'n')
		case c == '\r':
			out = append(out, '\\', 'r')
		case c < 0x20 || c >= 0x7f:
			out = append(out, fmt.Sprintf(`\x%02x`, c)...)
		default:
			out = append(out, c)
		}
	}
	return out
}

func codecEscapeEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "escape_encode expected at least 1 argument, got 0")
	}
	// The C function parses argument 1 with O! against the exact bytes type, so a
	// bytearray or str is rejected, not encoded.
	v, ok := objects.AsBytes(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "escape_encode() argument 1 must be bytes, not %s", pos[0].TypeName())
	}
	out := bytesEscapeEncode(v)
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(v)))}), nil
}

func codecReadbufferEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "readbuffer_encode expected at least 1 argument, got 0")
	}
	// The C s* format takes any object exposing the buffer protocol verbatim and
	// encodes a str as UTF-8; anything else raises the bytes-like TypeError.
	var v []byte
	if s, ok := objects.AsStr(pos[0]); ok {
		b, err := objects.EncodeStr(s, "utf-8", "strict")
		if err != nil {
			return nil, err
		}
		v = b
	} else if b, ok := objects.AsBufferBytes(pos[0]); ok {
		v = b
	} else {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", pos[0].TypeName())
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(v), objects.NewInt(int64(len(v)))}), nil
}
