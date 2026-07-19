package objects

import (
	"fmt"
	"net"
	"testing"
)

// awaitCoro drives a coroutine to completion inside a running loop, returning its
// result. It is the socket tests' bridge from a plain coroutine object to a value.
func awaitCoro(y Yielder, coro Object) (Object, error) {
	return awaitObj(y, coro)
}

// freeLoopbackPort binds and immediately releases a loopback port, returning a
// number nothing is serving so a dial to it is refused.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// serverPort reads the ephemeral port a port-zero start_server bound, through
// server.sockets[0].getsockname(), the same path a Python caller uses.
func serverPort(t *testing.T, s *asyncioServer) int {
	t.Helper()
	socks := asyncioServerSockets(s)
	tup, ok := socks.(*tupleObject)
	if !ok || len(tup.elts) != 1 {
		t.Fatalf("sockets: want 1 socket, got %v", socks)
	}
	sock, ok := tup.elts[0].(*asyncioSocket)
	if !ok {
		t.Fatalf("sockets[0]: want socket, got %T", tup.elts[0])
	}
	name, err := asyncioSocketMethod(sock, "getsockname", nil)
	if err != nil {
		t.Fatalf("getsockname: %v", err)
	}
	nt := name.(*tupleObject)
	port, _ := AsInt(nt.elts[1])
	return int(port)
}

// TestSocketOpenConnectionRoundTrip stands up a real loopback echo server through
// start_server, dials it with open_connection, and checks a line written by the
// client comes back echoed. This drives the netpoller handoff end to end: the
// accept goroutine, the per-connection read pumps on both ends, and the handler
// task, all serialised onto the loop through callSoon.
func TestSocketOpenConnectionRoundTrip(t *testing.T) {
	handled := make(chan struct{}, 1)

	// handler echoes one line then closes, the classic echo server body.
	handler := NewFunc("handler", 2, func(args []Object) (Object, error) {
		reader := args[0].(*asyncioStreamReader)
		writer := args[1].(*asyncioStreamWriter)
		body := func(y Yielder) (Object, error) {
			line, err := awaitCoro(y, reader.readlineCoro())
			if err != nil {
				return nil, err
			}
			b, _ := bytesLike(line)
			if _, err := asyncioStreamWriterMethod(writer, "write", []Object{NewBytes(b)}); err != nil {
				return nil, err
			}
			if _, err := awaitCoro(y, writer.drainCoro()); err != nil {
				return nil, err
			}
			writer.close()
			if _, err := awaitCoro(y, writer.waitClosedCoro()); err != nil {
				return nil, err
			}
			select {
			case handled <- struct{}{}:
			default:
			}
			return None, nil
		}
		return NewCoroutine("handler", body), nil
	})

	var got Object
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		srvObj, err := awaitCoro(y, AsyncioStartServer(handler, "127.0.0.1", 0))
		if err != nil {
			return nil, err
		}
		srv := srvObj.(*asyncioServer)
		port := serverPort(t, srv)

		pairObj, err := awaitCoro(y, AsyncioOpenConnection("127.0.0.1", port))
		if err != nil {
			return nil, err
		}
		pair := pairObj.(*tupleObject)
		reader := pair.elts[0].(*asyncioStreamReader)
		writer := pair.elts[1].(*asyncioStreamWriter)

		if _, err := asyncioStreamWriterMethod(writer, "write", []Object{NewBytes([]byte("ping\n"))}); err != nil {
			return nil, err
		}
		if _, err := awaitCoro(y, writer.drainCoro()); err != nil {
			return nil, err
		}
		got, err = awaitCoro(y, reader.readlineCoro())
		if err != nil {
			return nil, err
		}
		writer.close()
		if _, err := awaitCoro(y, writer.waitClosedCoro()); err != nil {
			return nil, err
		}
		srv.closeServer()
		if _, err := awaitCoro(y, srv.waitClosedCoro()); err != nil {
			return nil, err
		}
		return None, nil
	})

	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	wantBytes(t, got, "ping\n")
	select {
	case <-handled:
	default:
		t.Errorf("handler did not run")
	}
}

// TestSocketServerAsyncWith drives the Server through the async context-manager
// protocol the way `async with server:` lowers: AsyncWithEnterT awaits __aenter__
// and must hand back the server itself, and the returned __aexit__ closes it, so
// the server reports not-serving once the block exits.
func TestSocketServerAsyncWith(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		noop := NewFunc("noop", 2, func(args []Object) (Object, error) {
			return NewCoroutine("noop", func(y Yielder) (Object, error) { return None, nil }), nil
		})
		srvObj, err := awaitCoro(y, AsyncioStartServer(noop, "127.0.0.1", 0))
		if err != nil {
			return nil, err
		}
		srv := srvObj.(*asyncioServer)

		aexit, entered, err := AsyncWithEnterT(mainThread, y, srv)
		if err != nil {
			return nil, err
		}
		if entered != Object(srv) {
			t.Errorf("__aenter__: want the server itself, got %v", entered)
		}
		if !srv.serving {
			t.Errorf("inside async with: want serving")
		}

		// __aexit__ returns a coroutine; awaiting it closes the server.
		exitCoro, err := CallT(mainThread, aexit, []Object{None, None, None})
		if err != nil {
			return nil, err
		}
		if _, err := awaitCoro(y, exitCoro); err != nil {
			return nil, err
		}
		if srv.serving {
			t.Errorf("after async with: want not serving")
		}
		if _, err := awaitCoro(y, srv.waitClosedCoro()); err != nil {
			return nil, err
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestSocketConcurrentEcho is the streams exit-gate smoke test: one event loop
// stands up a loopback echo server and drives many clients at once, each writing
// a payload unique to it and asserting that exact payload comes back. Every part
// of the netpoller handoff runs under contention here at once: the accept
// goroutine feeding connections through callSoon, one read pump per connection on
// each end, a handler task per accepted connection, and a client task per dial,
// all serialised onto the single loop goroutine. Run under -race it proves the
// handoff has no data race and no lost or crossed bytes.
func TestSocketConcurrentEcho(t *testing.T) {
	const clients = 200

	// handler echoes one line back, finishes the write side, and closes.
	handler := NewFunc("handler", 2, func(args []Object) (Object, error) {
		reader := args[0].(*asyncioStreamReader)
		writer := args[1].(*asyncioStreamWriter)
		body := func(y Yielder) (Object, error) {
			line, err := awaitCoro(y, reader.readlineCoro())
			if err != nil {
				return nil, err
			}
			b, _ := bytesLike(line)
			if _, err := asyncioStreamWriterMethod(writer, "write", []Object{NewBytes(b)}); err != nil {
				return nil, err
			}
			if _, err := awaitCoro(y, writer.drainCoro()); err != nil {
				return nil, err
			}
			writer.close()
			if _, err := awaitCoro(y, writer.waitClosedCoro()); err != nil {
				return nil, err
			}
			return None, nil
		}
		return NewCoroutine("handler", body), nil
	})

	// mismatches records any client that did not read back its own payload. The
	// clients all run on the loop goroutine, so appending needs no lock.
	var mismatches []string

	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		srvObj, err := awaitCoro(y, AsyncioStartServer(handler, "127.0.0.1", 0))
		if err != nil {
			return nil, err
		}
		srv := srvObj.(*asyncioServer)
		port := serverPort(t, srv)

		tasks := make([]Object, 0, clients)
		for i := 0; i < clients; i++ {
			want := fmt.Sprintf("client-%d\n", i)
			body := func(y Yielder) (Object, error) {
				pairObj, err := awaitCoro(y, AsyncioOpenConnection("127.0.0.1", port))
				if err != nil {
					return nil, err
				}
				pair := pairObj.(*tupleObject)
				reader := pair.elts[0].(*asyncioStreamReader)
				writer := pair.elts[1].(*asyncioStreamWriter)

				if _, err := asyncioStreamWriterMethod(writer, "write", []Object{NewBytes([]byte(want))}); err != nil {
					return nil, err
				}
				if _, err := awaitCoro(y, writer.drainCoro()); err != nil {
					return nil, err
				}
				got, err := awaitCoro(y, reader.readlineCoro())
				if err != nil {
					return nil, err
				}
				gb, _ := bytesLike(got)
				if string(gb) != want {
					mismatches = append(mismatches, fmt.Sprintf("want %q, got %q", want, string(gb)))
				}
				writer.close()
				if _, err := awaitCoro(y, writer.waitClosedCoro()); err != nil {
					return nil, err
				}
				return None, nil
			}
			task, err := AsyncioCreateTask(NewCoroutine("client", body), "")
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, task)
		}

		if _, err := awaitCoro(y, mustGather(tasks)); err != nil {
			return nil, err
		}
		srv.closeServer()
		if _, err := awaitCoro(y, srv.waitClosedCoro()); err != nil {
			return nil, err
		}
		return None, nil
	})

	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(mismatches) != 0 {
		t.Fatalf("%d/%d clients mismatched, first: %s", len(mismatches), clients, mismatches[0])
	}
}

// mustGather is the gather the concurrent test awaits; a builder that panics on
// the impossible no-running-loop error keeps the test body flat.
func mustGather(tasks []Object) Object {
	g, err := AsyncioGather(tasks, false)
	if err != nil {
		panic(err)
	}
	return g
}

// TestSocketConnectionRefused checks open_connection to a closed port surfaces
// ConnectionRefusedError, the dial-failure path.
func TestSocketConnectionRefused(t *testing.T) {
	srvPort := freeLoopbackPort(t)
	var kind string
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		_, err := awaitCoro(y, AsyncioOpenConnection("127.0.0.1", srvPort))
		if err != nil {
			kind = coroExcKind(err)
			return None, nil
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if kind != "ConnectionRefusedError" && kind != "OSError" {
		t.Fatalf("want ConnectionRefusedError, got %q", kind)
	}
}

// TestSocketServerIsServing checks a fresh server reports serving and stops
// reporting it after close.
func TestSocketServerIsServing(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		noop := NewFunc("noop", 2, func(args []Object) (Object, error) {
			return NewCoroutine("noop", func(y Yielder) (Object, error) { return None, nil }), nil
		})
		srvObj, err := awaitCoro(y, AsyncioStartServer(noop, "127.0.0.1", 0))
		if err != nil {
			return nil, err
		}
		srv := srvObj.(*asyncioServer)
		if !srv.serving {
			t.Errorf("is_serving: want true after start")
		}
		srv.closeServer()
		if srv.serving {
			t.Errorf("is_serving: want false after close")
		}
		if _, err := awaitCoro(y, srv.waitClosedCoro()); err != nil {
			return nil, err
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
}
