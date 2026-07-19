package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// codecsAttr fetches a name from the _codecs module the way compiled code
// reaches it through `from _codecs import *`.
func codecsAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("_codecs")
	if err != nil {
		t.Fatalf("import _codecs: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_codecs.%s: %v", name, err)
	}
	return fn
}

// callCodecs calls a _codecs function by name with positional arguments.
func callCodecs(t *testing.T, name string, args ...objects.Object) objects.Object {
	t.Helper()
	v, err := objects.Call(codecsAttr(t, name), args)
	if err != nil {
		t.Fatalf("_codecs.%s(...): %v", name, err)
	}
	return v
}

func TestCodecsEncodeDecode(t *testing.T) {
	// encode hands back the encoded bytes, decode the decoded str, both under
	// the default utf-8 and a named codec.
	b, ok := objects.AsBytes(callCodecs(t, "encode", objects.NewStr("hé")))
	if !ok || string(b) != "h\xc3\xa9" {
		t.Fatalf("encode default = %q, %v", b, ok)
	}
	s, ok := objects.AsStr(callCodecs(t, "decode", objects.NewBytes([]byte("h\xc3\xa9"))))
	if !ok || s != "hé" {
		t.Fatalf("decode default = %q, %v", s, ok)
	}
	b, _ = objects.AsBytes(callCodecs(t, "encode", objects.NewStr("hi"), objects.NewStr("ascii")))
	if string(b) != "hi" {
		t.Fatalf("encode ascii = %q", b)
	}
	// An unencodable character under a narrow codec raises, and an unknown codec
	// raises LookupError.
	if _, err := objects.Call(codecsAttr(t, "encode"),
		[]objects.Object{objects.NewStr("ÿ"), objects.NewStr("ascii")}); err == nil {
		t.Fatal("encode of non-ascii under ascii did not raise")
	}
	if _, err := objects.Call(codecsAttr(t, "decode"),
		[]objects.Object{objects.NewBytes([]byte("x")), objects.NewStr("no-such")}); err == nil {
		t.Fatal("decode under unknown codec did not raise")
	}
}

func TestCodecsPerCodecFunctions(t *testing.T) {
	// utf_8_encode returns (bytes, input-length); utf_8_decode returns
	// (str, consumed-bytes).
	enc := callCodecs(t, "utf_8_encode", objects.NewStr("aé"))
	if got := objects.Repr(enc); got != "(b'a\\xc3\\xa9', 2)" {
		t.Fatalf("utf_8_encode = %s", got)
	}
	dec := callCodecs(t, "utf_8_decode", objects.NewBytes([]byte("a\xc3\xa9")))
	if got := objects.Repr(dec); got != "('aé', 3)" {
		t.Fatalf("utf_8_decode = %s", got)
	}
	if got := objects.Repr(callCodecs(t, "latin_1_encode", objects.NewStr("ÿ"))); got != "(b'\\xff', 1)" {
		t.Fatalf("latin_1_encode = %s", got)
	}
}

func TestCodecsErrorHandlers(t *testing.T) {
	// The standard strict handler re-raises the exception it is handed; the
	// placeholder handlers are named but raise NotImplementedError when run.
	strict := codecsAttr(t, "lookup_error")
	h, err := objects.Call(strict, []objects.Object{objects.NewStr("strict")})
	if err != nil {
		t.Fatalf("lookup_error('strict'): %v", err)
	}
	if _, err := objects.Call(h, []objects.Object{objects.Raise("ValueError", "boom")}); err == nil {
		t.Fatal("strict handler did not re-raise")
	}
	// A custom handler round-trips through register_error and lookup_error.
	marker := objects.NewFunc("marker", 1, func(a []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	callCodecs(t, "register_error", objects.NewStr("unagi_marker"), marker)
	if got := callCodecs(t, "lookup_error", objects.NewStr("unagi_marker")); got != marker {
		t.Fatal("register_error/lookup_error did not round-trip the handler")
	}
	// An unknown handler name raises LookupError.
	if _, err := objects.Call(codecsAttr(t, "lookup_error"),
		[]objects.Object{objects.NewStr("no-such-handler")}); err == nil {
		t.Fatal("lookup_error of unknown name did not raise")
	}
}

func TestCodecsRegisterLookupUnregister(t *testing.T) {
	// A registered search function drives lookup; a hit is cached and the codec
	// is found by its normalized name. Registering here keeps the registry
	// non-empty, so lookup never falls to the encodings cold path the unit
	// binary has no floor for.
	info := objects.NewStr("sentinel-codec-info")
	var seen string
	search := objects.NewFunc("search", 1, func(a []objects.Object) (objects.Object, error) {
		seen, _ = objects.AsStr(a[0])
		if seen == "unagi_test_codec" {
			return info, nil
		}
		return objects.None, nil
	})
	callCodecs(t, "register", search)
	defer func() { callCodecs(t, "unregister", search) }()

	// The name is normalized to lowercase-with-underscores before the search.
	if got := callCodecs(t, "lookup", objects.NewStr("Unagi Test Codec")); got != info {
		t.Fatalf("lookup did not return the registered codec, got %v", got)
	}
	if seen != "unagi_test_codec" {
		t.Fatalf("search saw %q, want normalized name", seen)
	}
}

func TestDunderImportBuiltinModule(t *testing.T) {
	// __import__ imports a module by name and hands back the top package when
	// there is no fromlist. A builtin module resolves in the unit binary.
	m, err := dunderImport([]objects.Object{objects.NewStr("sys")}, nil, nil)
	if err != nil {
		t.Fatalf("__import__('sys'): %v", err)
	}
	if _, err := objects.LoadAttr(m, "maxsize"); err != nil {
		t.Fatalf("imported sys missing maxsize: %v", err)
	}
	// A missing module raises, and a non-zero level is rejected.
	if _, err := dunderImport([]objects.Object{objects.NewStr("no_such_module_xyz")}, nil, nil); err == nil {
		t.Fatal("__import__ of missing module did not raise")
	}
	if _, err := dunderImport([]objects.Object{objects.NewStr("sys")},
		[]string{"level"}, []objects.Object{objects.NewInt(1)}); err == nil {
		t.Fatal("__import__ with level=1 did not raise")
	}
}
