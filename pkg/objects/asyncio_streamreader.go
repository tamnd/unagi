package objects

import (
	"bytes"
	"fmt"
	"sync"
)

// asyncio.StreamReader is the read half of the streams API. It buffers bytes fed
// by a transport and hands them to a coroutine that reads them, suspending the
// reader while the buffer is empty and no EOF has arrived. On its own, fed by
// feed_data and feed_eof, it is a deterministic byte buffer that a coroutine
// drains; open_connection and start_server build one over a socket transport in
// later slices. Like the other asyncio primitives it lives on the loop
// goroutine, so its fields need no lock; the one off-loop writer a real transport
// adds hands data in through the loop's callSoon, which serialises against the
// reader's own steps.
//
// The read surface is the constructor, feed_data/feed_eof/at_eof, the exception
// surface, read(n), and the read-until family: readexactly(n), readuntil(sep),
// readline, and async iteration over lines. The StreamWriter half and the socket
// transports that feed a live reader are later slices.

// streamReaderDefaultLimit is CPython's _DEFAULT_LIMIT, the 64 KiB buffer bound a
// StreamReader built without an explicit limit uses.
const streamReaderDefaultLimit = 1 << 16

type asyncioStreamReader struct {
	buffer []byte
	eof    bool
	limit  int
	// waiter is the single future a read parks on while the buffer is empty. Only
	// one coroutine may wait at a time, matching CPython, which raises when a
	// second read starts while the first is still waiting for data.
	waiter *asyncFuture
	// exc is the exception set_exception stored, re-raised at the next read the way
	// CPython's StreamReader surfaces a transport error to its reader.
	exc *Exception
}

func (r *asyncioStreamReader) TypeName() string { return "StreamReader" }

// AsyncioNewStreamReader builds asyncio.StreamReader(limit). A limit of zero or
// less is the ValueError CPython raises, since the buffer bound must be positive.
func AsyncioNewStreamReader(limit int) (Object, error) {
	if limit <= 0 {
		return nil, Raise(ValueError, "Limit cannot be <= 0")
	}
	return &asyncioStreamReader{limit: limit}, nil
}

// asyncioStreamReaderMethod dispatches the reader surface for a positional call.
// feed_data, feed_eof, and set_exception act at once; at_eof and exception read
// the state; read hands back the coroutine the caller awaits.
func asyncioStreamReaderMethod(r *asyncioStreamReader, name string, args []Object) (Object, error) {
	switch name {
	case "feed_data":
		if len(args) != 1 {
			return nil, Raise(TypeError, "feed_data() takes exactly one argument (%d given)", len(args))
		}
		return None, r.feedData(args[0])
	case "feed_eof":
		if len(args) != 0 {
			return nil, Raise(TypeError, "feed_eof() takes 1 positional argument but %d were given", len(args)+1)
		}
		r.feedEOF()
		return None, nil
	case "at_eof":
		if len(args) != 0 {
			return nil, Raise(TypeError, "at_eof() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(r.atEOF()), nil
	case "exception":
		if len(args) != 0 {
			return nil, Raise(TypeError, "exception() takes 1 positional argument but %d were given", len(args)+1)
		}
		if r.exc == nil {
			return None, nil
		}
		return errorObject(r.exc), nil
	case "set_exception":
		if len(args) != 1 {
			return nil, Raise(TypeError, "set_exception() takes exactly one argument (%d given)", len(args))
		}
		return None, r.setException(args[0])
	case "read":
		if len(args) > 1 {
			return nil, Raise(TypeError, "read() takes from 1 to 2 positional arguments but %d were given", len(args)+1)
		}
		n := -1
		if len(args) == 1 {
			v, ok := AsInt(args[0])
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
			}
			n = int(v)
		}
		return r.readCoro(n), nil
	case "readexactly":
		if len(args) != 1 {
			return nil, Raise(TypeError, "readexactly() takes exactly one argument (%d given)", len(args))
		}
		v, ok := AsInt(args[0])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
		}
		return r.readexactlyCoro(int(v)), nil
	case "readuntil":
		if len(args) > 1 {
			return nil, Raise(TypeError, "readuntil() takes from 1 to 2 positional arguments but %d were given", len(args)+1)
		}
		sep := []byte("\n")
		if len(args) == 1 {
			b, ok := bytesLike(args[0])
			if !ok {
				return nil, Raise(TypeError, "argument should be a bytes-like object, not '%s'", args[0].TypeName())
			}
			sep = append([]byte(nil), b...)
		}
		return r.readuntilCoro(sep), nil
	case "readline":
		if len(args) != 0 {
			return nil, Raise(TypeError, "readline() takes 1 positional argument but %d were given", len(args)+1)
		}
		return r.readlineCoro(), nil
	case "__aiter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aiter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return r, nil
	case "__anext__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__anext__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return r.anextCoro(), nil
	}
	return nil, noAttr(r, name)
}

// asyncioStreamReaderMethodKw handles the one reader method that takes a keyword,
// read(n=-1); every other method rejects keywords as CPython does.
func asyncioStreamReaderMethodKw(r *asyncioStreamReader, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name == "read" {
		if len(pos) > 1 {
			return nil, Raise(TypeError, "read() takes from 1 to 2 positional arguments but %d were given", len(pos)+1)
		}
		var nArg Object
		if len(pos) == 1 {
			nArg = pos[0]
		}
		for i, k := range kwNames {
			if k != "n" {
				return nil, Raise(TypeError, "'%s' is an invalid keyword argument for read()", k)
			}
			if nArg != nil {
				return nil, Raise(TypeError, "argument for read() given by name ('n') and position")
			}
			nArg = kwVals[i]
		}
		n := -1
		if nArg != nil {
			v, ok := AsInt(nArg)
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", nArg.TypeName())
			}
			n = int(v)
		}
		return r.readCoro(n), nil
	}
	if name == "readexactly" {
		var nArg Object
		if len(pos) > 1 {
			return nil, Raise(TypeError, "readexactly() takes 2 positional arguments but %d were given", len(pos)+1)
		}
		if len(pos) == 1 {
			nArg = pos[0]
		}
		for i, k := range kwNames {
			if k != "n" {
				return nil, Raise(TypeError, "'%s' is an invalid keyword argument for readexactly()", k)
			}
			if nArg != nil {
				return nil, Raise(TypeError, "argument for readexactly() given by name ('n') and position")
			}
			nArg = kwVals[i]
		}
		if nArg == nil {
			return nil, Raise(TypeError, "readexactly() missing 1 required positional argument: 'n'")
		}
		v, ok := AsInt(nArg)
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", nArg.TypeName())
		}
		return r.readexactlyCoro(int(v)), nil
	}
	if name == "readuntil" {
		var sepArg Object
		if len(pos) > 1 {
			return nil, Raise(TypeError, "readuntil() takes from 1 to 2 positional arguments but %d were given", len(pos)+1)
		}
		if len(pos) == 1 {
			sepArg = pos[0]
		}
		for i, k := range kwNames {
			if k != "separator" {
				return nil, Raise(TypeError, "'%s' is an invalid keyword argument for readuntil()", k)
			}
			if sepArg != nil {
				return nil, Raise(TypeError, "argument for readuntil() given by name ('separator') and position")
			}
			sepArg = kwVals[i]
		}
		sep := []byte("\n")
		if sepArg != nil {
			b, ok := bytesLike(sepArg)
			if !ok {
				return nil, Raise(TypeError, "argument should be a bytes-like object, not '%s'", sepArg.TypeName())
			}
			sep = append([]byte(nil), b...)
		}
		return r.readuntilCoro(sep), nil
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", r.TypeName(), name)
}

// feedData appends data to the buffer and wakes a parked reader, matching
// CPython's feed_data. Feeding after feed_eof is the AssertionError CPython's
// assert raises, and empty data is a no-op that wakes nobody.
func (r *asyncioStreamReader) feedData(data Object) error {
	b, ok := bytesLike(data)
	if !ok {
		return Raise(TypeError, "a bytes-like object is required, not '%s'", data.TypeName())
	}
	if r.eof {
		return Raise("AssertionError", "feed_data after feed_eof")
	}
	if len(b) == 0 {
		return nil
	}
	r.buffer = append(r.buffer, b...)
	r.wakeupWaiter()
	return nil
}

// feedEOF marks the stream ended and wakes a parked reader so it observes the
// EOF and returns what it has.
func (r *asyncioStreamReader) feedEOF() {
	r.eof = true
	r.wakeupWaiter()
}

// atEOF reports that the stream has ended and the buffer is drained, the point
// where read returns empty bytes.
func (r *asyncioStreamReader) atEOF() bool { return r.eof && len(r.buffer) == 0 }

// setException stores exc and fails a parked reader with it, so a transport error
// surfaces at the next read. The argument is turned into a raisable instance the
// way a raise statement would, matching Future.set_exception on the waiter.
func (r *asyncioStreamReader) setException(exc Object) error {
	e, err := asRaiseInstance(exc)
	if err != nil {
		return err
	}
	r.exc = e
	if w := r.waiter; w != nil {
		r.waiter = nil
		if !w.isCancelled() {
			w.setException(e)
		}
	}
	return nil
}

// wakeupWaiter resolves the parked reader's future, if any, so its read resumes.
// A cancelled waiter is left alone, matching CPython's _wakeup_waiter.
func (r *asyncioStreamReader) wakeupWaiter() {
	w := r.waiter
	if w == nil {
		return
	}
	r.waiter = nil
	if !w.isCancelled() {
		w.setResult(None)
	}
}

// waitForData parks the reading coroutine on a fresh future until feed_data or
// feed_eof wakes it. A second reader arriving while one already waits is the
// RuntimeError CPython raises, since a StreamReader serves one reader at a time.
func (r *asyncioStreamReader) waitForData(y Yielder, funcName string) error {
	if r.waiter != nil {
		return Raise(RuntimeError, "%s() called while another coroutine is already waiting for incoming data", funcName)
	}
	loop := runningLoop.Load()
	if loop == nil {
		return Raise(RuntimeError, "no running event loop")
	}
	fut := &asyncFuture{loop: loop}
	r.waiter = fut
	_, err := y.YieldFrom(&futureAwait{f: fut})
	r.waiter = nil
	return err
}

// readCoro is the coroutine read returns. A stored exception is re-raised first.
// read(0) is empty bytes; read(-1) drains to EOF; read(n) returns up to n bytes,
// parking once while the buffer is empty and the stream has not ended.
func (r *asyncioStreamReader) readCoro(n int) Object {
	body := func(y Yielder) (Object, error) {
		if r.exc != nil {
			return nil, r.exc
		}
		if n == 0 {
			return NewBytes(nil), nil
		}
		if n < 0 {
			var out []byte
			for {
				if len(r.buffer) == 0 {
					if r.eof {
						break
					}
					if err := r.waitForData(y, "read"); err != nil {
						return nil, err
					}
					continue
				}
				out = append(out, r.buffer...)
				r.buffer = r.buffer[:0]
			}
			return NewBytes(out), nil
		}
		if len(r.buffer) == 0 && !r.eof {
			if err := r.waitForData(y, "read"); err != nil {
				return nil, err
			}
		}
		take := n
		if take > len(r.buffer) {
			take = len(r.buffer)
		}
		data := append([]byte(nil), r.buffer[:take]...)
		r.buffer = r.buffer[take:]
		return NewBytes(data), nil
	}
	return &generatorObject{qual: "StreamReader.read", body: fromTop(body), ret: None, isCoro: true}
}

// bytesLike reads the raw bytes of a bytes or bytearray, the two buffer types a
// transport feeds a reader. A memoryview and other buffers are later work.
func bytesLike(o Object) ([]byte, bool) {
	switch b := o.(type) {
	case *bytesObject:
		return b.v, true
	case *bytearrayObject:
		return b.v, true
	}
	return nil, false
}

// asyncio.IncompleteReadError and asyncio.LimitOverrunError are the two
// exceptions the read-until family raises. They live in the asyncio.exceptions
// module, matching their __module__ in CPython, and are built once on demand.
// IncompleteReadError derives from EOFError and carries .partial (the bytes read
// before EOF) and .expected (the total wanted, or None); readline swallows it and
// returns the partial line. LimitOverrunError derives from Exception and carries
// .consumed (the offset the separator search reached); readline turns it into the
// ValueError CPython raises when a line exceeds the limit.
var (
	asyncioIncompleteReadOnce  sync.Once
	asyncioIncompleteReadClass *classObject
	asyncioLimitOverrunOnce    sync.Once
	asyncioLimitOverrunClass   *classObject
)

const asyncioExceptionsModule = "asyncio.exceptions"

// AsyncioIncompleteReadErrorClass returns asyncio.IncompleteReadError, raised when
// EOF arrives before readexactly or readuntil has the bytes it was asked for.
func AsyncioIncompleteReadErrorClass() Object {
	asyncioIncompleteReadOnce.Do(func() {
		asyncioIncompleteReadClass = buildAsyncioStreamExc("IncompleteReadError", ExcClass2("EOFError"))
	})
	return asyncioIncompleteReadClass
}

// AsyncioLimitOverrunErrorClass returns asyncio.LimitOverrunError, raised when a
// readuntil separator search runs past the reader's buffer limit.
func AsyncioLimitOverrunErrorClass() Object {
	asyncioLimitOverrunOnce.Do(func() {
		asyncioLimitOverrunClass = buildAsyncioStreamExc("LimitOverrunError", ExcClass2("Exception"))
	})
	return asyncioLimitOverrunClass
}

func buildAsyncioStreamExc(name string, base Object) *classObject {
	qual := asyncioExceptionsModule + "." + name
	c, err := NewClass(name, qual, []Object{base}, []string{"__module__"}, []Object{NewStr(asyncioExceptionsModule)}, nil, nil)
	if err != nil {
		panic("unagi: building " + qual + ": " + err.Error())
	}
	return c.(*classObject)
}

// newIncompleteReadError builds asyncio.IncompleteReadError(partial, expected),
// mirroring CPython's __init__: the message reports how many bytes were read of
// how many expected (repr of expected, or 'undefined' when it is None), and
// .partial/.expected are set as instance attributes.
func newIncompleteReadError(partial []byte, expected Object) error {
	rExpected := "undefined"
	if expected != nil && expected != None {
		rExpected = Repr(expected)
	}
	msg := fmt.Sprintf("%d bytes read on a total of %s expected bytes", len(partial), rExpected)
	inst, err := Instantiate(AsyncioIncompleteReadErrorClass().(*classObject), []Object{NewStr(msg)}, nil, nil)
	if err != nil {
		return err
	}
	e, ok := inst.(*Exception)
	if !ok {
		return Raise(RuntimeError, "IncompleteReadError")
	}
	exp := expected
	if exp == nil {
		exp = None
	}
	if _, err := excStoreAttr(e, "partial", NewBytes(append([]byte(nil), partial...))); err != nil {
		return err
	}
	if _, err := excStoreAttr(e, "expected", exp); err != nil {
		return err
	}
	return e
}

// newLimitOverrunError builds asyncio.LimitOverrunError(message, consumed),
// mirroring CPython's __init__: the message becomes the sole arg and .consumed
// records how far the separator search had reached.
func newLimitOverrunError(message string, consumed int) error {
	inst, err := Instantiate(AsyncioLimitOverrunErrorClass().(*classObject), []Object{NewStr(message)}, nil, nil)
	if err != nil {
		return err
	}
	e, ok := inst.(*Exception)
	if !ok {
		return Raise(RuntimeError, "%s", message)
	}
	if _, err := excStoreAttr(e, "consumed", NewInt(int64(consumed))); err != nil {
		return err
	}
	return e
}

// readexactlyCoro is the coroutine readexactly returns. It reads exactly n bytes,
// parking while the buffer is short of n and no EOF has arrived. A negative n is
// the ValueError CPython raises; EOF before n bytes is IncompleteReadError
// carrying the partial read and the expected total.
func (r *asyncioStreamReader) readexactlyCoro(n int) Object {
	body := func(y Yielder) (Object, error) {
		if n < 0 {
			return nil, Raise(ValueError, "readexactly size can not be less than zero")
		}
		if r.exc != nil {
			return nil, r.exc
		}
		if n == 0 {
			return NewBytes(nil), nil
		}
		for len(r.buffer) < n {
			if r.eof {
				incomplete := append([]byte(nil), r.buffer...)
				r.buffer = r.buffer[:0]
				return nil, newIncompleteReadError(incomplete, NewInt(int64(n)))
			}
			if err := r.waitForData(y, "readexactly"); err != nil {
				return nil, err
			}
		}
		data := append([]byte(nil), r.buffer[:n]...)
		r.buffer = r.buffer[n:]
		return NewBytes(data), nil
	}
	return &generatorObject{qual: "StreamReader.readexactly", body: fromTop(body), ret: None, isCoro: true}
}

// readUntilImpl runs the separator search readuntil and readline share. It scans
// for sep, parking while it is absent and the stream is open, raising
// LimitOverrunError once the unmatched span passes the limit and
// IncompleteReadError when EOF arrives with no separator. On success it removes
// the line through the separator from the buffer and returns it.
func (r *asyncioStreamReader) readUntilImpl(y Yielder, sep []byte) (Object, error) {
	seplen := len(sep)
	if seplen == 0 {
		return nil, Raise(ValueError, "Separator should be at least one-byte string")
	}
	if r.exc != nil {
		return nil, r.exc
	}
	offset := 0
	isep := -1
	for {
		buflen := len(r.buffer)
		if buflen-offset >= seplen {
			isep = indexFrom(r.buffer, sep, offset)
			if isep != -1 {
				break
			}
			offset = buflen + 1 - seplen
			if offset > r.limit {
				return nil, newLimitOverrunError("Separator is not found, and chunk exceed the limit", offset)
			}
		}
		if r.eof {
			chunk := append([]byte(nil), r.buffer...)
			r.buffer = r.buffer[:0]
			return nil, newIncompleteReadError(chunk, None)
		}
		if err := r.waitForData(y, "readuntil"); err != nil {
			return nil, err
		}
	}
	if isep > r.limit {
		return nil, newLimitOverrunError("Separator is found, but chunk is longer than limit", isep)
	}
	end := isep + seplen
	chunk := append([]byte(nil), r.buffer[:end]...)
	r.buffer = r.buffer[end:]
	return NewBytes(chunk), nil
}

// readuntilCoro is the coroutine readuntil returns, reading up to and including
// the separator.
func (r *asyncioStreamReader) readuntilCoro(sep []byte) Object {
	body := func(y Yielder) (Object, error) { return r.readUntilImpl(y, sep) }
	return &generatorObject{qual: "StreamReader.readuntil", body: fromTop(body), ret: None, isCoro: true}
}

// readLineImpl runs the readline logic readline and __anext__ share. It reads to
// the next newline, returning the partial data on EOF (IncompleteReadError) and,
// when a line overruns the limit (LimitOverrunError), trimming what it can from
// the buffer before re-raising the overrun as the ValueError CPython surfaces.
func (r *asyncioStreamReader) readLineImpl(y Yielder) (Object, error) {
	sep := []byte("\n")
	line, err := r.readUntilImpl(y, sep)
	if err == nil {
		return line, nil
	}
	e, ok := err.(*Exception)
	if !ok {
		return nil, err
	}
	if ExcMatchesClass(e, AsyncioIncompleteReadErrorClass()) {
		if p, ok := e.Dict["partial"]; ok {
			return p, nil
		}
		return NewBytes(nil), nil
	}
	if ExcMatchesClass(e, AsyncioLimitOverrunErrorClass()) {
		consumed := 0
		if c, ok := e.Dict["consumed"]; ok {
			if v, ok2 := AsInt(c); ok2 {
				consumed = int(v)
			}
		}
		if startsAt(r.buffer, sep, consumed) {
			cut := consumed + len(sep)
			if cut > len(r.buffer) {
				cut = len(r.buffer)
			}
			r.buffer = r.buffer[cut:]
		} else {
			r.buffer = r.buffer[:0]
		}
		msg := ""
		if len(e.Args) > 0 {
			if s, ok := AsStr(e.Args[0]); ok {
				msg = s
			}
		}
		return nil, Raise(ValueError, "%s", msg)
	}
	return nil, err
}

// readlineCoro is the coroutine readline returns.
func (r *asyncioStreamReader) readlineCoro() Object {
	body := func(y Yielder) (Object, error) { return r.readLineImpl(y) }
	return &generatorObject{qual: "StreamReader.readline", body: fromTop(body), ret: None, isCoro: true}
}

// anextCoro is the coroutine __anext__ returns, driving the async-for protocol: it
// reads a line and raises StopAsyncIteration once readline returns empty bytes at
// end of stream.
func (r *asyncioStreamReader) anextCoro() Object {
	body := func(y Yielder) (Object, error) {
		val, err := r.readLineImpl(y)
		if err != nil {
			return nil, err
		}
		if b, ok := AsBytes(val); ok && len(b) == 0 {
			return nil, &Exception{Kind: "StopAsyncIteration", Context: CurrentHandled()}
		}
		return val, nil
	}
	return &generatorObject{qual: "StreamReader.__anext__", body: fromTop(body), ret: None, isCoro: true}
}

// indexFrom finds sep in buf at or after start, returning the absolute index or
// -1, the bytes-buffer find(sep, start) CPython's readuntil uses.
func indexFrom(buf, sep []byte, start int) int {
	if start > len(buf) {
		return -1
	}
	i := bytes.Index(buf[start:], sep)
	if i < 0 {
		return -1
	}
	return start + i
}

// startsAt reports whether buf has sep at offset off, the buffer.startswith(sep,
// off) readline uses to decide how much of an over-limit line to drop.
func startsAt(buf, sep []byte, off int) bool {
	if off < 0 || off+len(sep) > len(buf) {
		return false
	}
	return bytes.Equal(buf[off:off+len(sep)], sep)
}
