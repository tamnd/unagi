package objects

import "testing"

// TestDequeCopy checks deque.__copy__ makes an independent shallow copy that
// preserves the bound, the hook copy.copy reaches. Before it, __copy__ was
// absent so the bound read and copy.copy both raised.
func TestDequeCopy(t *testing.T) {
	d := NewDeque([]Object{NewInt(1), NewInt(2), NewInt(3)}, 5)

	m, err := LoadAttr(d, "__copy__")
	if err != nil {
		t.Fatalf("deque.__copy__: %v", err)
	}
	cp, err := Call(m, nil)
	if err != nil {
		t.Fatalf("__copy__(): %v", err)
	}
	dc, ok := cp.(*dequeObject)
	if !ok {
		t.Fatalf("__copy__ returned %T; want a deque", cp)
	}
	if len(dc.elts) != 3 || dc.maxlen != 5 {
		t.Fatalf("copy elts=%v maxlen=%d; want 3 elements, maxlen 5", dc.elts, dc.maxlen)
	}
	// The copy is independent: appending to the original does not touch it.
	orig := d.(*dequeObject)
	orig.elts = append(orig.elts, NewInt(4))
	if len(dc.elts) != 3 {
		t.Fatalf("copy elts=%v after original append; want it unchanged", dc.elts)
	}
}
