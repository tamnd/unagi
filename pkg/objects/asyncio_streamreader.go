package objects

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
// This slice is the core: the constructor, feed_data/feed_eof/at_eof, the
// exception surface, and read(n). readline, readexactly, readuntil, and async
// iteration are later slices, as are the StreamWriter half and the socket
// transports that feed a live reader.

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
