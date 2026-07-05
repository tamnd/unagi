package objects

import "testing"

// drain pulls every value a generator yields, returning them and any error the
// generator raised on the way out.
func drain(g Object) ([]Object, error) {
	it, err := Iter(g)
	if err != nil {
		return nil, err
	}
	var out []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return out, err
		}
		if !ok {
			return out, nil
		}
		out = append(out, v)
	}
}

func TestGeneratorIterates(t *testing.T) {
	g := NewGenerator("count", func(y Yielder) (Object, error) {
		for i := int64(0); i < 3; i++ {
			if _, err := y.Yield(NewInt(i)); err != nil {
				return nil, err
			}
		}
		return None, nil
	})
	vals, err := drain(g)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("got %d values, want 3", len(vals))
	}
	for i, v := range vals {
		if n, _ := AsInt(v); n != int64(i) {
			t.Fatalf("value %d = %v, want %d", i, Str(v), i)
		}
	}
	// A second pull past the end stays exhausted.
	if _, ok, err := g.(*generatorObject).Next(); ok || err != nil {
		t.Fatalf("Next after exhaustion = ok %v err %v", ok, err)
	}
}

func TestGeneratorSend(t *testing.T) {
	// Echo the sent value back out; the first yield produces 0.
	g := NewGenerator("echo", func(y Yielder) (Object, error) {
		cur := NewInt(0)
		for {
			sent, err := y.Yield(cur)
			if err != nil {
				return nil, err
			}
			cur = sent
		}
	})
	first, err := CallMethod(g, "send", []Object{None})
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	if n, _ := AsInt(first); n != 0 {
		t.Fatalf("first yield = %v, want 0", Str(first))
	}
	got, err := CallMethod(g, "send", []Object{NewInt(42)})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if n, _ := AsInt(got); n != 42 {
		t.Fatalf("send echo = %v, want 42", Str(got))
	}
}

func TestGeneratorSendBeforeStart(t *testing.T) {
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		_, err := y.Yield(None)
		return None, err
	})
	_, err := CallMethod(g, "send", []Object{NewInt(1)})
	e, ok := err.(*Exception)
	if !ok || e.Kind != TypeError {
		t.Fatalf("send before start err = %v, want TypeError", err)
	}
	if e.Text() != "can't send non-None value to a just-started generator" {
		t.Fatalf("wrong message: %q", e.Text())
	}
}

func TestGeneratorReturnValue(t *testing.T) {
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		if _, err := y.Yield(NewInt(1)); err != nil {
			return nil, err
		}
		return NewInt(99), nil
	})
	if _, err := NextValue([]Object{g}); err != nil {
		t.Fatalf("first next: %v", err)
	}
	_, err := NextValue([]Object{g})
	e, ok := err.(*Exception)
	if !ok || e.Kind != "StopIteration" {
		t.Fatalf("completion err = %v, want StopIteration", err)
	}
	if len(e.Args) != 1 {
		t.Fatalf("StopIteration args = %v, want the return value", e.Args)
	}
	if n, _ := AsInt(e.Args[0]); n != 99 {
		t.Fatalf("StopIteration value = %v, want 99", Str(e.Args[0]))
	}
}

func TestGeneratorClose(t *testing.T) {
	closed := false
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		defer func() { closed = true }()
		for {
			if _, err := y.Yield(NewInt(1)); err != nil {
				return nil, err
			}
		}
	})
	if _, err := NextValue([]Object{g}); err != nil {
		t.Fatalf("prime: %v", err)
	}
	r, err := CallMethod(g, "close", nil)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if r != None {
		t.Fatalf("close returned %v, want None", Str(r))
	}
	if !closed {
		t.Fatal("body did not run its cleanup on close")
	}
	// A second close on an exhausted generator is a no-op.
	if _, err := CallMethod(g, "close", nil); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// A StopIteration escaping the body becomes the PEP 479 RuntimeError carrying
// the original as both cause and context with context suppressed.
func TestGeneratorStopIterationConverts(t *testing.T) {
	orig := &Exception{Kind: "StopIteration", Args: []Object{NewStr("escaped")}}
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		return nil, orig
	})
	_, err := NextValue([]Object{g})
	e, ok := err.(*Exception)
	if !ok {
		t.Fatalf("err = %T, want *Exception", err)
	}
	if e.Kind != "RuntimeError" {
		t.Fatalf("Kind = %q, want RuntimeError", e.Kind)
	}
	if len(e.Args) == 0 {
		t.Fatal("RuntimeError carries no message")
	}
	if s, _ := AsStr(e.Args[0]); s != "generator raised StopIteration" {
		t.Fatalf("message = %q, want generator raised StopIteration", s)
	}
	if e.Cause != orig || e.Context != orig {
		t.Fatalf("cause/context = %v/%v, want the original StopIteration", e.Cause, e.Context)
	}
	if !e.SuppressContext {
		t.Fatal("SuppressContext = false, want true")
	}
}

// close() converts a StopIteration raised while handling GeneratorExit into the
// same PEP 479 RuntimeError instead of treating it as a clean close.
func TestGeneratorCloseStopIterationConverts(t *testing.T) {
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		if _, err := y.Yield(NewInt(1)); err != nil {
			// The injected GeneratorExit arrives here; raising StopIteration in
			// response must convert, not close cleanly.
			return nil, &Exception{Kind: "StopIteration", Args: []Object{NewStr("during close")}}
		}
		return None, nil
	})
	if _, err := NextValue([]Object{g}); err != nil {
		t.Fatalf("prime: %v", err)
	}
	_, err := CallMethod(g, "close", nil)
	e, ok := err.(*Exception)
	if !ok || e.Kind != "RuntimeError" {
		t.Fatalf("close err = %v, want RuntimeError", err)
	}
}

func TestGeneratorThrow(t *testing.T) {
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		_, err := y.Yield(NewInt(1))
		if err != nil {
			if e, ok := err.(*Exception); ok && e.Kind == ValueError {
				// Swallow the thrown ValueError and yield a marker.
				return y.Yield(NewStr("caught"))
			}
			return nil, err
		}
		return None, nil
	})
	if _, err := NextValue([]Object{g}); err != nil {
		t.Fatalf("prime: %v", err)
	}
	got, err := CallMethod(g, "throw", []Object{Raise(ValueError, "boom")})
	if err != nil {
		t.Fatalf("throw: %v", err)
	}
	if s, _ := AsStr(got); s != "caught" {
		t.Fatalf("throw resumed to %v, want caught", Str(got))
	}
}

func TestGeneratorThrowPropagates(t *testing.T) {
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		_, err := y.Yield(NewInt(1))
		return None, err
	})
	if _, err := NextValue([]Object{g}); err != nil {
		t.Fatalf("prime: %v", err)
	}
	_, err := CallMethod(g, "throw", []Object{Raise(ValueError, "boom")})
	e, ok := err.(*Exception)
	if !ok || e.Kind != ValueError {
		t.Fatalf("throw err = %v, want ValueError", err)
	}
}

func TestYieldFromIterable(t *testing.T) {
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		ret, err := y.YieldFrom(NewList([]Object{NewInt(1), NewInt(2), NewInt(3)}))
		if err != nil {
			return nil, err
		}
		if ret != None {
			t.Fatalf("plain iterable yield-from value = %v, want None", Str(ret))
		}
		return None, nil
	})
	vals, err := drain(g)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("yield from list gave %d values, want 3", len(vals))
	}
}

func TestYieldFromGeneratorReturnValue(t *testing.T) {
	sub := func() Object {
		return NewGenerator("sub", func(y Yielder) (Object, error) {
			if _, err := y.Yield(NewInt(1)); err != nil {
				return nil, err
			}
			return NewStr("sub-done"), nil
		})
	}
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		ret, err := y.YieldFrom(sub())
		if err != nil {
			return nil, err
		}
		return y.Yield(ret)
	})
	vals, err := drain(g)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(vals) != 2 {
		t.Fatalf("got %d values, want 2", len(vals))
	}
	if s, _ := AsStr(vals[1]); s != "sub-done" {
		t.Fatalf("delegated return value = %v, want sub-done", Str(vals[1]))
	}
}

func TestYieldFromUserIteratorStopValue(t *testing.T) {
	c := mkclass(t, "It")
	c.setAttr("__iter__", mkfn("It.__iter__", 1, func(args []Object) (Object, error) {
		return args[0], nil
	}))
	c.setAttr("__next__", mkfn("It.__next__", 1, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		i, _ := AsInt(self.dict["i"])
		if i >= 2 {
			return nil, &Exception{Kind: "StopIteration", Args: []Object{NewStr("carried")}}
		}
		self.dict["i"] = NewInt(i + 1)
		return NewInt(i + 1), nil
	}))
	x := inst(c)
	x.dict["i"] = NewInt(0)
	g := NewGenerator("g", func(y Yielder) (Object, error) {
		ret, err := y.YieldFrom(x)
		if err != nil {
			return nil, err
		}
		return y.Yield(ret)
	})
	vals, err := drain(g)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("got %d values, want 3", len(vals))
	}
	if s, _ := AsStr(vals[2]); s != "carried" {
		t.Fatalf("yield-from value = %v, want carried", Str(vals[2]))
	}
}

func TestNextOnNonIterator(t *testing.T) {
	_, err := NextValue([]Object{NewList([]Object{NewInt(1)})})
	e, ok := err.(*Exception)
	if !ok || e.Kind != TypeError {
		t.Fatalf("next on list = %v, want TypeError", err)
	}
	if e.Text() != "'list' object is not an iterator" {
		t.Fatalf("wrong message: %q", e.Text())
	}
}

func TestNextDefault(t *testing.T) {
	g := NewGenerator("g", func(y Yielder) (Object, error) { return None, nil })
	// Exhaust it, then the default rides out instead of StopIteration.
	if _, err := NextValue([]Object{g}); err != nil {
		e, ok := err.(*Exception)
		if !ok || e.Kind != "StopIteration" {
			t.Fatalf("first next = %v", err)
		}
	}
	v, err := NextValue([]Object{g, NewStr("fallback")})
	if err != nil {
		t.Fatalf("next with default: %v", err)
	}
	if s, _ := AsStr(v); s != "fallback" {
		t.Fatalf("default = %v, want fallback", Str(v))
	}
}

func TestGeneratorTypeAndRepr(t *testing.T) {
	g := NewGenerator("myqual", func(y Yielder) (Object, error) { return None, nil })
	if g.TypeName() != "generator" {
		t.Fatalf("type = %q, want generator", g.TypeName())
	}
	r := Repr(g)
	if want := "<generator object myqual at 0x"; len(r) < len(want) || r[:len(want)] != want {
		t.Fatalf("repr = %q, want prefix %q", r, want)
	}
}

func TestStopIterationValueAttr(t *testing.T) {
	v, err := LoadAttr(&Exception{Kind: "StopIteration", Args: []Object{NewInt(7)}}, "value")
	if err != nil {
		t.Fatalf("value attr: %v", err)
	}
	if n, _ := AsInt(v); n != 7 {
		t.Fatalf("StopIteration.value = %v, want 7", Str(v))
	}
	empty, err := LoadAttr(&Exception{Kind: "StopIteration"}, "value")
	if err != nil {
		t.Fatalf("empty value attr: %v", err)
	}
	if empty != None {
		t.Fatalf("StopIteration().value = %v, want None", Str(empty))
	}
	// Non-StopIteration exceptions have no value attribute.
	if _, err := LoadAttr(Raise(ValueError, "x"), "value"); err == nil {
		t.Fatal("ValueError.value should raise AttributeError")
	}
}
