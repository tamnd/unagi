package runtime

import (
	"strings"
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs is a built-in module, the C accelerator behind the pure-Python codecs
// module. codecs.py opens with `from _codecs import *` and turns an ImportError
// into a fatal SystemError, so the accelerator has to exist before codecs
// imports at all; the pure fallback the other modules ship does not exist here.
//
// This slice provides the registry (register/unregister/lookup with a
// normalized-name cache), the stateless encode/decode entry points, the error
// handler registry (register_error/lookup_error), and the per-codec functions
// for the utf-8, ascii and latin-1 families that codecs.py and the encodings
// package name directly.
//
// The codec functions apply their handler in Go through objects.EncodeStr and
// objects.DecodeBytes, which own the utf-8/ascii/latin-1 error wording the
// bytes and str paths already use, so encode and decode read through to that
// one place. encode and decode take the same fast path for those families
// rather than routing every call through a registered search function, so the
// core codecs work before the encodings search function is registered; the
// registry still drives lookup for any codec a search function provides.
//
// The error handler objects are API surface: codecs.py binds strict_errors and
// the rest at import from lookup_error, and register_error/lookup_error round
// trip a custom handler. Only strict is invocable in this tier; the others
// raise NotImplementedError if called, because reporting a replacement needs
// the UnicodeError object to expose object/start/end, a later slice.

// codecRegistry holds the process-global codec state the C module keeps in the
// interpreter: the registered search functions, the normalized-name lookup
// cache, and the error handler table. The mutex guards every field because a
// threaded program can register or look up a codec from any goroutine.
var codecRegistry = struct {
	mu       sync.Mutex
	search   []objects.Object
	cache    map[string]objects.Object
	errors   map[string]objects.Object
	seeded   bool
	encTried bool
}{
	cache:  map[string]objects.Object{},
	errors: map[string]objects.Object{},
}

func init() {
	moduleTable["_codecs"] = &moduleEntry{builtin: true, exec: initCodecs}
	// Let the objects package's built-in codec paths (str.encode, bytes.decode,
	// the bytes/str constructors) fall through to the registry for any codec
	// beyond utf-8/ascii/latin-1, so they resolve the encodings package's codecs
	// even before a program imports codecs itself. The hooks read only the
	// process-global registry, which imports encodings lazily on first lookup.
	objects.CodecEncodeHook = codecEncodeHook
	objects.CodecDecodeHook = codecDecodeHook
}

// codecsExports lists every name `from _codecs import *` binds. It mirrors the
// BuiltinStarExports["_codecs"] entry in pkg/lower/lower.go, the list that
// binds the star surface at compile time; keep the two in step.
var codecsExports = []string{
	"register", "unregister", "lookup", "encode", "decode",
	"lookup_error", "register_error",
	"utf_8_encode", "utf_8_decode",
	"ascii_encode", "ascii_decode",
	"latin_1_encode", "latin_1_decode",
	"charmap_encode", "charmap_decode", "charmap_build",
	"utf_16_encode", "utf_16_le_encode", "utf_16_be_encode",
	"utf_16_decode", "utf_16_le_decode", "utf_16_be_decode", "utf_16_ex_decode",
	"utf_32_encode", "utf_32_le_encode", "utf_32_be_encode",
	"utf_32_decode", "utf_32_le_decode", "utf_32_be_decode", "utf_32_ex_decode",
	"unicode_escape_encode", "unicode_escape_decode",
	"raw_unicode_escape_encode", "raw_unicode_escape_decode",
}

// stdErrorNames are the error handlers the C module preregisters. strict is the
// only one that runs in this tier; the rest are placeholders bound so codecs.py
// can hand them back from lookup_error.
var stdErrorNames = []string{
	"strict", "ignore", "replace", "xmlcharrefreplace",
	"backslashreplace", "namereplace", "surrogatepass", "surrogateescape",
}

func initCodecs(m *objects.Module) error {
	codecRegistry.mu.Lock()
	if !codecRegistry.seeded {
		for _, name := range stdErrorNames {
			if name == "strict" {
				codecRegistry.errors[name] = objects.NewFunc("strict_errors", 1, codecStrictHandler)
				continue
			}
			codecRegistry.errors[name] = objects.NewFunc(name+"_errors", 1, codecPlaceholderHandler(name))
		}
		codecRegistry.seeded = true
	}
	codecRegistry.mu.Unlock()

	attrs := map[string]objects.Object{
		"register":       objects.NewFunc("register", 1, codecRegister),
		"unregister":     objects.NewFunc("unregister", 1, codecUnregister),
		"lookup":         objects.NewFunc("lookup", 1, codecLookup),
		"encode":         objects.NewFuncKw("encode", codecEncode),
		"decode":         objects.NewFuncKw("decode", codecDecode),
		"lookup_error":   objects.NewFunc("lookup_error", 1, codecLookupError),
		"register_error": objects.NewFunc("register_error", 2, codecRegisterError),
		"utf_8_encode":   objects.NewFuncKw("utf_8_encode", codecEncoder("utf-8")),
		"utf_8_decode":   objects.NewFuncKw("utf_8_decode", codecDecoder("utf-8")),
		"ascii_encode":   objects.NewFuncKw("ascii_encode", codecEncoder("ascii")),
		"ascii_decode":   objects.NewFuncKw("ascii_decode", codecDecoder("ascii")),
		"latin_1_encode": objects.NewFuncKw("latin_1_encode", codecEncoder("latin-1")),
		"latin_1_decode": objects.NewFuncKw("latin_1_decode", codecDecoder("latin-1")),
		"charmap_encode": objects.NewFuncKw("charmap_encode", codecCharmapEncode),
		"charmap_decode": objects.NewFuncKw("charmap_decode", codecCharmapDecode),
		"charmap_build":  objects.NewFunc("charmap_build", 1, codecCharmapBuild),

		"utf_16_encode":    objects.NewFuncKw("utf_16_encode", codecUTF16Encode),
		"utf_16_le_encode": objects.NewFuncKw("utf_16_le_encode", codecUTF16LEEncode),
		"utf_16_be_encode": objects.NewFuncKw("utf_16_be_encode", codecUTF16BEEncode),
		"utf_16_decode":    objects.NewFuncKw("utf_16_decode", codecUTF16Decode),
		"utf_16_le_decode": objects.NewFuncKw("utf_16_le_decode", codecUTF16LEDecode),
		"utf_16_be_decode": objects.NewFuncKw("utf_16_be_decode", codecUTF16BEDecode),
		"utf_16_ex_decode": objects.NewFuncKw("utf_16_ex_decode", codecUTF16ExDecode),
		"utf_32_encode":    objects.NewFuncKw("utf_32_encode", codecUTF32Encode),
		"utf_32_le_encode": objects.NewFuncKw("utf_32_le_encode", codecUTF32LEEncode),
		"utf_32_be_encode": objects.NewFuncKw("utf_32_be_encode", codecUTF32BEEncode),
		"utf_32_decode":    objects.NewFuncKw("utf_32_decode", codecUTF32Decode),
		"utf_32_le_decode": objects.NewFuncKw("utf_32_le_decode", codecUTF32LEDecode),
		"utf_32_be_decode": objects.NewFuncKw("utf_32_be_decode", codecUTF32BEDecode),
		"utf_32_ex_decode": objects.NewFuncKw("utf_32_ex_decode", codecUTF32ExDecode),

		"unicode_escape_encode":     objects.NewFuncKw("unicode_escape_encode", codecUnicodeEscapeEncode),
		"unicode_escape_decode":     objects.NewFuncKw("unicode_escape_decode", codecUnicodeEscapeDecode),
		"raw_unicode_escape_encode": objects.NewFuncKw("raw_unicode_escape_encode", codecRawUnicodeEscapeEncode),
		"raw_unicode_escape_decode": objects.NewFuncKw("raw_unicode_escape_decode", codecRawUnicodeEscapeDecode),
	}
	for _, name := range codecsExports {
		if err := objects.StoreAttr(m, name, attrs[name]); err != nil {
			return err
		}
	}
	return nil
}

// normalizeCodecName folds an encoding name the way the C registry does before
// a lookup: lowercased with spaces turned to underscores, so "UTF 8" and
// "utf_8" cache under the same key.
func normalizeCodecName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "_")
}

// codecRegister implements _codecs.register(search_function): append the search
// function to the registry and drop the cache so a name a new function claims is
// looked up fresh.
func codecRegister(args []objects.Object) (objects.Object, error) {
	fn := args[0]
	codecRegistry.mu.Lock()
	defer codecRegistry.mu.Unlock()
	codecRegistry.search = append(codecRegistry.search, fn)
	clear(codecRegistry.cache)
	return objects.None, nil
}

// codecUnregister implements _codecs.unregister(search_function): remove the
// search function by identity and drop the cache.
func codecUnregister(args []objects.Object) (objects.Object, error) {
	fn := args[0]
	codecRegistry.mu.Lock()
	defer codecRegistry.mu.Unlock()
	for i, s := range codecRegistry.search {
		if s == fn {
			codecRegistry.search = append(codecRegistry.search[:i], codecRegistry.search[i+1:]...)
			break
		}
	}
	clear(codecRegistry.cache)
	return objects.None, nil
}

// codecLookup implements _codecs.lookup(encoding): normalize the name, answer
// from the cache, then consult each search function in registration order until
// one returns a CodecInfo. An unknown encoding raises LookupError with the
// registry's wording.
func codecLookup(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "lookup() argument must be str, not %s", args[0].TypeName())
	}
	key := normalizeCodecName(name)

	if err := ensureEncodings(); err != nil {
		return nil, err
	}

	codecRegistry.mu.Lock()
	if info, hit := codecRegistry.cache[key]; hit {
		codecRegistry.mu.Unlock()
		return info, nil
	}
	search := append([]objects.Object(nil), codecRegistry.search...)
	codecRegistry.mu.Unlock()

	// The search functions run without the lock so one that imports a module or
	// calls back into the registry does not deadlock.
	for _, fn := range search {
		res, err := objects.Call(fn, []objects.Object{objects.NewStr(key)})
		if err != nil {
			return nil, err
		}
		if res == objects.None {
			continue
		}
		codecRegistry.mu.Lock()
		codecRegistry.cache[key] = res
		codecRegistry.mu.Unlock()
		return res, nil
	}
	return nil, objects.Raise("LookupError", "unknown encoding: %s", name)
}

// ensureEncodings imports the encodings package the first time a lookup needs a
// search function, the cold path CPython runs when the codec registry is still
// empty: the C _codecs.lookup imports encodings, whose __init__ registers a
// search function that resolves every codec by importing encodings.<name>. The
// import runs without the registry lock held because encodings.__init__ calls
// back into register. It is attempted once; if encodings is not part of this
// program (a build that reaches no non-core codec never compiles it in), the
// ImportError is swallowed so the lookup that triggered it raises the ordinary
// unknown-encoding LookupError rather than surfacing the missing package. Any
// other failure — a genuine error in encodings.__init__ — propagates.
func ensureEncodings() error {
	codecRegistry.mu.Lock()
	if codecRegistry.encTried || len(codecRegistry.search) > 0 {
		codecRegistry.mu.Unlock()
		return nil
	}
	codecRegistry.encTried = true
	codecRegistry.mu.Unlock()

	if _, err := ImportModule("encodings"); err != nil {
		if exc, ok := err.(*objects.Exception); ok && objects.Matches(exc.Kind, objects.ImportError) {
			return nil
		}
		return err
	}
	return nil
}

// codecEncode implements _codecs.encode(obj, encoding='utf-8', errors='strict'):
// the stateless encode entry codecs.encode is bound to. It hands back the
// encoded bytes, not the (bytes, length) pair the per-codec functions return.
func codecEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	obj, enc, errs, err := codecApplyArgs("encode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	// A str takes the core encoder fast path. Anything else (bytes for the
	// bytes-to-bytes transform codecs like base64_codec and zlib_codec) is
	// dispatched straight through the registry the way CPython's PyCodec_Encode
	// is, returning whatever object the codec hands back rather than forcing bytes.
	s, ok := objects.AsStr(obj)
	if !ok {
		return codecViaRegistry("encode", obj, enc, errs)
	}
	b, err := objects.EncodeStr(s, enc, errs)
	if err != nil {
		// A str-to-str codec such as rot_13 hands back a str, which the
		// bytes-forcing core path rejects. codecs.encode, unlike str.encode,
		// returns whatever the codec produces, so fall back to the registry and
		// return its object when it succeeds; otherwise keep the original error.
		if res, rerr := codecViaRegistry("encode", obj, enc, errs); rerr == nil {
			return res, nil
		}
		return nil, err
	}
	return objects.NewBytes(b), nil
}

// codecDecode implements _codecs.decode(obj, encoding='utf-8', errors='strict'):
// the stateless decode entry codecs.decode is bound to. It hands back the
// decoded str, not the (str, length) pair the per-codec functions return.
func codecDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	obj, enc, errs, err := codecApplyArgs("decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	v, ok := objects.AsBytesLike(obj)
	if !ok {
		// A str-to-str codec such as rot_13 decodes a str. CPython lets the codec
		// itself decide whether it accepts a str rather than pre-rejecting here,
		// so dispatch through the registry and let it raise if the codec does not.
		if _, isStr := objects.AsStr(obj); isStr {
			return codecViaRegistry("decode", obj, enc, errs)
		}
		return nil, objects.Raise(objects.TypeError, "decode() argument 'obj' must be bytes-like, not %s", obj.TypeName())
	}
	return objects.DecodeBytes(v, enc, errs)
}

// codecViaRegistry runs one stateless codec call for a name the objects
// package's core switch does not handle. It resolves the codec through the
// registry the way CPython's PyCodec_Encode/Decode do, calls the CodecInfo's
// encode or decode with (obj, errors), and returns just the result from the
// (result, length) pair the codec hands back. who is "encode" or "decode". It
// backs the objects.CodecEncodeHook/CodecDecodeHook so str.encode, bytes.decode
// and codecs.encode/decode all reach the encodings package's codecs.
func codecViaRegistry(who string, obj objects.Object, enc, errs string) (objects.Object, error) {
	info, err := codecLookup([]objects.Object{objects.NewStr(enc)})
	if err != nil {
		return nil, err
	}
	fn, err := objects.LoadAttr(info, who)
	if err != nil {
		return nil, err
	}
	res, err := objects.Call(fn, []objects.Object{obj, objects.NewStr(errs)})
	if err != nil {
		return nil, err
	}
	// CPython requires the codec to return a (result, length) tuple and hands
	// back only the result; a codec that returns something else is a bug.
	out, err := objects.GetItem(res, objects.NewInt(0))
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "%s() codec must return a tuple (object, integer)", who)
	}
	return out, nil
}

// codecEncodeHook backs objects.CodecEncodeHook: encode a str through the codec
// registry and hand back the raw bytes the core encoder path expects.
func codecEncodeHook(s, enc, errh string) ([]byte, error) {
	out, err := codecViaRegistry("encode", objects.NewStr(s), enc, errh)
	if err != nil {
		return nil, err
	}
	b, ok := objects.AsBytesLike(out)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' encoder returned '%s' instead of 'bytes'", enc, out.TypeName())
	}
	return b, nil
}

// codecDecodeHook backs objects.CodecDecodeHook: decode bytes through the codec
// registry and hand back the str the core decoder path expects.
func codecDecodeHook(v []byte, enc, errh string) (objects.Object, error) {
	return codecViaRegistry("decode", objects.NewBytes(v), enc, errh)
}

// codecApplyArgs reads the shared (obj, encoding='utf-8', errors='strict')
// signature of encode and decode, threading the positional and keyword forms.
func codecApplyArgs(who string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (obj objects.Object, enc, errs string, err error) {
	enc, errs = "utf-8", "strict"
	if len(pos) < 1 {
		return nil, "", "", objects.Raise(objects.TypeError, "%s() missing required argument 'obj'", who)
	}
	obj = pos[0]
	if len(pos) >= 2 {
		if enc, err = codecStrArg(who, "encoding", pos[1]); err != nil {
			return nil, "", "", err
		}
	}
	if len(pos) >= 3 {
		if errs, err = codecStrArg(who, "errors", pos[2]); err != nil {
			return nil, "", "", err
		}
	}
	if len(pos) > 3 {
		return nil, "", "", objects.Raise(objects.TypeError, "%s() takes at most 3 arguments (%d given)", who, len(pos))
	}
	for i, kn := range kwNames {
		switch kn {
		case "encoding":
			if enc, err = codecStrArg(who, "encoding", kwVals[i]); err != nil {
				return nil, "", "", err
			}
		case "errors":
			if errs, err = codecStrArg(who, "errors", kwVals[i]); err != nil {
				return nil, "", "", err
			}
		default:
			return nil, "", "", objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for %s()", kn, who)
		}
	}
	return obj, enc, errs, nil
}

// codecStrArg reads a str argument, raising the TypeError CPython raises when
// the encoding or errors argument is not a string.
func codecStrArg(who, arg string, o objects.Object) (string, error) {
	s, ok := objects.AsStr(o)
	if !ok {
		return "", objects.Raise(objects.TypeError, "%s() argument '%s' must be str, not %s", who, arg, o.TypeName())
	}
	return s, nil
}

// codecEncoder builds a per-codec encode function such as utf_8_encode. It reads
// (str, errors='strict') and returns (bytes, length), where length is the count
// of input code points the way the C encoder reports it.
func codecEncoder(enc string) func([]objects.Object, []string, []objects.Object) (objects.Object, error) {
	return func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "%s_encode() missing required argument", strings.ReplaceAll(enc, "-", "_"))
		}
		s, ok := objects.AsStr(pos[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "argument must be str, not %s", pos[0].TypeName())
		}
		errs := "strict"
		if len(pos) >= 2 && pos[1] != objects.None {
			e, ok := objects.AsStr(pos[1])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "errors must be str, not %s", pos[1].TypeName())
			}
			errs = e
		}
		b, err := objects.EncodeStr(s, enc, errs)
		if err != nil {
			return nil, err
		}
		return objects.NewTuple([]objects.Object{objects.NewBytes(b), objects.NewInt(int64(len([]rune(s))))}), nil
	}
}

// codecDecoder builds a per-codec decode function such as utf_8_decode. It reads
// (bytes, errors='strict', final=False) and returns (str, length), where length
// is the count of input bytes consumed.
func codecDecoder(enc string) func([]objects.Object, []string, []objects.Object) (objects.Object, error) {
	return func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "%s_decode() missing required argument", strings.ReplaceAll(enc, "-", "_"))
		}
		v, ok := objects.AsBytesLike(pos[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "argument must be bytes-like, not %s", pos[0].TypeName())
		}
		errs := "strict"
		if len(pos) >= 2 {
			var err error
			if errs, err = codecStrArg("decode", "errors", pos[1]); err != nil {
				return nil, err
			}
		}
		s, err := objects.DecodeBytes(v, enc, errs)
		if err != nil {
			return nil, err
		}
		return objects.NewTuple([]objects.Object{s, objects.NewInt(int64(len(v)))}), nil
	}
}

// codecLookupError implements _codecs.lookup_error(name): hand back the handler
// registered under name, raising LookupError with CPython's wording when none
// is registered.
func codecLookupError(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "lookup_error() argument must be str, not %s", args[0].TypeName())
	}
	codecRegistry.mu.Lock()
	defer codecRegistry.mu.Unlock()
	if h, found := codecRegistry.errors[name]; found {
		return h, nil
	}
	return nil, objects.Raise("LookupError", "unknown error handler name '%s'", name)
}

// codecRegisterError implements _codecs.register_error(name, handler): record a
// custom error handler under name for lookup_error to return.
func codecRegisterError(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "register_error() argument 'name' must be str, not %s", args[0].TypeName())
	}
	codecRegistry.mu.Lock()
	defer codecRegistry.mu.Unlock()
	codecRegistry.errors[name] = args[1]
	return objects.None, nil
}

// codecStrictHandler is the strict error handler: it re-raises the UnicodeError
// it is handed, the behavior codecs.strict_errors documents.
func codecStrictHandler(args []objects.Object) (objects.Object, error) {
	if len(args) == 1 {
		if e, ok := args[0].(error); ok {
			return nil, e
		}
	}
	return nil, objects.Raise(objects.TypeError, "codec must pass exception instance")
}

// codecPlaceholderHandler builds a non-strict error handler stub. The handler
// object is bound so codecs.py can hand it back from lookup_error, but calling
// it raises NotImplementedError until the UnicodeError object exposes the
// object/start/end a replacement needs.
func codecPlaceholderHandler(name string) func([]objects.Object) (objects.Object, error) {
	return func([]objects.Object) (objects.Object, error) {
		return nil, objects.Raise("NotImplementedError", "the %q error handler is not available in this build", name)
	}
}
