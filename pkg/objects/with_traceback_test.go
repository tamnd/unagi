package objects

import "testing"

// TestExcWithTraceback checks BaseException.with_traceback(tb): it returns self
// and leaves __traceback__ None, the form assertRaises's __exit__ and
// `raise exc.with_traceback(tb)` rely on. unagi models no traceback object, so
// the argument is accepted and dropped.
func TestExcWithTraceback(t *testing.T) {
	e := Raise(ValueError, "boom")
	got, err := CallMethod(e, "with_traceback", []Object{None})
	if err != nil {
		t.Fatalf("with_traceback: %v", err)
	}
	if got != Object(e) {
		t.Fatalf("with_traceback returned %s; want self", Str(got))
	}
	tb, err := LoadAttr(e, "__traceback__")
	if err != nil || tb != None {
		t.Fatalf("__traceback__ = %v, %v; want None", tb, err)
	}
	// Wrong arity raises TypeError, matching the other exception methods.
	if _, err := CallMethod(e, "with_traceback", nil); err == nil {
		t.Fatal("with_traceback() with no args did not raise")
	}
}
