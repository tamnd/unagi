package objects

import (
	"bytes"
	"compress/bzip2"
	"io"
)

// The _bz2 module's streaming codec objects live here next to the other native
// types so their methods bind through the same LoadAttr and CallMethod paths;
// pkg/runtime/bz2mod.go builds them from BZ2Compressor and BZ2Decompressor.
//
// Go's standard library carries a bzip2 decompressor (compress/bzip2) but no
// compressor, so this slice is honest about the split: the Decompressor is real
// and exact, and the Compressor exists as an object you can construct (CPython
// lets you) but its compress raises NotImplementedError, since there is no
// stdlib-only way to produce a bzip2 stream. This is what lets `import bz2`
// succeed and bz2.decompress and reading a .bz2 file work end to end, while the
// unavailable write half fails at the exact operation with a clear message.

// bz2DecompressObject is a _bz2.BZ2Decompressor. It accumulates the fed input and
// inflates the whole of it on each call, handing back the newly produced suffix,
// the same design the zlib Decompress object uses so needs_input, eof, and
// unused_data stay exact. Go's bzip2 reader decodes concatenated streams
// transparently, so one full-input decode yields every member at once and leaves
// no unused tail, which collapses bz2.decompress's stream loop to a single pass
// with byte-identical output.
type bz2DecompressObject struct {
	input     []byte
	out       []byte
	returned  int
	streamEnd bool
	eof       bool
	needsIn   bool
	unused    []byte
}

func (d *bz2DecompressObject) TypeName() string { return "_bz2.BZ2Decompressor" }

// NewBZ2Decompressor builds a BZ2Decompressor. It takes no arguments; the
// trailing_error keyword _compression.DecompressReader passes is accepted and
// ignored by the runtime module before it reaches here.
func NewBZ2Decompressor() Object {
	return &bz2DecompressObject{needsIn: true}
}

// bz2Inflate decompresses the whole buffer, returning the output, the trailing
// bytes left after the stream, and whether the stream reached its end. A
// truncated stream is not an error: it reports eof false and whatever output the
// reader produced before running short, so a later call with more input picks up
// where it left off. Corrupt data is the OSError the caller raises.
func bz2Inflate(data []byte) (out []byte, unused []byte, eof bool, err error) {
	br := bytes.NewReader(data)
	out, rerr := io.ReadAll(bzip2.NewReader(br))
	if rerr == nil {
		unused = data[len(data)-br.Len():]
		return out, unused, true, nil
	}
	if rerr == io.ErrUnexpectedEOF || rerr == io.EOF {
		return out, nil, false, nil
	}
	return nil, nil, false, rerr
}

// bz2DecompressMethod dispatches the BZ2Decompressor's one method.
func bz2DecompressMethod(d *bz2DecompressObject, name string, args []Object) (Object, error) {
	switch name {
	case "decompress":
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "decompress() takes 1 or 2 arguments (%d given)", len(args))
		}
		if d.eof {
			return nil, Raise("EOFError", "end of stream was already found")
		}
		data, ok := AsBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		maxLength := -1
		if len(args) == 2 {
			m, ok := AsInt(args[1])
			if !ok {
				return nil, Raise(TypeError, "an integer is required")
			}
			maxLength = int(m)
		}
		d.input = append(d.input, data...)
		out, unused, end, err := bz2Inflate(d.input)
		if err != nil {
			return nil, Raise("OSError", "Invalid data stream")
		}
		d.out = out
		d.streamEnd = end
		avail := d.out[d.returned:]
		if maxLength >= 0 && len(avail) > maxLength {
			avail = avail[:maxLength]
		}
		d.returned += len(avail)
		d.updateFlags(unused)
		return NewBytes(append([]byte(nil), avail...)), nil
	}
	return nil, noAttr(d, name)
}

// updateFlags sets eof, needs_input, and unused_data from the drain position,
// matching the zlib Decompress object: eof holds only once the stream ended and
// every decoded byte has been handed back, so a length-limited call that ended
// the stream but still owes output keeps eof false until the pending bytes drain.
// unused_data is the bytes past the stream end and only surfaces once eof holds.
func (d *bz2DecompressObject) updateFlags(unused []byte) {
	drained := d.returned == len(d.out)
	d.eof = d.streamEnd && drained
	d.needsIn = !d.streamEnd && drained
	if d.eof {
		d.unused = unused
	} else {
		d.unused = nil
	}
}

var bz2DecompressMethodNames = map[string]bool{"decompress": true}

// bz2DecompressLoadAttr reads the BZ2Decompressor's data attributes and binds its
// method. unused_data is the trailing bytes after the stream, eof reports the
// stream end, and needs_input reports whether the last call drained the input.
func bz2DecompressLoadAttr(d *bz2DecompressObject, name string) (Object, error) {
	switch name {
	case "unused_data":
		return NewBytes(append([]byte(nil), d.unused...)), nil
	case "eof":
		return NewBool(d.eof), nil
	case "needs_input":
		return NewBool(d.needsIn), nil
	}
	if bz2DecompressMethodNames[name] {
		return builtinMethodValue(d, name), nil
	}
	return nil, noAttr(d, name)
}

// bz2CompressObject is a _bz2.BZ2Compressor. It holds no state because there is
// no stdlib-only bzip2 compressor to drive; it exists so the object can be
// constructed the way CPython allows, and its compress and flush raise
// NotImplementedError with a clear message rather than silently producing wrong
// output.
type bz2CompressObject struct{}

func (c *bz2CompressObject) TypeName() string { return "_bz2.BZ2Compressor" }

// NewBZ2Compressor builds a BZ2Compressor. The compresslevel argument is
// validated by the runtime module for the CPython signature; the object itself
// carries no state.
func NewBZ2Compressor() Object { return &bz2CompressObject{} }

// bz2CompressMethod dispatches the BZ2Compressor's methods, both of which raise
// because Go's standard library has no bzip2 compressor.
func bz2CompressMethod(c *bz2CompressObject, name string, args []Object) (Object, error) {
	switch name {
	case "compress", "flush":
		return nil, Raise("NotImplementedError", "bzip2 compression is not available: this build provides bzip2 decompression only")
	}
	return nil, noAttr(c, name)
}

var bz2CompressMethodNames = map[string]bool{"compress": true, "flush": true}

// bz2CompressLoadAttr binds the BZ2Compressor's methods.
func bz2CompressLoadAttr(c *bz2CompressObject, name string) (Object, error) {
	if bz2CompressMethodNames[name] {
		return builtinMethodValue(c, name), nil
	}
	return nil, noAttr(c, name)
}
