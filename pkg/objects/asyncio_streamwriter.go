package objects

import "net"

// asyncio.StreamWriter is the write half of the streams API. It hands bytes to a
// transport and exposes the flow-control and shutdown surface a caller drives:
// write/writelines buffer data, drain awaits the transport draining below its
// high-water mark, and close/wait_closed shut the connection down. Like the read
// half it lives on the loop goroutine, so its fields need no lock.
//
// This slice backs a writer by an in-memory transport that feeds a paired
// StreamReader, the shape a connected pair takes: whatever one side writes the
// other side reads. open_connection and start_server put a socket transport
// behind the same surface in later slices, and start_tls waits on the SSL work.
// Without a socket there is no send buffer to fill, so a write is always
// accepted and drain returns at once, byte-identical to a CPython writer whose
// transport never pauses.

// asyncioStreamTransport is the transport behind a writer. With a peer reader and
// no conn it is the in-memory half of a connected pair: what is written lands in
// the peer's StreamReader buffer, and a close feeds the peer EOF. With a conn it
// is a socket transport: writes go to the connection and close shuts it down, the
// form open_connection and start_server put behind the writer. Both run on the
// loop goroutine.
type asyncioStreamTransport struct {
	peer    *asyncioStreamReader
	conn    net.Conn
	closing bool
	eofSent bool
	extra   map[string]Object
}

func (tr *asyncioStreamTransport) write(b []byte) {
	if len(b) == 0 || tr.closing {
		return
	}
	if tr.conn != nil {
		// A short or failed write surfaces to the reader as EOF via the read pump;
		// the writer does not raise here, matching a CPython transport that reports
		// send errors through the protocol rather than from write().
		_, _ = tr.conn.Write(b)
		return
	}
	if tr.peer == nil || tr.peer.eof {
		return
	}
	tr.peer.buffer = append(tr.peer.buffer, b...)
	tr.peer.wakeupWaiter()
}

func (tr *asyncioStreamTransport) writeEOF() {
	if tr.eofSent || tr.closing {
		return
	}
	tr.eofSent = true
	if tr.conn != nil {
		if c, ok := tr.conn.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
		return
	}
	if tr.peer != nil {
		tr.peer.feedEOF()
	}
}

func (tr *asyncioStreamTransport) canWriteEOF() bool {
	if tr.conn != nil {
		_, ok := tr.conn.(interface{ CloseWrite() error })
		return ok
	}
	return true
}

func (tr *asyncioStreamTransport) close() {
	if tr.closing {
		return
	}
	tr.closing = true
	if tr.conn != nil {
		_ = tr.conn.Close()
		return
	}
	if tr.peer != nil {
		tr.peer.feedEOF()
	}
}

func (tr *asyncioStreamTransport) getExtraInfo(name string, def Object) Object {
	if tr.extra != nil {
		if v, ok := tr.extra[name]; ok {
			return v
		}
	}
	return def
}

type asyncioStreamWriter struct {
	tr *asyncioStreamTransport
	// reader is the local read half, whose stored exception a drain surfaces the
	// way CPython's writer reports a transport error to the writer.
	reader       *asyncioStreamReader
	closed       bool
	closeWaiters []*asyncFuture
}

func (w *asyncioStreamWriter) TypeName() string { return "StreamWriter" }

// newStreamWriterTo builds a writer whose transport feeds the given reader, the
// one-direction half of a connected pair.
func newStreamWriterTo(peer *asyncioStreamReader) *asyncioStreamWriter {
	return &asyncioStreamWriter{tr: &asyncioStreamTransport{peer: peer}}
}

// asyncioStreamWriterMethod dispatches the writer surface for a positional call.
func asyncioStreamWriterMethod(w *asyncioStreamWriter, name string, args []Object) (Object, error) {
	switch name {
	case "write":
		if len(args) != 1 {
			return nil, Raise(TypeError, "write() takes exactly one argument (%d given)", len(args))
		}
		b, ok := bytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		w.tr.write(b)
		return None, nil
	case "writelines":
		if len(args) != 1 {
			return nil, Raise(TypeError, "writelines() takes exactly one argument (%d given)", len(args))
		}
		return None, w.writelines(args[0])
	case "write_eof":
		if len(args) != 0 {
			return nil, Raise(TypeError, "write_eof() takes 1 positional argument but %d were given", len(args)+1)
		}
		w.tr.writeEOF()
		return None, nil
	case "can_write_eof":
		if len(args) != 0 {
			return nil, Raise(TypeError, "can_write_eof() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(w.tr.canWriteEOF()), nil
	case "is_closing":
		if len(args) != 0 {
			return nil, Raise(TypeError, "is_closing() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(w.tr.closing), nil
	case "close":
		if len(args) != 0 {
			return nil, Raise(TypeError, "close() takes 1 positional argument but %d were given", len(args)+1)
		}
		w.close()
		return None, nil
	case "drain":
		if len(args) != 0 {
			return nil, Raise(TypeError, "drain() takes 1 positional argument but %d were given", len(args)+1)
		}
		return w.drainCoro(), nil
	case "wait_closed":
		if len(args) != 0 {
			return nil, Raise(TypeError, "wait_closed() takes 1 positional argument but %d were given", len(args)+1)
		}
		return w.waitClosedCoro(), nil
	case "get_extra_info":
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "get_extra_info() takes from 2 to 3 positional arguments but %d were given", len(args)+1)
		}
		key, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "get_extra_info() argument must be str, not %s", args[0].TypeName())
		}
		def := Object(None)
		if len(args) == 2 {
			def = args[1]
		}
		return w.tr.getExtraInfo(key, def), nil
	}
	return nil, noAttr(w, name)
}

// asyncioStreamWriterMethodKw handles get_extra_info(default=...); every other
// writer method rejects keywords the way CPython does.
func asyncioStreamWriterMethodKw(w *asyncioStreamWriter, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name == "get_extra_info" {
		var keyArg, defArg Object
		if len(pos) > 2 {
			return nil, Raise(TypeError, "get_extra_info() takes from 2 to 3 positional arguments but %d were given", len(pos)+1)
		}
		if len(pos) >= 1 {
			keyArg = pos[0]
		}
		if len(pos) == 2 {
			defArg = pos[1]
		}
		for i, k := range kwNames {
			switch k {
			case "name":
				if keyArg != nil {
					return nil, Raise(TypeError, "argument for get_extra_info() given by name ('name') and position")
				}
				keyArg = kwVals[i]
			case "default":
				if defArg != nil {
					return nil, Raise(TypeError, "argument for get_extra_info() given by name ('default') and position")
				}
				defArg = kwVals[i]
			default:
				return nil, Raise(TypeError, "'%s' is an invalid keyword argument for get_extra_info()", k)
			}
		}
		if keyArg == nil {
			return nil, Raise(TypeError, "get_extra_info() missing 1 required positional argument: 'name'")
		}
		key, ok := AsStr(keyArg)
		if !ok {
			return nil, Raise(TypeError, "get_extra_info() argument must be str, not %s", keyArg.TypeName())
		}
		if defArg == nil {
			defArg = None
		}
		return w.tr.getExtraInfo(key, defArg), nil
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", w.TypeName(), name)
}

// writelines writes each item of an iterable of bytes-like objects in order, the
// gather-then-write CPython's writelines performs.
func (w *asyncioStreamWriter) writelines(data Object) error {
	it, err := Iter(data)
	if err != nil {
		return err
	}
	for {
		item, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		b, ok := bytesLike(item)
		if !ok {
			return Raise(TypeError, "a bytes-like object is required, not '%s'", item.TypeName())
		}
		w.tr.write(b)
	}
	return nil
}

// close shuts the writer down: the transport feeds the peer EOF, and any coroutine
// waiting in wait_closed is resolved.
func (w *asyncioStreamWriter) close() {
	w.tr.close()
	if w.closed {
		return
	}
	w.closed = true
	for _, f := range w.closeWaiters {
		if !f.isCancelled() {
			f.setResult(None)
		}
	}
	w.closeWaiters = nil
}

// drainCoro is the coroutine drain returns. With no send buffer to fill, the
// transport is always writable, so drain returns at once; a stored reader
// exception surfaces here the way CPython's drain reports a transport error.
func (w *asyncioStreamWriter) drainCoro() Object {
	body := func(y Yielder) (Object, error) {
		if w.reader != nil && w.reader.exc != nil {
			return nil, w.reader.exc
		}
		return None, nil
	}
	return &generatorObject{qual: "StreamWriter.drain", body: fromTop(body), ret: None, isCoro: true}
}

// waitClosedCoro is the coroutine wait_closed returns, parking until close runs.
func (w *asyncioStreamWriter) waitClosedCoro() Object {
	body := func(y Yielder) (Object, error) {
		if w.closed {
			return None, nil
		}
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		fut := &asyncFuture{loop: loop}
		w.closeWaiters = append(w.closeWaiters, fut)
		_, err := y.YieldFrom(&futureAwait{f: fut})
		return None, err
	}
	return &generatorObject{qual: "StreamWriter.wait_closed", body: fromTop(body), ret: None, isCoro: true}
}
