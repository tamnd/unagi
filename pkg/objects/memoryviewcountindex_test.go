package objects

import "testing"

// Expected values and messages probed on CPython 3.14.6.

// TestMemoryviewCountIndexFlat checks that count and index over a plain
// one-dimensional view read the flat element run.
func TestMemoryviewCountIndexFlat(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte{1, 2, 3, 2}))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	got, err := CallMethod(mv, "count", []Object{NewInt(2)})
	if err != nil {
		t.Fatalf("count(2): %v", err)
	}
	if n, _ := AsInt(got); n != 2 {
		t.Fatalf("count(2) = %d, want 2", n)
	}
	got, err = CallMethod(mv, "index", []Object{NewInt(2)})
	if err != nil {
		t.Fatalf("index(2): %v", err)
	}
	if n, _ := AsInt(got); n != 1 {
		t.Fatalf("index(2) = %d, want 1", n)
	}
	if _, err := CallMethod(mv, "index", []Object{NewInt(9)}); err == nil {
		t.Fatal("index(9) did not raise")
	}
}

// TestMemoryviewCountIndexMultiDim checks that count and index decline a
// multi-dimensional view, each with its own NotImplementedError wording.
func TestMemoryviewCountIndexMultiDim(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte{1, 2, 3, 4}))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	md, err := CallMethodKw(mv, "cast", []Object{NewStr("b")},
		[]string{"shape"}, []Object{NewList([]Object{NewInt(2), NewInt(2)})})
	if err != nil {
		t.Fatalf("cast(b, shape=[2,2]): %v", err)
	}
	_, err = CallMethod(md, "count", []Object{NewInt(1)})
	if err == nil {
		t.Fatal("count on multi-dim view did not raise")
	}
	if ex, ok := err.(*Exception); !ok || ex.Kind != "NotImplementedError" ||
		ex.Text() != "multi-dimensional sub-views are not implemented" {
		t.Fatalf("count multi-dim error = %v", err)
	}
	_, err = CallMethod(md, "index", []Object{NewInt(1)})
	if err == nil {
		t.Fatal("index on multi-dim view did not raise")
	}
	if ex, ok := err.(*Exception); !ok || ex.Kind != "NotImplementedError" ||
		ex.Text() != "multi-dimensional lookup is not implemented" {
		t.Fatalf("index multi-dim error = %v", err)
	}
}

// TestMemoryviewIndexBoundPrecedence checks that index converts its start bound
// before the multi-dimensional decline, so a non-integer bound is the
// slice-index TypeError even on a multi-dimensional view.
func TestMemoryviewIndexBoundPrecedence(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte{1, 2, 3, 4}))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	md, err := CallMethodKw(mv, "cast", []Object{NewStr("b")},
		[]string{"shape"}, []Object{NewList([]Object{NewInt(2), NewInt(2)})})
	if err != nil {
		t.Fatalf("cast(b, shape=[2,2]): %v", err)
	}
	_, err = CallMethod(md, "index", []Object{NewInt(1), NewStr("x")})
	if err == nil {
		t.Fatal("index with a str bound did not raise")
	}
	if ex, ok := err.(*Exception); !ok || ex.Kind != TypeError ||
		ex.Text() != "slice indices must be integers or have an __index__ method" {
		t.Fatalf("index bad-bound error = %v", err)
	}
}
