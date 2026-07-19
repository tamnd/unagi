package objects

import "testing"

// callWriter invokes a writer method positionally, failing the test on error.
func callWriter(t *testing.T, w *asyncioStreamWriter, name string, args ...Object) Object {
	t.Helper()
	got, err := asyncioStreamWriterMethod(w, name, args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return got
}

// TestStreamWriterWriteReadPair checks bytes written to a writer land in the
// paired reader, and close feeds the reader EOF so read(-1) returns them all.
func TestStreamWriterWriteReadPair(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	w := newStreamWriterTo(r)
	callWriter(t, w, "write", NewBytes([]byte("hello ")))
	callWriter(t, w, "write", NewBytes([]byte("world")))
	w.close()
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return awaitObj(y, r.readCoro(-1))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantBytes(t, got, "hello world")
}

// TestStreamWriterWritelines checks writelines writes each chunk of an iterable in
// order.
func TestStreamWriterWritelines(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	w := newStreamWriterTo(r)
	lines := NewList([]Object{
		NewBytes([]byte("a\n")),
		NewBytes([]byte("b\n")),
		NewBytes([]byte("c\n")),
	})
	callWriter(t, w, "writelines", lines)
	w.close()
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return awaitObj(y, r.readCoro(-1))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantBytes(t, got, "a\nb\nc\n")
}

// TestStreamWriterDrain checks drain is an immediate no-op await on an in-memory
// transport that never pauses.
func TestStreamWriterDrain(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	w := newStreamWriterTo(r)
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		callWriter(t, w, "write", NewBytes([]byte("x")))
		return awaitObj(y, w.drainCoro())
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got != None {
		t.Fatalf("drain() = %s, want None", Repr(got))
	}
}

// TestStreamWriterWaitClosed checks wait_closed parks until close runs, then
// returns None.
func TestStreamWriterWaitClosed(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	w := newStreamWriterTo(r)
	var order []string
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		waiter := NewCoroutine("waiter", func(y Yielder) (Object, error) {
			if _, err := awaitObj(y, w.waitClosedCoro()); err != nil {
				return nil, err
			}
			order = append(order, "closed")
			return None, nil
		})
		task, err := AsyncioCreateTask(waiter, "")
		if err != nil {
			return nil, err
		}
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		order = append(order, "close")
		w.close()
		return awaitObj(y, task)
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"close", "closed"}
	if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

// TestStreamWriterIsClosing checks is_closing flips once close runs.
func TestStreamWriterIsClosing(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	w := newStreamWriterTo(r)
	if Truth(callWriter(t, w, "is_closing")) {
		t.Fatalf("is_closing before close = True, want False")
	}
	if !Truth(callWriter(t, w, "can_write_eof")) {
		t.Fatalf("can_write_eof = False, want True")
	}
	w.close()
	if !Truth(callWriter(t, w, "is_closing")) {
		t.Fatalf("is_closing after close = False, want True")
	}
}

// TestStreamWriterWriteEOF checks write_eof feeds the reader EOF so at_eof reports
// true once the buffer drains.
func TestStreamWriterWriteEOF(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	w := newStreamWriterTo(r)
	callWriter(t, w, "write", NewBytes([]byte("bye")))
	callWriter(t, w, "write_eof")
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return awaitObj(y, r.readCoro(-1))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantBytes(t, got, "bye")
	if !r.atEOF() {
		t.Fatalf("at_eof after write_eof and drain = False, want True")
	}
}

// TestStreamWriterGetExtraInfo checks get_extra_info returns the default for an
// in-memory transport that carries no socket detail.
func TestStreamWriterGetExtraInfo(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	w := newStreamWriterTo(r)
	if got := callWriter(t, w, "get_extra_info", NewStr("peername")); got != None {
		t.Fatalf("get_extra_info('peername') = %s, want None", Repr(got))
	}
	def := NewStr("fallback")
	if got := callWriter(t, w, "get_extra_info", NewStr("peername"), def); got != def {
		t.Fatalf("get_extra_info with default = %s, want fallback", Repr(got))
	}
}
