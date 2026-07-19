package objects

import (
	"net"
	"strconv"
)

// This file puts a real socket behind the streams API: open_connection dials a
// client pair, start_server listens and hands each accepted connection to a
// handler coroutine, and the Server tracks the listener. The bridge between an
// off-loop socket and the single-threaded loop is the netpoller handoff the spec
// anticipated: a per-connection read goroutine blocks on conn.Read and feeds the
// bytes back through loop.callSoon, which serialises them against the loop's own
// steps, while addPending keeps the loop from declaring deadlock while a read or
// accept goroutine is still outstanding. Writes go straight to the connection
// from the loop goroutine; true send-buffer flow control is a later slice.

// newStreamWriterOverConn builds the write half over a live connection, with the
// local reader attached so drain can surface a transport error.
func newStreamWriterOverConn(conn net.Conn, reader *asyncioStreamReader, loop *eventLoop) *asyncioStreamWriter {
	return &asyncioStreamWriter{
		tr:     &asyncioStreamTransport{conn: conn},
		reader: reader,
	}
}

// startReadPump runs the read side of a connection: a goroutine blocks on Read and
// hands each chunk to the reader through callSoon, so the reader's buffer is only
// ever touched on the loop goroutine. EOF or a read error feeds the reader EOF.
// addPending holds the loop open while the goroutine is blocked; donePending wakes
// it once the connection ends.
func startReadPump(loop *eventLoop, conn net.Conn, reader *asyncioStreamReader) {
	loop.addPending()
	go func() {
		defer loop.donePending()
		buf := make([]byte, streamReaderDefaultLimit)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				loop.callSoon(func() {
					if reader.eof {
						return
					}
					reader.buffer = append(reader.buffer, chunk...)
					reader.wakeupWaiter()
				})
			}
			if err != nil {
				loop.callSoon(func() { reader.feedEOF() })
				return
			}
		}
	}()
}

// AsyncioOpenConnection implements asyncio.open_connection(host, port). It dials
// the address off the loop, then returns a (reader, writer) pair over the
// connection, the same shape start_server hands a handler.
func AsyncioOpenConnection(host string, port int) Object {
	body := func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		fut := &asyncFuture{loop: loop}
		var conn net.Conn
		var dialErr error
		loop.addPending()
		go func() {
			c, e := net.Dial("tcp", addr)
			loop.callSoon(func() {
				conn, dialErr = c, e
				fut.setResult(None)
			})
			loop.donePending()
		}()
		if _, err := y.YieldFrom(&futureAwait{f: fut}); err != nil {
			return nil, err
		}
		if dialErr != nil {
			return nil, Raise("ConnectionRefusedError", "%s", dialErr.Error())
		}
		reader, err := AsyncioNewStreamReader(streamReaderDefaultLimit)
		if err != nil {
			return nil, err
		}
		r := reader.(*asyncioStreamReader)
		w := newStreamWriterOverConn(conn, r, loop)
		startReadPump(loop, conn, r)
		return NewTuple([]Object{r, w}), nil
	}
	return &generatorObject{qual: "open_connection", body: fromTop(body), ret: None, isCoro: true}
}

// asyncioServer is asyncio.Server: it owns the listener and the accept goroutine,
// spawning a handler task per accepted connection.
type asyncioServer struct {
	ln           net.Listener
	loop         *eventLoop
	cb           Object
	serving      bool
	accepting    bool
	closed       bool
	closeWaiters []*asyncFuture
}

func (s *asyncioServer) TypeName() string { return "Server" }

// asyncioServerMethodNames is the Server method surface LoadAttr binds, so
// server.close and server.serve_forever read as bound methods.
var asyncioServerMethodNames = map[string]bool{
	"serve_forever": true,
	"wait_closed":   true,
	"close":         true,
	"is_serving":    true,
	"get_loop":      true,
	"start_serving": true,
	"__aenter__":    true,
	"__aexit__":     true,
}

// AsyncioStartServer implements asyncio.start_server(client_connected_cb, host,
// port). It binds a listener and begins accepting at once, the start_serving
// default, and returns the Server already listening.
func AsyncioStartServer(cb Object, host string, port int) Object {
	body := func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, Raise("OSError", "%s", err.Error())
		}
		s := &asyncioServer{ln: ln, loop: loop, cb: cb}
		s.startAccept()
		return s, nil
	}
	return &generatorObject{qual: "start_server", body: fromTop(body), ret: None, isCoro: true}
}

// startAccept launches the accept goroutine, which hands each connection to the
// loop through callSoon so the handler is built and scheduled on the loop
// goroutine. addPending keeps the loop alive while the goroutine blocks on Accept.
func (s *asyncioServer) startAccept() {
	if s.accepting {
		return
	}
	s.accepting = true
	s.serving = true
	s.loop.addPending()
	go func() {
		defer s.loop.donePending()
		for {
			conn, err := s.ln.Accept()
			if err != nil {
				return
			}
			c := conn
			s.loop.callSoon(func() { s.handleConn(c) })
		}
	}()
}

// handleConn builds a reader/writer pair over an accepted connection and schedules
// the client-connected callback as a task, the per-connection work start_server
// runs on the loop.
func (s *asyncioServer) handleConn(conn net.Conn) {
	reader, err := AsyncioNewStreamReader(streamReaderDefaultLimit)
	if err != nil {
		_ = conn.Close()
		return
	}
	r := reader.(*asyncioStreamReader)
	w := newStreamWriterOverConn(conn, r, s.loop)
	startReadPump(s.loop, conn, r)
	coro, err := CallT(mainThread, s.cb, []Object{r, w})
	if err != nil {
		return
	}
	if g, ok := coro.(*generatorObject); ok && g.isCoro {
		_, _ = scheduleTask(coro, s.loop, "")
	}
}

// serveForeverCoro is the coroutine serve_forever returns. It parks until the
// awaiting task is cancelled, the way CPython's serve_forever runs until its task
// is cancelled; close alone does not end it.
func (s *asyncioServer) serveForeverCoro() Object {
	body := func(y Yielder) (Object, error) {
		if s.closed {
			return nil, Raise(RuntimeError, "server is closed")
		}
		loop := s.loop
		fut := &asyncFuture{loop: loop}
		_, err := y.YieldFrom(&futureAwait{f: fut})
		return None, err
	}
	return &generatorObject{qual: "Server.serve_forever", body: fromTop(body), ret: None, isCoro: true}
}

// waitClosedCoro is the coroutine wait_closed returns, parking until close runs.
func (s *asyncioServer) waitClosedCoro() Object {
	body := func(y Yielder) (Object, error) {
		if s.closed {
			return None, nil
		}
		loop := s.loop
		fut := &asyncFuture{loop: loop}
		s.closeWaiters = append(s.closeWaiters, fut)
		_, err := y.YieldFrom(&futureAwait{f: fut})
		return None, err
	}
	return &generatorObject{qual: "Server.wait_closed", body: fromTop(body), ret: None, isCoro: true}
}

// closeServer stops accepting and closes the listener, then resolves any
// wait_closed waiter. The accept goroutine ends when Accept returns the
// listener-closed error.
func (s *asyncioServer) closeServer() {
	if s.closed {
		return
	}
	s.closed = true
	s.serving = false
	_ = s.ln.Close()
	for _, f := range s.closeWaiters {
		if !f.isCancelled() {
			f.setResult(None)
		}
	}
	s.closeWaiters = nil
}

// asyncioServerMethod dispatches the Server method surface.
func asyncioServerMethod(s *asyncioServer, name string, args []Object) (Object, error) {
	switch name {
	case "serve_forever":
		if len(args) != 0 {
			return nil, Raise(TypeError, "serve_forever() takes 1 positional argument but %d were given", len(args)+1)
		}
		return s.serveForeverCoro(), nil
	case "wait_closed":
		if len(args) != 0 {
			return nil, Raise(TypeError, "wait_closed() takes 1 positional argument but %d were given", len(args)+1)
		}
		return s.waitClosedCoro(), nil
	case "close":
		if len(args) != 0 {
			return nil, Raise(TypeError, "close() takes 1 positional argument but %d were given", len(args)+1)
		}
		s.closeServer()
		return None, nil
	case "is_serving":
		if len(args) != 0 {
			return nil, Raise(TypeError, "is_serving() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(s.serving), nil
	case "get_loop":
		if len(args) != 0 {
			return nil, Raise(TypeError, "get_loop() takes 1 positional argument but %d were given", len(args)+1)
		}
		return s.loop, nil
	case "start_serving":
		if len(args) != 0 {
			return nil, Raise(TypeError, "start_serving() takes 1 positional argument but %d were given", len(args)+1)
		}
		s.startAccept()
		return None, nil
	case "__aenter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aenter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return s.aenterCoro(), nil
	case "__aexit__":
		return s.aexitCoro(), nil
	}
	return nil, noAttr(s, name)
}

// aenter and aexit make Server a native async context manager, the path
// AsyncWithEnterT takes for objects that are not Python instances: entering hands
// back the server itself, exiting closes it and awaits wait_closed.
func (s *asyncioServer) aenter(t *Thread) (Object, error)               { return s.aenterCoro(), nil }
func (s *asyncioServer) aexit(t *Thread, args []Object) (Object, error) { return s.aexitCoro(), nil }

// aenterCoro is __aenter__: async with server hands back the server itself,
// matching CPython where the context manager value is the Server.
func (s *asyncioServer) aenterCoro() Object {
	body := func(y Yielder) (Object, error) { return s, nil }
	return &generatorObject{qual: "Server.__aenter__", body: fromTop(body), ret: None, isCoro: true}
}

// aexitCoro is __aexit__: it closes the server and awaits wait_closed, the
// close-then-wait CPython's Server.__aexit__ performs. closeServer resolves any
// waiter and marks the server closed synchronously, so the wait_closed it then
// performs returns at once with None; the coroutine returns None so the context
// manager never suppresses an exception.
func (s *asyncioServer) aexitCoro() Object {
	body := func(y Yielder) (Object, error) {
		s.closeServer()
		return None, nil
	}
	return &generatorObject{qual: "Server.__aexit__", body: fromTop(body), ret: None, isCoro: true}
}

// asyncioServerSockets returns the Server.sockets list: one wrapper per bound
// listener, exposing getsockname the way a socket does.
func asyncioServerSockets(s *asyncioServer) Object {
	if s.closed {
		return NewTuple(nil)
	}
	return NewTuple([]Object{&asyncioSocket{addr: s.ln.Addr()}})
}

// asyncioSocket is the minimal socket view Server.sockets hands back: enough to
// read the bound address, the getsockname a caller uses to learn the port a
// port-zero bind chose.
type asyncioSocket struct {
	addr net.Addr
}

func (s *asyncioSocket) TypeName() string { return "socket" }

func asyncioSocketMethod(s *asyncioSocket, name string, args []Object) (Object, error) {
	switch name {
	case "getsockname":
		if len(args) != 0 {
			return nil, Raise(TypeError, "getsockname() takes 1 positional argument but %d were given", len(args)+1)
		}
		if tcp, ok := s.addr.(*net.TCPAddr); ok {
			return NewTuple([]Object{NewStr(tcp.IP.String()), NewInt(int64(tcp.Port))}), nil
		}
		return NewTuple([]Object{NewStr(s.addr.String()), NewInt(0)}), nil
	}
	return nil, noAttr(s, name)
}
