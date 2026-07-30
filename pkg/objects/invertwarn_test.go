package objects

import "testing"

// TestInvertBoolWarnHook checks Invert fires the deprecation hook for an exact
// bool and not for a plain int, still returning the bitwise inversion of the
// underlying int in both cases. A hook that returns an error aborts the invert,
// the way a warnings filter promoting the warning to an exception does.
func TestInvertBoolWarnHook(t *testing.T) {
	saved := InvertBoolWarnHook
	defer func() { InvertBoolWarnHook = saved }()

	calls := 0
	InvertBoolWarnHook = func() error { calls++; return nil }

	// A bool fires the hook and inverts the underlying int: ~False == -1.
	r, err := Invert(False)
	if err != nil {
		t.Fatalf("Invert(False): %v", err)
	}
	if n, _ := AsInt(r); n != -1 {
		t.Errorf("~False = %s, want -1", Repr(r))
	}
	if r, _ = Invert(True); Repr(r) != "-2" {
		t.Errorf("~True = %s, want -2", Repr(r))
	}
	if calls != 2 {
		t.Errorf("hook fired %d times for two bools, want 2", calls)
	}

	// A plain int does not fire the hook.
	calls = 0
	if r, _ = Invert(NewInt(5)); Repr(r) != "-6" {
		t.Errorf("~5 = %s, want -6", Repr(r))
	}
	if calls != 0 {
		t.Errorf("hook fired %d times for an int, want 0", calls)
	}

	// A hook error aborts the operation.
	InvertBoolWarnHook = func() error { return Raise(ValueError, "promoted") }
	if _, err = Invert(True); err == nil {
		t.Error("Invert(True) with erroring hook returned no error")
	}
}
