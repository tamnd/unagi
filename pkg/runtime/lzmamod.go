package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _lzma is the C accelerator lzma.py opens with `from _lzma import *` and
// `from _lzma import _encode_filter_properties, _decode_filter_properties`. The
// module was missing, so `import lzma` failed with ModuleNotFoundError, taking
// the xz paths of tarfile and shutil with it.
//
// _lzma wraps liblzma (the xz library). The Go standard library carries no xz
// codec at all, neither compressor nor decompressor, so unlike _bz2 (where Go's
// compress/bzip2 backs a real decompressor) there is no half to implement
// honestly. What the module does carry that is real and usable is its constant
// block: the check ids, filter ids, container formats, match finders, modes, and
// presets, which callers read to build filter chains and inspect archives.
//
// lzma.py reads only those names at import; it constructs LZMACompressor and
// LZMADecompressor lazily inside its file and one-shot helpers, never at module
// scope. So the constants plus present-but-raising constructors let `import lzma`
// succeed while any actual xz work fails cleanly at the call with a message
// pointing at the missing backend, rather than fabricating bytes. This is the
// same reduced-surface stance _ast, marshal, and _symtable (#788) take, and the
// same honest gap as the _bz2 compressor (#785).
//
// The constant values are CPython 3.14.6's, fixed by liblzma's headers, so they
// are platform-stable by construction.

func init() {
	moduleTable["_lzma"] = &moduleEntry{builtin: true, exec: initLzma}
}

// lzmaUnavailable is the error every xz operation raises: there is no xz backend
// in this build, so the codec cannot run rather than return wrong output.
func lzmaUnavailable() error {
	return objects.Raise("NotImplementedError",
		"xz/lzma compression is not available: this build carries no xz backend (Go's standard library has no lzma codec)")
}

func initLzma(m *objects.Module) error {
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_lzma: Exception base is unavailable")
	}
	// LZMAError is the exception liblzma errors surface as; it is a real, catchable
	// Exception subclass so `except lzma.LZMAError` binds even though the codec
	// itself is unavailable.
	errCls, err := objects.NewClass("LZMAError", "_lzma.LZMAError", []objects.Object{exc}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "LZMAError", errCls); err != nil {
		return err
	}

	// The constant block liblzma exports: check ids, filter ids, container
	// formats, match finders, encoder modes, and presets. These are real and
	// usable regardless of the missing codec.
	consts := []struct {
		name string
		val  int64
	}{
		{"CHECK_NONE", 0},
		{"CHECK_CRC32", 1},
		{"CHECK_CRC64", 4},
		{"CHECK_SHA256", 10},
		{"CHECK_ID_MAX", 15},
		{"CHECK_UNKNOWN", 16},
		{"FILTER_LZMA1", 4611686018427387905},
		{"FILTER_LZMA2", 33},
		{"FILTER_DELTA", 3},
		{"FILTER_X86", 4},
		{"FILTER_IA64", 6},
		{"FILTER_ARM", 7},
		{"FILTER_ARMTHUMB", 8},
		{"FILTER_POWERPC", 5},
		{"FILTER_SPARC", 9},
		{"FORMAT_AUTO", 0},
		{"FORMAT_XZ", 1},
		{"FORMAT_ALONE", 2},
		{"FORMAT_RAW", 3},
		{"MF_HC3", 3},
		{"MF_HC4", 4},
		{"MF_BT2", 18},
		{"MF_BT3", 19},
		{"MF_BT4", 20},
		{"MODE_FAST", 1},
		{"MODE_NORMAL", 2},
		{"PRESET_DEFAULT", 6},
		{"PRESET_EXTREME", 2147483648},
	}
	for _, c := range consts {
		if err := objects.StoreAttr(m, c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}

	// LZMACompressor(format=FORMAT_XZ, check=-1, preset=None, filters=None) and
	// LZMADecompressor(format=FORMAT_AUTO, memlimit=None, filters=None) drive the
	// xz codec, which this build does not carry, so they raise at construction
	// rather than returning an object that can do nothing. lzma.py never builds
	// one at import, so the import still succeeds.
	names := []string{"LZMACompressor", "LZMADecompressor", "_encode_filter_properties", "_decode_filter_properties", "is_check_supported"}
	for _, name := range names {
		fn := objects.NewFuncKw(name, func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
			return nil, lzmaUnavailable()
		})
		if err := objects.StoreAttr(m, name, fn); err != nil {
			return err
		}
	}
	return nil
}
