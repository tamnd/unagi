package objects

import "testing"

// TestDequeMethodReadsBindReceiver checks a deque method reads back as a bound
// method that carries the deque through __self__ and names __qualname__ after
// deque, the way CPython exposes a bound built-in method, while __name__ stays
// bare. A read used to raise AttributeError even though the fused call worked.
func TestDequeMethodReadsBindReceiver(t *testing.T) {
	d := NewDeque([]Object{NewInt(1), NewInt(2)}, -1)
	for _, name := range []string{"append", "appendleft", "pop", "rotate", "count", "__reversed__"} {
		m, err := LoadAttr(d, name)
		if err != nil {
			t.Fatalf("deque.%s: %v", name, err)
		}
		self, err := LoadAttr(m, "__self__")
		if err != nil {
			t.Fatalf("deque.%s.__self__: %v", name, err)
		}
		if self != d {
			t.Fatalf("deque.%s.__self__ = %v; want the deque", name, self)
		}
		if got := loadStr(t, m, "__qualname__"); got != "deque."+name {
			t.Fatalf("deque.%s.__qualname__ = %q; want %q", name, got, "deque."+name)
		}
		if got := loadStr(t, m, "__name__"); got != name {
			t.Fatalf("deque.%s.__name__ = %q; want %q", name, got, name)
		}
	}
}

// TestDequeMethodReadCalls checks the value a deque method read yields dispatches
// back to the deque method, so a read then a call matches the fused call.
func TestDequeMethodReadCalls(t *testing.T) {
	d := NewDeque([]Object{NewInt(1)}, -1)
	push, err := LoadAttr(d, "append")
	if err != nil {
		t.Fatalf("deque.append: %v", err)
	}
	if _, err := Call(push, []Object{NewInt(2)}); err != nil {
		t.Fatalf("append(2): %v", err)
	}
	dq := d.(*dequeObject)
	if len(dq.elts) != 2 || dq.elts[1] != Object(NewInt(2)) {
		t.Fatalf("after read-then-call append, elts = %v; want [1 2]", dq.elts)
	}
}

// TestDequeMissingMethodStillErrors guards the boundary: a name the deque does
// not carry still reports the attribute miss rather than binding a callable.
func TestDequeMissingMethodStillErrors(t *testing.T) {
	d := NewDeque(nil, -1)
	if _, err := LoadAttr(d, "sort"); err == nil {
		t.Fatalf("deque.sort = no error; want AttributeError")
	}
}
