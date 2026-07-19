package objects

import "testing"

// newStreamReader builds a StreamReader for a test, failing on the ValueError
// path so the caller works with a concrete reader.
func newStreamReader(t *testing.T, limit int) *asyncioStreamReader {
	t.Helper()
	r, err := AsyncioNewStreamReader(limit)
	if err != nil {
		t.Fatalf("StreamReader(%d): %v", limit, err)
	}
	return r.(*asyncioStreamReader)
}

// TestStreamReaderReadBuffered checks a read served entirely from the buffer
// returns up to n bytes and leaves the rest.
func TestStreamReaderReadBuffered(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	if _, err := asyncioStreamReaderMethod(r, "feed_data", []Object{NewBytes([]byte("hello world"))}); err != nil {
		t.Fatalf("feed_data: %v", err)
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		first, err := awaitObj(y, r.readCoro(5))
		if err != nil {
			return nil, err
		}
		wantBytes(t, first, "hello")
		return awaitObj(y, r.readCoro(100))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantBytes(t, got, " world")
}

// TestStreamReaderReadBlocksUntilFed checks a read on an empty stream parks until
// another coroutine feeds it, then returns the fed bytes.
func TestStreamReaderReadBlocksUntilFed(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	var order []string
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		reader := NewCoroutine("reader", func(y Yielder) (Object, error) {
			data, err := awaitObj(y, r.readCoro(100))
			if err != nil {
				return nil, err
			}
			order = append(order, "read "+string(mustBytes(t, data)))
			return None, nil
		})
		task, err := AsyncioCreateTask(reader, "")
		if err != nil {
			return nil, err
		}
		// Let the reader park on an empty buffer before the feed arrives.
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		order = append(order, "feed")
		if _, err := asyncioStreamReaderMethod(r, "feed_data", []Object{NewBytes([]byte("ping"))}); err != nil {
			return nil, err
		}
		return awaitObj(y, task)
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"feed", "read ping"}
	if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("interleave = %v, want %v", order, want)
	}
}

// TestStreamReaderReadAllAcrossFeeds checks read(-1) collects every fed chunk and
// returns once feed_eof arrives.
func TestStreamReaderReadAllAcrossFeeds(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		reader := NewCoroutine("reader", func(y Yielder) (Object, error) {
			return awaitObj(y, r.readCoro(-1))
		})
		task, err := AsyncioCreateTask(reader, "")
		if err != nil {
			return nil, err
		}
		for _, chunk := range []string{"one", "two", "three"} {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
			if _, err := asyncioStreamReaderMethod(r, "feed_data", []Object{NewBytes([]byte(chunk))}); err != nil {
				return nil, err
			}
		}
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		if _, err := asyncioStreamReaderMethod(r, "feed_eof", nil); err != nil {
			return nil, err
		}
		return awaitObj(y, task)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantBytes(t, got, "onetwothree")
}

// TestStreamReaderAtEOF checks feed_eof makes at_eof report true once the buffer
// drains and a read past EOF returns empty bytes.
func TestStreamReaderAtEOF(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	if _, err := asyncioStreamReaderMethod(r, "feed_data", []Object{NewBytes([]byte("hi"))}); err != nil {
		t.Fatalf("feed_data: %v", err)
	}
	if _, err := asyncioStreamReaderMethod(r, "feed_eof", nil); err != nil {
		t.Fatalf("feed_eof: %v", err)
	}
	eof, err := asyncioStreamReaderMethod(r, "at_eof", nil)
	if err != nil {
		t.Fatalf("at_eof: %v", err)
	}
	if Truth(eof) {
		t.Fatalf("at_eof before drain = True, want False")
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		if _, err := awaitObj(y, r.readCoro(-1)); err != nil {
			return nil, err
		}
		return awaitObj(y, r.readCoro(10))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantBytes(t, got, "")
	eof, err = asyncioStreamReaderMethod(r, "at_eof", nil)
	if err != nil {
		t.Fatalf("at_eof: %v", err)
	}
	if !Truth(eof) {
		t.Fatalf("at_eof after drain = False, want True")
	}
}

// TestStreamReaderSetException checks set_exception makes the next read re-raise
// the stored exception.
func TestStreamReaderSetException(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	boom := errorObject(Raise(ValueError, "boom"))
	if _, err := asyncioStreamReaderMethod(r, "set_exception", []Object{boom}); err != nil {
		t.Fatalf("set_exception: %v", err)
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return awaitObj(y, r.readCoro(10))
	})
	if _, err := AsyncioRun(main); coroExcKind(err) != "ValueError" {
		t.Fatalf("read after set_exception = %v, want ValueError", err)
	}
	exc, err := asyncioStreamReaderMethod(r, "exception", nil)
	if err != nil {
		t.Fatalf("exception: %v", err)
	}
	if exc != boom {
		t.Fatalf("exception() = %s, want the stored ValueError", Repr(exc))
	}
}

// TestStreamReaderTwoWaiters checks a second read starting while the first is
// still waiting is the RuntimeError CPython raises.
func TestStreamReaderTwoWaiters(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		first := NewCoroutine("first", func(y Yielder) (Object, error) {
			return awaitObj(y, r.readCoro(10))
		})
		if _, err := AsyncioCreateTask(first, ""); err != nil {
			return nil, err
		}
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		return awaitObj(y, r.readCoro(10))
	})
	if _, err := AsyncioRun(main); coroExcKind(err) != "RuntimeError" {
		t.Fatalf("second waiting read = %v, want RuntimeError", err)
	}
}

// TestStreamReaderLimitInvalid checks a non-positive limit is the ValueError the
// constructor raises.
func TestStreamReaderLimitInvalid(t *testing.T) {
	if _, err := AsyncioNewStreamReader(0); coroExcKind(err) != "ValueError" {
		t.Fatalf("StreamReader(0) = %v, want ValueError", err)
	}
	if _, err := AsyncioNewStreamReader(-1); coroExcKind(err) != "ValueError" {
		t.Fatalf("StreamReader(-1) = %v, want ValueError", err)
	}
}

// TestStreamReaderFeedAfterEOF checks feeding data after feed_eof is the
// AssertionError CPython's assert raises.
func TestStreamReaderFeedAfterEOF(t *testing.T) {
	r := newStreamReader(t, streamReaderDefaultLimit)
	if _, err := asyncioStreamReaderMethod(r, "feed_eof", nil); err != nil {
		t.Fatalf("feed_eof: %v", err)
	}
	if _, err := asyncioStreamReaderMethod(r, "feed_data", []Object{NewBytes([]byte("x"))}); coroExcKind(err) != "AssertionError" {
		t.Fatalf("feed_data after feed_eof = %v, want AssertionError", err)
	}
}

// mustBytes reads the raw bytes of an object or fails the test.
func mustBytes(t *testing.T, o Object) []byte {
	t.Helper()
	b, ok := AsBytes(o)
	if !ok {
		t.Fatalf("want bytes, got %s", Repr(o))
	}
	return b
}
