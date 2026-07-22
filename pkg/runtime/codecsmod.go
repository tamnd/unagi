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
// back into register. It is attempted once; a failure leaves the registry empty
// so the lookup that triggered it raises the ordinary unknown-encoding error.
func ensureEncodings() error {
	codecRegistry.mu.Lock()
	if codecRegistry.encTried || len(codecRegistry.search) > 0 {
		codecRegistry.mu.Unlock()
		return nil
	}
	codecRegistry.encTried = true
	codecRegistry.mu.Unlock()

	if _, err := ImportModule("encodings"); err != nil {
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
	s, ok := objects.AsStr(obj)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "encode() argument 'obj' must be str, not %s", obj.TypeName())
	}
	b, err := objects.EncodeStr(s, enc, errs)
	if err != nil {
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
		return nil, objects.Raise(objects.TypeError, "decode() argument 'obj' must be bytes-like, not %s", obj.TypeName())
	}
	return objects.DecodeBytes(v, enc, errs)
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
