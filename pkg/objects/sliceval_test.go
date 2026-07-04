package objects

import "testing"

// TestSliceOfArity covers the slice() constructor: one argument fills stop,
// two fill start and stop, three fill all, and out-of-range counts raise the
// probed arity TypeError.
func TestSliceOfArity(t *testing.T) {
	tests := []struct {
		name    string
		args    []Object
		want    string
		wantErr string
	}{
		{"one", []Object{NewInt(5)}, "slice(None, 5, None)", ""},
		{"two", []Object{NewInt(1), NewInt(2)}, "slice(1, 2, None)", ""},
		{"three", []Object{NewInt(1), NewInt(10), NewInt(2)}, "slice(1, 10, 2)", ""},
		{"none", nil, "", "TypeError: slice expected at least 1 argument, got 0"},
		{"four", []Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)}, "",
			"TypeError: slice expected at most 3 arguments, got 4"},
	}
	for _, tt := range tests {
		got, err := SliceOf(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

// TestSliceAttrs reads the three read-only parts, keeping None and non-int
// bounds verbatim.
func TestSliceAttrs(t *testing.T) {
	s := NewSlice(NewFloat(1.5), NewStr("x"), None)
	for _, tt := range []struct {
		name string
		want string
	}{
		{"start", "1.5"}, {"stop", "'x'"}, {"step", "None"},
	} {
		v, err := LoadAttr(s, tt.name)
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if got := Repr(v); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
	if _, err := LoadAttr(s, "missing"); err == nil {
		t.Error("missing attr: expected AttributeError")
	}
}

// TestSliceEqualHash checks that equal slices compare equal and hash the same,
// and that a slice never collides with the like-looking tuple.
func TestSliceEqualHash(t *testing.T) {
	a := NewSlice(NewInt(1), NewInt(2), NewInt(3))
	b := NewSlice(NewInt(1), NewInt(2), NewInt(3))
	c := NewSlice(NewInt(1), NewInt(2), None)
	if !equals(a, b) {
		t.Error("equal slices compared unequal")
	}
	if equals(a, c) {
		t.Error("differing slices compared equal")
	}
	if equals(a, NewInt(5)) {
		t.Error("slice compared equal to int")
	}
	ha, _ := PyHash(a)
	hb, _ := PyHash(b)
	if ha != hb {
		t.Errorf("equal slices hashed differently: %d vs %d", ha, hb)
	}
	ht, _ := PyHash(NewTuple([]Object{NewInt(1), NewInt(2), NewInt(3)}))
	if ha == ht {
		t.Error("slice hash collided with the equal tuple")
	}
}

// TestSliceIndicesMethod exercises slice.indices against the probed triples,
// including the negative-length and arity errors.
func TestSliceIndicesMethod(t *testing.T) {
	tests := []struct {
		name    string
		slice   Object
		arg     Object
		want    string
		wantErr string
	}{
		{"forward", NewSlice(None, NewInt(5), None), NewInt(20), "(0, 5, 1)", ""},
		{"step", NewSlice(NewInt(1), NewInt(10), NewInt(2)), NewInt(20), "(1, 10, 2)", ""},
		{"reverse", NewSlice(None, None, NewInt(-1)), NewInt(5), "(4, -1, -1)", ""},
		{"bool-len", NewSlice(NewInt(1), NewInt(2), None), NewBool(true), "(1, 1, 1)", ""},
		{"neg", NewSlice(NewInt(1), NewInt(2), None), NewInt(-5), "",
			"ValueError: length should not be negative"},
		{"type", NewSlice(NewInt(1), NewInt(2), None), NewStr("x"), "",
			"TypeError: 'str' object cannot be interpreted as an integer"},
	}
	for _, tt := range tests {
		got, err := CallMethod(tt.slice, "indices", []Object{tt.arg})
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
	if _, err := CallMethod(NewSlice(NewInt(1), NewInt(2), None), "indices",
		[]Object{NewInt(1), NewInt(2)}); err == nil {
		t.Error("indices arity: expected TypeError")
	}
}
