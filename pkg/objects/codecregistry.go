package objects

// The utf-8, ascii and latin-1 codec families are built into this package so
// str.encode, bytes.decode and the two-argument bytes/str constructors resolve
// them without importing anything. Every other codec — utf-16, hex_codec,
// rot_13, utf-8-sig and the rest of the encodings package — is a pure-Python
// module the runtime resolves through the codec registry, a layer above this
// one. These hooks let the built-in codec paths fall through to that registry
// for a name they do not handle, so "hi".encode("utf-8-sig") reaches the same
// codec codecs.encode("hi", "utf-8-sig") does. The runtime's _codecs module
// installs them at import; while they are nil an unknown codec raises the
// ordinary LookupError, the behavior before the registry is wired.
var (
	// CodecEncodeHook encodes a str under a codec the core switch does not
	// handle, resolving it through the registry's search functions. It returns
	// the encoded bytes, or a LookupError if no registered codec claims enc.
	CodecEncodeHook func(s, enc, errh string) ([]byte, error)

	// CodecDecodeHook decodes bytes under a codec the core switch does not
	// handle, resolving it through the registry's search functions. It returns
	// the decoded str, or a LookupError if no registered codec claims enc.
	CodecDecodeHook func(v []byte, enc, errh string) (Object, error)

	// CodecTextCheckHook reports whether a codec may be used where a text codec
	// is required, the guard CPython's PyUnicode_AsEncodedString and
	// PyUnicode_Decode apply on str.encode, bytes.decode and the str/bytes
	// constructors. It returns the LookupError to raise when the codec's
	// CodecInfo marks _is_text_encoding false (a bytes-to-bytes or str-to-str
	// transform codec such as base64_codec or rot_13), or nil when the codec is
	// a text codec or unknown. direction is "encode" or "decode" and picks the
	// codecs.encode()/codecs.decode() hint in the message.
	CodecTextCheckHook func(enc, direction string) error
)

// guardTextCodec applies the text-encoding guard for the codec paths that
// require a text codec: str.encode, bytes.decode and the str/bytes
// constructors. A binary transform codec used there raises "<name> is not a
// text encoding" rather than being dispatched. The utf-8, ascii and latin-1
// codecs are handled inline and are always text codecs, so they skip the
// registry round trip; every other name defers to CodecTextCheckHook, which
// reads the CodecInfo _is_text_encoding flag off the registry.
func guardTextCodec(enc, direction string) error {
	switch normalizeCodec(enc) {
	case "utf8", "ascii", "latin1":
		return nil
	}
	if CodecTextCheckHook == nil {
		return nil
	}
	return CodecTextCheckHook(enc, direction)
}
