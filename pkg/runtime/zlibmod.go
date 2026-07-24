package runtime

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"hash/crc32"
	"io"

	"github.com/tamnd/unagi/pkg/objects"
)

// zlib is a pure C module in CPython with no Python wrapper, so the accelerator
// carries the whole surface programs use directly. This slice lands the one-shot
// codec and the checksums: compress, decompress, crc32 and adler32, the module
// constants, and the zlib.error exception. The streaming compressobj and
// decompressobj that gzip.py drives come next.
//
// The compressed bytes a codec emits are implementation-defined (Go's DEFLATE
// and C zlib pick different matches for the same input), so callers rely on the
// round trip and the checksums, both of which are exact: Go decompresses what C
// zlib produced and the CRC-32 and Adler-32 values are algorithm-defined.

// zlibErrorClass is zlib.error, a subclass of Exception the codec raises on bad
// input, built once at module init and reused for every raise.
var zlibErrorClass objects.Object

func init() {
	moduleTable["zlib"] = &moduleEntry{builtin: true, exec: initZlib}
}

func initZlib(m *objects.Module) error {
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "zlib: Exception base is unavailable")
	}
	errCls, err := objects.NewClass("error", "zlib.error", []objects.Object{exc}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	zlibErrorClass = errCls
	objects.SetZlibError(errCls)
	if err := objects.StoreAttr(m, "error", errCls); err != nil {
		return err
	}

	// The version strings track the zlib the compiled world behaves like. They
	// are informational; programs read them but the codec does not branch on them.
	strs := []struct{ name, val string }{
		{"ZLIB_VERSION", "1.2.12"},
		{"ZLIB_RUNTIME_VERSION", "1.2.12"},
	}
	for _, s := range strs {
		if err := objects.StoreAttr(m, s.name, objects.NewStr(s.val)); err != nil {
			return err
		}
	}

	// The constants zlib re-exports: the window and buffer sizes, the compression
	// levels, the compression method, the memory level, and the flush and strategy
	// selectors the streaming codec will accept.
	consts := []struct {
		name string
		val  int64
	}{
		{"MAX_WBITS", 15},
		{"DEFLATED", 8},
		{"DEF_MEM_LEVEL", 8},
		{"DEF_BUF_SIZE", 16384},
		{"Z_NO_COMPRESSION", 0},
		{"Z_BEST_SPEED", 1},
		{"Z_BEST_COMPRESSION", 9},
		{"Z_DEFAULT_COMPRESSION", -1},
		{"Z_DEFAULT_STRATEGY", 0},
		{"Z_FILTERED", 1},
		{"Z_HUFFMAN_ONLY", 2},
		{"Z_RLE", 3},
		{"Z_FIXED", 4},
		{"Z_NO_FLUSH", 0},
		{"Z_PARTIAL_FLUSH", 1},
		{"Z_SYNC_FLUSH", 2},
		{"Z_FULL_FLUSH", 3},
		{"Z_FINISH", 4},
		{"Z_BLOCK", 5},
		{"Z_TREES", 6},
	}
	for _, c := range consts {
		if err := objects.StoreAttr(m, c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}

	fns := []struct {
		name string
		fn   func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error)
	}{
		{"compress", zlibCompress},
		{"decompress", zlibDecompress},
		{"compressobj", zlibCompressObj},
		{"decompressobj", zlibDecompressObj},
		{"_ZlibDecompressor", zlibDecompressor},
		{"crc32", zlibCrc32},
		{"adler32", zlibAdler32},
	}
	for _, f := range fns {
		if err := objects.StoreAttr(m, f.name, objects.NewFuncKw(f.name, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// zlibError raises a zlib.error carrying the message.
func zlibError(msg string) error {
	if zlibErrorClass != nil {
		if inst, err := objects.Call(zlibErrorClass, []objects.Object{objects.NewStr(msg)}); err == nil {
			if e, ok := inst.(error); ok {
				return e
			}
		}
	}
	return objects.Raise(objects.RuntimeError, "%s", msg)
}

// zlibBytesArg reads a bytes-like first argument, the buffer every zlib call
// takes.
func zlibBytesArg(fn string, o objects.Object) ([]byte, error) {
	b, ok := objects.AsBytesLike(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", o.TypeName())
	}
	return b, nil
}

// zlibIntArg reads an integer argument, defaulting when it is absent.
func zlibIntArg(fn, name string, v objects.Object, def int) (int, error) {
	if v == nil {
		return def, nil
	}
	n, ok := objects.AsInt(v)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "%s() argument '%s' must be an integer", fn, name)
	}
	return int(n), nil
}

// zlibCompress is zlib.compress(data, level=-1, wbits=MAX_WBITS): it deflates the
// buffer at the given level, wrapping the stream in the zlib, raw, or gzip
// framing the wbits sign and magnitude select.
func zlibCompress(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("compress", []string{"data", "level", "wbits"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	if vals["data"] == nil {
		return nil, objects.Raise(objects.TypeError, "compress() missing required argument 'data'")
	}
	data, err := zlibBytesArg("compress", vals["data"])
	if err != nil {
		return nil, err
	}
	level, err := zlibIntArg("compress", "level", vals["level"], -1)
	if err != nil {
		return nil, err
	}
	wbits, err := zlibIntArg("compress", "wbits", vals["wbits"], 15)
	if err != nil {
		return nil, err
	}
	if level < -1 || level > 9 {
		return nil, zlibError("Bad compression level")
	}

	var buf bytes.Buffer
	var w io.WriteCloser
	switch {
	case wbits >= 9 && wbits <= 15:
		w, err = zlib.NewWriterLevel(&buf, level)
	case wbits <= -9 && wbits >= -15:
		w, err = flate.NewWriter(&buf, level)
	case wbits >= 25 && wbits <= 31:
		w, err = gzip.NewWriterLevel(&buf, level)
	default:
		return nil, zlibError("Invalid initialization option")
	}
	if err != nil {
		return nil, zlibError(err.Error())
	}
	if _, err := w.Write(data); err != nil {
		return nil, zlibError(err.Error())
	}
	if err := w.Close(); err != nil {
		return nil, zlibError(err.Error())
	}
	return objects.NewBytes(buf.Bytes()), nil
}

// zlibDecompress is zlib.decompress(data, wbits=MAX_WBITS, bufsize=DEF_BUF_SIZE):
// it inflates the buffer, reading the zlib, raw, or gzip framing the wbits select,
// with the high range auto-detecting zlib versus gzip. bufsize only hints the
// output size, which the reader grows on its own, so it is accepted and ignored.
func zlibDecompress(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("decompress", []string{"data", "wbits", "bufsize"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	if vals["data"] == nil {
		return nil, objects.Raise(objects.TypeError, "decompress() missing required argument 'data'")
	}
	data, err := zlibBytesArg("decompress", vals["data"])
	if err != nil {
		return nil, err
	}
	wbits, err := zlibIntArg("decompress", "wbits", vals["wbits"], 15)
	if err != nil {
		return nil, err
	}

	r, err := zlibReader(wbits, data)
	if err != nil {
		return nil, zlibError(err.Error())
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, zlibError(err.Error())
	}
	if err := r.Close(); err != nil {
		return nil, zlibError(err.Error())
	}
	return objects.NewBytes(out), nil
}

// zlibCompressObj is zlib.compressobj(level=-1, method=DEFLATED, wbits=15,
// memLevel=8, strategy=0): it opens a streaming compressor over the framing wbits
// selects. method, memLevel, and strategy are accepted for the CPython signature
// but do not change Go's DEFLATE, which has no tunable memory level or strategy.
func zlibCompressObj(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("compressobj", []string{"level", "method", "wbits", "memLevel", "strategy"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	level, err := zlibIntArg("compressobj", "level", vals["level"], -1)
	if err != nil {
		return nil, err
	}
	wbits, err := zlibIntArg("compressobj", "wbits", vals["wbits"], 15)
	if err != nil {
		return nil, err
	}
	if level < -1 || level > 9 {
		return nil, zlibError("Bad compression level")
	}
	return objects.NewZlibCompress(level, wbits)
}

// zlibDecompressObj is zlib.decompressobj(wbits=15): it opens a streaming
// decompressor reading the framing wbits selects, the object gzip.decompress
// feeds a whole member into to read back the stream, its end, and the trailer.
func zlibDecompressObj(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("decompressobj", []string{"wbits", "zdict"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	wbits, err := zlibIntArg("decompressobj", "wbits", vals["wbits"], 15)
	if err != nil {
		return nil, err
	}
	return objects.NewZlibDecompress(wbits), nil
}

// zlibDecompressor is zlib._ZlibDecompressor(wbits=15, zdict=b”): the private
// class GzipFile reads through compression._common._streams.DecompressReader. It
// exposes decompress(data, max_length), eof, needs_input, and unused_data so the
// reader can walk one gzip member at a time.
func zlibDecompressor(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("_ZlibDecompressor", []string{"wbits", "zdict"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	wbits, err := zlibIntArg("_ZlibDecompressor", "wbits", vals["wbits"], 15)
	if err != nil {
		return nil, err
	}
	return objects.NewZlibDecompressor(wbits), nil
}

// zlibReader picks the inflate reader the wbits value asks for: the 9..15 range
// is zlib framing, the negative range is a raw stream, the 25..31 range is gzip,
// and the 32+ range auto-detects zlib versus gzip from the leading bytes.
func zlibReader(wbits int, data []byte) (io.ReadCloser, error) {
	switch {
	case wbits >= 8 && wbits <= 15, wbits == 0:
		return zlib.NewReader(bytes.NewReader(data))
	case wbits <= -8 && wbits >= -15:
		return flate.NewReader(bytes.NewReader(data)), nil
	case wbits >= 24 && wbits <= 31:
		return gzip.NewReader(bytes.NewReader(data))
	case wbits >= 40 && wbits <= 47:
		if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
			return gzip.NewReader(bytes.NewReader(data))
		}
		return zlib.NewReader(bytes.NewReader(data))
	default:
		return nil, objects.Raise(objects.ValueError, "Invalid initialization option")
	}
}

// zlibCrc32 is zlib.crc32(data, value=0): the running CRC-32 of the buffer over
// the IEEE polynomial zlib uses, returned unsigned so it chains with a later call.
func zlibCrc32(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, value, err := zlibChecksumArgs("crc32", 0, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(crc32.Update(value, crc32.IEEETable, data))), nil
}

// zlibAdler32 is zlib.adler32(data, value=1): the running Adler-32 checksum,
// seeded at 1 the way zlib defines it, returned unsigned to chain.
func zlibAdler32(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, value, err := zlibChecksumArgs("adler32", 1, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	const mod = 65521
	s1 := value & 0xffff
	s2 := (value >> 16) & 0xffff
	for _, b := range data {
		s1 = (s1 + uint32(b)) % mod
		s2 = (s2 + s1) % mod
	}
	return objects.NewInt(int64(s2<<16 | s1)), nil
}

// zlibChecksumArgs reads the shared (data, value) signature of crc32 and adler32,
// applying the checksum's own default seed when value is absent.
func zlibChecksumArgs(fn string, seed uint32, pos []objects.Object, kwNames []string, kwVals []objects.Object) ([]byte, uint32, error) {
	vals, err := bindArgs(fn, []string{"data", "value"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, 0, err
	}
	if vals["data"] == nil {
		return nil, 0, objects.Raise(objects.TypeError, "%s() missing required argument 'data'", fn)
	}
	data, err := zlibBytesArg(fn, vals["data"])
	if err != nil {
		return nil, 0, err
	}
	value := seed
	if vals["value"] != nil {
		n, ok := objects.AsInt(vals["value"])
		if !ok {
			return nil, 0, objects.Raise(objects.TypeError, "%s() argument 'value' must be an integer", fn)
		}
		value = uint32(n)
	}
	return data, value, nil
}
