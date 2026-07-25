package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _bz2 is a pure C module in CPython that bz2.py imports BZ2Compressor and
// BZ2Decompressor from. Go's standard library carries a bzip2 decompressor
// (compress/bzip2) but no compressor, so this native accelerator lands the
// decompressor for real and an honest compressor that constructs but raises on
// use, which is enough for `import bz2` to succeed and for bz2.decompress and
// reading a .bz2 file to work end to end. The streaming objects themselves live
// in pkg/objects/bz2codec.go so they bind through the same LoadAttr and
// CallMethod paths as the other native codecs.

func init() {
	moduleTable["_bz2"] = &moduleEntry{builtin: true, exec: initBZ2}
}

func initBZ2(m *objects.Module) error {
	fns := []struct {
		name string
		fn   func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error)
	}{
		{"BZ2Compressor", bz2NewCompressor},
		{"BZ2Decompressor", bz2NewDecompressor},
	}
	for _, f := range fns {
		if err := objects.StoreAttr(m, f.name, objects.NewFuncKw(f.name, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// bz2NewCompressor is _bz2.BZ2Compressor(compresslevel=9): CPython lets the
// object be constructed and only fails when it is fed data, so this validates the
// level for the signature and hands back the honest compressor whose compress
// raises. compresslevel must be between 1 and 9.
func bz2NewCompressor(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("BZ2Compressor", []string{"compresslevel"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	level := 9
	if vals["compresslevel"] != nil {
		n, ok := objects.AsInt(vals["compresslevel"])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", vals["compresslevel"].TypeName())
		}
		level = int(n)
	}
	if level < 1 || level > 9 {
		return nil, objects.Raise(objects.ValueError, "compresslevel must be between 1 and 9")
	}
	return objects.NewBZ2Compressor(), nil
}

// bz2NewDecompressor is _bz2.BZ2Decompressor(trailing_error=()): it opens a
// streaming decompressor. trailing_error names the exception class the C module
// treats as a benign trailing-garbage signal; Go's reader handles concatenated
// streams itself, so the argument is accepted for the signature and ignored.
func bz2NewDecompressor(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if _, err := bindArgs("BZ2Decompressor", []string{"trailing_error"}, pos, kwNames, kwVals); err != nil {
		return nil, err
	}
	return objects.NewBZ2Decompressor(), nil
}
