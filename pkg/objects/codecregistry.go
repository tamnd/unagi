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
)
