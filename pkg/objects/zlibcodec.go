package objects

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
)

// The zlib module's streaming codec objects live here next to the other native
// types so their methods bind through the same LoadAttr and CallMethod paths;
// pkg/runtime/zlibmod.go builds them from compressobj and decompressobj. A
// Compress object wraps an open DEFLATE writer feeding an internal buffer that
// each compress and flush call drains; a Decompress object buffers its input and
// inflates it, tracking the stream end and the trailing bytes gzip reads back as
// its checksum.

// zlibWriter is the shared surface of the three framings' writers: zlib.Writer,
// flate.Writer, and gzip.Writer all write, sync-flush, and close.
type zlibWriter interface {
	io.Writer
	Flush() error
	Close() error
}

// zlibCompressObject is a zlib.compressobj result: an open writer draining into
// buf, plus the done flag flush(Z_FINISH) sets so a later compress raises the
// way CPython's finished compressor does.
type zlibCompressObject struct {
	buf  *bytes.Buffer
	w    zlibWriter
	done bool
}

func (c *zlibCompressObject) TypeName() string { return "zlib.Compress" }

// NewZlibCompress builds a Compress object writing the framing wbits selects at
// the given level: 9..15 is zlib framing, the negative range is raw DEFLATE, and
// 25..31 is gzip. An unsupported wbits is the ValueError compressobj raises.
func NewZlibCompress(level, wbits int) (Object, error) {
	buf := &bytes.Buffer{}
	var w zlibWriter
	var err error
	switch {
	case wbits >= 9 && wbits <= 15:
		w, err = zlib.NewWriterLevel(buf, level)
	case wbits <= -9 && wbits >= -15:
		w, err = flate.NewWriter(buf, level)
	case wbits >= 25 && wbits <= 31:
		w, err = gzip.NewWriterLevel(buf, level)
	default:
		return nil, Raise(ValueError, "Invalid initialization option")
	}
	if err != nil {
		return nil, Raise(ValueError, "%s", err.Error())
	}
	return &zlibCompressObject{buf: buf, w: w}, nil
}

// drain empties the buffer and returns what had accumulated, the bytes a
// compress or flush call hands back.
func (c *zlibCompressObject) drain() []byte {
	out := append([]byte(nil), c.buf.Bytes()...)
	c.buf.Reset()
	return out
}

// zlibCompressMethod dispatches the Compress object's methods.
func zlibCompressMethod(c *zlibCompressObject, name string, args []Object) (Object, error) {
	switch name {
	case "compress":
		if len(args) != 1 {
			return nil, Raise(TypeError, "compress() takes exactly one argument (%d given)", len(args))
		}
		if c.done {
			return nil, zlibCodecError("compressor object already flushed")
		}
		data, ok := AsBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		if _, err := c.w.Write(data); err != nil {
			return nil, zlibCodecError(err.Error())
		}
		return NewBytes(c.drain()), nil
	case "flush":
		mode := zlibZFinish
		if len(args) >= 1 {
			m, ok := AsInt(args[0])
			if !ok {
				return nil, Raise(TypeError, "an integer is required")
			}
			mode = int(m)
		}
		if c.done {
			return nil, zlibCodecError("compressor object already flushed")
		}
		// Z_FINISH ends the stream so the trailer is written and the object is
		// spent; the softer flush modes push pending output through but leave the
		// stream open for more compress calls.
		if mode == zlibZFinish {
			if err := c.w.Close(); err != nil {
				return nil, zlibCodecError(err.Error())
			}
			c.done = true
		} else if mode != zlibZNoFlush {
			if err := c.w.Flush(); err != nil {
				return nil, zlibCodecError(err.Error())
			}
		}
		return NewBytes(c.drain()), nil
	}
	return nil, noAttr(c, name)
}

var zlibCompressMethodNames = map[string]bool{"compress": true, "flush": true}

// zlibCompressLoadAttr binds the Compress object's methods.
func zlibCompressLoadAttr(c *zlibCompressObject, name string) (Object, error) {
	if zlibCompressMethodNames[name] {
		return builtinMethodValue(c, name), nil
	}
	return nil, noAttr(c, name)
}

const (
	zlibZNoFlush = 0
	zlibZFinish  = 4
)

// zlibDecompressObject is a zlib.decompressobj result. It accumulates the fed
// input and inflates the whole of it on each call, handing back the newly
// produced suffix; this keeps unused_data and eof exact, since the input is a
// bytes.Reader that flate consumes one byte at a time and stops precisely at the
// stream's end marker, leaving the trailing bytes in place.
type zlibDecompressObject struct {
	wbits     int
	name      string
	input     []byte
	out       []byte
	returned  int
	streamEnd bool
	eof       bool
	needsIn   bool
	unused    []byte
}

func (d *zlibDecompressObject) TypeName() string { return d.name }

// NewZlibDecompress builds a Decompress object reading the framing wbits selects,
// mirroring the one-shot decompress: the positive range is zlib framing, the
// negative range is raw DEFLATE, and the high ranges are gzip and autodetect.
func NewZlibDecompress(wbits int) Object {
	return &zlibDecompressObject{wbits: wbits, name: "zlib.Decompress", needsIn: true}
}

// NewZlibDecompressor builds the private _ZlibDecompressor GzipFile reads through.
// It shares the Decompress machinery but exposes needs_input and no flush, matching
// the C class DecompressReader drives one member at a time.
func NewZlibDecompressor(wbits int) Object {
	return &zlibDecompressObject{wbits: wbits, name: "zlib._ZlibDecompressor", needsIn: true}
}

// zlibInflate inflates the whole buffer, returning the output, the trailing
// bytes left after the stream, and whether the stream reached its end. A
// truncated stream is not an error: it reports eof false and the partial output,
// so a later call with more input picks up where it left off. Corrupt data is
// the zlib.error the caller raises.
func zlibInflate(wbits int, data []byte) (out []byte, unused []byte, eof bool, err error) {
	br := bytes.NewReader(data)
	r, cerr := zlibNewInflater(wbits, br)
	if cerr != nil {
		if cerr == io.ErrUnexpectedEOF || cerr == io.EOF {
			return nil, nil, false, nil
		}
		return nil, nil, false, cerr
	}
	out, rerr := io.ReadAll(r)
	_ = r.Close()
	if rerr == nil {
		unused = data[len(data)-br.Len():]
		return out, unused, true, nil
	}
	if rerr == io.ErrUnexpectedEOF || rerr == io.EOF {
		return out, nil, false, nil
	}
	return nil, nil, false, rerr
}

// zlibNewInflater picks the inflate reader for wbits over a byte-at-a-time
// reader, so it stops at the stream end and never over-reads into the trailer.
func zlibNewInflater(wbits int, br *bytes.Reader) (io.ReadCloser, error) {
	switch {
	case wbits >= 8 && wbits <= 15, wbits == 0:
		return zlib.NewReader(br)
	case wbits <= -8 && wbits >= -15:
		return flate.NewReader(br), nil
	case wbits >= 24 && wbits <= 31:
		return gzip.NewReader(br)
	case wbits >= 40 && wbits <= 47:
		if br.Len() >= 2 {
			b, _ := br.ReadByte()
			b2, _ := br.ReadByte()
			_, _ = br.Seek(0, io.SeekStart)
			if b == 0x1f && b2 == 0x8b {
				return gzip.NewReader(br)
			}
		}
		return zlib.NewReader(br)
	default:
		return nil, Raise(ValueError, "Invalid initialization option")
	}
}

// zlibDecompressMethod dispatches the Decompress object's methods.
func zlibDecompressMethod(d *zlibDecompressObject, name string, args []Object) (Object, error) {
	switch name {
	case "decompress":
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "decompress() takes 1 or 2 arguments (%d given)", len(args))
		}
		data, ok := AsBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		maxLength := 0
		if len(args) == 2 {
			m, ok := AsInt(args[1])
			if !ok {
				return nil, Raise(TypeError, "an integer is required")
			}
			maxLength = int(m)
		}
		d.input = append(d.input, data...)
		out, unused, end, err := zlibInflate(d.wbits, d.input)
		if err != nil {
			return nil, zlibCodecError(err.Error())
		}
		d.out = out
		d.streamEnd = end
		avail := d.out[d.returned:]
		if maxLength > 0 && len(avail) > maxLength {
			avail = avail[:maxLength]
		}
		d.returned += len(avail)
		d.updateFlags(unused)
		return NewBytes(append([]byte(nil), avail...)), nil
	case "flush":
		// flush returns whatever decoded output has not been handed back yet; the
		// optional length hint only sizes a buffer, so it is accepted and ignored.
		avail := d.out[d.returned:]
		d.returned = len(d.out)
		_, unused, _, _ := zlibInflate(d.wbits, d.input)
		d.updateFlags(unused)
		return NewBytes(append([]byte(nil), avail...)), nil
	}
	return nil, noAttr(d, name)
}

// updateFlags sets eof, needs_input, and unused_data from the drain position. eof
// holds only once the stream ended and every decoded byte has been handed back,
// so a length-limited call that ended the stream but still owes output keeps eof
// false until the pending bytes drain; this is what lets GzipFile read the trailer
// at the right moment instead of losing the tail of a member. unused_data is the
// bytes past the stream end and only surfaces once eof holds.
func (d *zlibDecompressObject) updateFlags(unused []byte) {
	drained := d.returned == len(d.out)
	d.eof = d.streamEnd && drained
	d.needsIn = !d.streamEnd && drained
	if d.eof {
		d.unused = unused
	} else {
		d.unused = nil
	}
}

// zlibCodecError raises the module's zlib.error carrying msg, falling back to a
// plain value error when the module has not stashed the class yet.
func zlibCodecError(msg string) error {
	if zlibModuleError != nil {
		if inst, cerr := Call(zlibModuleError, []Object{NewStr(msg)}); cerr == nil {
			if e, ok := inst.(error); ok {
				return e
			}
		}
	}
	return Raise(ValueError, "%s", msg)
}

// zlibModuleError is zlib.error, set by the runtime module at init so the codec
// objects raise the same class the one-shot functions do.
var zlibModuleError Object

// SetZlibError records the zlib.error class for the streaming codec to raise.
func SetZlibError(cls Object) { zlibModuleError = cls }

var zlibDecompressMethodNames = map[string]bool{"decompress": true, "flush": true}

// zlibDecompressLoadAttr reads the Decompress object's data attributes and binds
// its methods. unused_data is the trailing bytes gzip reads its checksum from,
// eof reports the stream end, and unconsumed_tail is the input a length-limited
// decompress left unread.
func zlibDecompressLoadAttr(d *zlibDecompressObject, name string) (Object, error) {
	switch name {
	case "unused_data":
		return NewBytes(append([]byte(nil), d.unused...)), nil
	case "unconsumed_tail":
		return NewBytes(nil), nil
	case "eof":
		return NewBool(d.eof), nil
	case "needs_input":
		return NewBool(d.needsIn), nil
	}
	if zlibDecompressMethodNames[name] {
		return builtinMethodValue(d, name), nil
	}
	return nil, noAttr(d, name)
}
