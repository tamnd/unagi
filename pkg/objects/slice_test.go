package objects

import "testing"

// Every expected value and message below was probed against python3.14
// (3.14.6). Notable probe results: slice bounds clamp instead of raising,
// 3.14.6 reports every non-iterable slice assignment as "must assign
// iterable to extended slice" even for a contiguous slice, and single-item
// deletion says "doesn't support item deletion" while slice deletion
// spells out "does not".

func ints(vs ...int64) []Object {
	out := make([]Object, len(vs))
	for i, v := range vs {
		out[i] = NewInt(v)
	}
	return out
}

func TestGetSlice(t *testing.T) {
	tests := []struct {
		name         string
		o            Object
		lo, hi, step Object
		want         string
		wantErr      string
	}{
		{"list-reverse", NewList(ints(1, 2, 3, 4, 5)), None, None, NewInt(-1), "[5, 4, 3, 2, 1]", ""},
		{"str-reverse", NewStr("abc"), None, None, NewInt(-1), "'cba'", ""},
		{"list-basic", NewList(ints(1, 2, 3)), NewInt(1), None, None, "[2, 3]", ""},
		{"list-lo-hi", NewList(ints(1, 2, 3, 4, 5)), NewInt(1), NewInt(3), None, "[2, 3]", ""},
		{"list-step2", NewList(ints(1, 2, 3, 4, 5)), None, None, NewInt(2), "[1, 3, 5]", ""},
		{"list-neg-step-bounds", NewList(ints(1, 2, 3, 4, 5)), NewInt(4), NewInt(0), NewInt(-2), "[5, 3]", ""},
		{"list-clamp", NewList(ints(1, 2, 3, 4, 5)), NewInt(-100), NewInt(100), None, "[1, 2, 3, 4, 5]", ""},
		{"list-oor-empty", NewList(ints(1, 2)), NewInt(10), NewInt(20), None, "[]", ""},
		{"str-oor-empty", NewStr("ab"), NewInt(5), None, None, "''", ""},
		{"str-neg-bounds", NewStr("abcdef"), NewInt(-4), NewInt(-1), None, "'cde'", ""},
		{"str-unicode", NewStr("héllo"), NewInt(1), NewInt(3), None, "'él'", ""},
		{"tuple-slice", NewTuple(ints(1, 2, 3)), NewInt(0), NewInt(2), None, "(1, 2)", ""},
		{"tuple-one", NewTuple(ints(1, 2)), NewInt(1), None, None, "(2,)", ""},
		{"bool-parts", NewList(ints(1, 2, 3)), False, True, None, "[1]", ""},
		// Probed on 3.14: [1][::0].
		{"step-zero", NewList(ints(1)), None, None, NewInt(0), "", "ValueError: slice step cannot be zero"},
		// Probed on 3.14: [1][None:'a'] and 'ab'[1.5:].
		{"bad-hi", NewList(ints(1)), None, NewStr("a"), None, "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		{"bad-lo-float", NewStr("ab"), NewFloat(1.5), None, None, "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		{"bad-step", NewTuple(ints(1)), None, None, NewStr("x"), "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		// Probed on 3.14: (1)[0:1].
		{"int-receiver", NewInt(1), NewInt(0), NewInt(1), None, "", "TypeError: 'int' object is not subscriptable"},
	}
	for _, tt := range tests {
		got, err := GetSlice(tt.o, tt.lo, tt.hi, tt.step)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestGetSliceCopies(t *testing.T) {
	orig := NewList(ints(1, 2, 3))
	cp, err := GetSlice(orig, None, None, None)
	if err != nil {
		t.Fatalf("full copy: %v", err)
	}
	if cp == orig {
		t.Fatal("xs[:] returned the same list object")
	}
	if err := SetItem(cp, NewInt(0), NewInt(9)); err != nil {
		t.Fatalf("SetItem on copy: %v", err)
	}
	if got := Repr(orig); got != "[1, 2, 3]" {
		t.Errorf("original changed through copy: %s", got)
	}
}

func TestSetSlice(t *testing.T) {
	tests := []struct {
		name         string
		o            Object
		lo, hi, step Object
		val          Object
		want         string
		wantErr      string
	}{
		{"replace-shrink", NewList(ints(1, 2, 3, 4, 5)), NewInt(1), NewInt(3), None, NewList(ints(9)), "[1, 9, 4, 5]", ""},
		{"insert-front", NewList(ints(1, 5)), NewInt(0), NewInt(0), None, NewList(ints(7, 8)), "[7, 8, 1, 5]", ""},
		{"grow", NewList(ints(1, 2)), NewInt(1), NewInt(2), None, NewList(ints(8, 9)), "[1, 8, 9]", ""},
		// Probed on 3.14: xs = [1, 2, 3]; xs[3:1] = [9] appends at 3.
		{"hi-below-lo", NewList(ints(1, 2, 3)), NewInt(3), NewInt(1), None, NewList(ints(9)), "[1, 2, 3, 9]", ""},
		{"clear-all", NewList(ints(1, 2, 3)), None, None, None, NewList(nil), "[]", ""},
		{"tuple-value", NewList(ints(1, 2)), NewInt(0), NewInt(1), None, NewTuple(ints(9)), "[9, 2]", ""},
		// Probed on 3.14: xs = [1]; xs[0:1] = 'ab' splices in the characters.
		{"str-value", NewList(ints(1)), NewInt(0), NewInt(1), None, NewStr("ab"), "['a', 'b']", ""},
		{"explicit-step1", NewList(ints(1, 2, 3)), NewInt(0), NewInt(2), NewInt(1), NewList(ints(9)), "[9, 3]", ""},
		// Probed on 3.14: xs = [1, 2, 3, 4]; xs[::2] = ['a', 'b'].
		{"extended", NewList(ints(1, 2, 3, 4)), None, None, NewInt(2),
			NewList([]Object{NewStr("a"), NewStr("b")}), "['a', 2, 'b', 4]", ""},
		{"extended-reverse", NewList(ints(1, 2, 3, 4)), None, None, NewInt(-1), NewList(ints(5, 6, 7, 8)), "[8, 7, 6, 5]", ""},
		// Probed on 3.14: xs = [1, 2, 3, 4]; xs[::2] = [1].
		{"extended-short", NewList(ints(1, 2, 3, 4)), None, None, NewInt(2), NewList(ints(1)), "",
			"ValueError: attempt to assign sequence of size 1 to extended slice of size 2"},
		{"extended-long", NewList(ints(1, 2, 3, 4)), None, None, NewInt(2), NewList(ints(1, 2, 3)), "",
			"ValueError: attempt to assign sequence of size 3 to extended slice of size 2"},
		// Probed on 3.14.6: xs[0:1] = 5 and xs[::2] = 5, one shared text.
		{"noniter-contig", NewList(ints(1, 2)), NewInt(0), NewInt(1), None, NewInt(5), "",
			"TypeError: must assign iterable to extended slice"},
		{"noniter-extended", NewList(ints(1, 2)), None, None, NewInt(2), NewInt(5), "",
			"TypeError: must assign iterable to extended slice"},
		// Probed on 3.14: xs[::0] = 5 raises before the value is looked at.
		{"step-zero-first", NewList(ints(1, 2)), None, None, NewInt(0), NewInt(5), "",
			"ValueError: slice step cannot be zero"},
		{"bad-part-first", NewList(ints(1, 2)), None, NewStr("a"), None, NewInt(5), "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		// Probed on 3.14: (1, 2)[0:1] = [9] and 'ab'[0:1] = 'x'.
		{"tuple-receiver", NewTuple(ints(1, 2)), NewInt(0), NewInt(1), None, NewList(ints(9)), "",
			"TypeError: 'tuple' object does not support item assignment"},
		{"str-receiver", NewStr("ab"), NewInt(0), NewInt(1), None, NewStr("x"), "",
			"TypeError: 'str' object does not support item assignment"},
	}
	for _, tt := range tests {
		err := SetSlice(tt.o, tt.lo, tt.hi, tt.step, tt.val)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, tt.o, err, tt.want)
	}
}

func TestSetSliceSelf(t *testing.T) {
	xs := NewList(ints(1, 2, 3))
	if err := SetSlice(xs, None, None, None, xs); err != nil {
		t.Fatalf("xs[:] = xs: %v", err)
	}
	if got := Repr(xs); got != "[1, 2, 3]" {
		t.Errorf("xs[:] = xs -> %s", got)
	}
}

func TestDelItem(t *testing.T) {
	xs := NewList(ints(1, 2, 3))
	if err := DelItem(xs, NewInt(1)); err != nil {
		t.Fatalf("del xs[1]: %v", err)
	}
	if got := Repr(xs); got != "[1, 3]" {
		t.Errorf("after del xs[1] = %s", got)
	}
	if err := DelItem(xs, NewInt(-1)); err != nil {
		t.Fatalf("del xs[-1]: %v", err)
	}
	if got := Repr(xs); got != "[1]" {
		t.Errorf("after del xs[-1] = %s", got)
	}
	// Probed on 3.14: xs = [1]; del xs[5].
	err := DelItem(xs, NewInt(5))
	checkErr(t, "del list oob", err, "IndexError: list assignment index out of range")
	// Probed on 3.14: del [1][None], type spelled bare.
	err = DelItem(xs, None)
	checkErr(t, "del list nonint", err, "TypeError: list indices must be integers or slices, not NoneType")

	d := D(t, NewStr("k"), NewInt(1), NewInt(2), NewInt(3))
	if err := DelItem(d, NewStr("k")); err != nil {
		t.Fatalf("del d['k']: %v", err)
	}
	if got := Repr(d); got != "{2: 3}" {
		t.Errorf("after del d['k'] = %s", got)
	}
	// Probed on 3.14: del {}['k'] -> KeyError: 'k' (repr of the key).
	err = DelItem(d, NewStr("k"))
	checkErr(t, "del dict missing", err, "KeyError: 'k'")
	err = DelItem(d, L(NewInt(1)))
	checkErr(t, "del dict unhashable", err, "TypeError: cannot use 'list' as a dict key (unhashable type: 'list')")

	// Probed on 3.14: del (1, 2)[0] and del 'ab'[0] say "doesn't".
	err = DelItem(T(NewInt(1)), NewInt(0))
	checkErr(t, "del tuple item", err, "TypeError: 'tuple' object doesn't support item deletion")
	err = DelItem(NewStr("ab"), NewInt(0))
	checkErr(t, "del str item", err, "TypeError: 'str' object doesn't support item deletion")
}

func TestDelSlice(t *testing.T) {
	tests := []struct {
		name         string
		o            Object
		lo, hi, step Object
		want         string
		wantErr      string
	}{
		{"contig", NewList(ints(1, 9, 4, 5)), NewInt(1), NewInt(3), None, "[1, 5]", ""},
		{"all", NewList(ints(1, 2, 3)), None, None, None, "[]", ""},
		{"oor-noop", NewList(ints(1, 2, 3)), NewInt(10), NewInt(20), None, "[1, 2, 3]", ""},
		// Probed on 3.14: xs = [1, 2, 3, 4, 5]; del xs[::2] -> [2, 4],
		// and del xs[::-2] leaves the same [2, 4].
		{"extended", NewList(ints(1, 2, 3, 4, 5)), None, None, NewInt(2), "[2, 4]", ""},
		{"extended-neg", NewList(ints(1, 2, 3, 4, 5)), None, None, NewInt(-2), "[2, 4]", ""},
		{"extended-bounds", NewList(ints(1, 2, 3, 4, 5)), NewInt(1), NewInt(4), NewInt(2), "[1, 3, 5]", ""},
		// Probed on 3.14: del [1, 2][::0].
		{"step-zero", NewList(ints(1, 2)), None, None, NewInt(0), "", "ValueError: slice step cannot be zero"},
		{"bad-part", NewList(ints(1, 2)), NewStr("a"), None, None, "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		// Probed on 3.14: del (1, 2)[0:1] and del 'ab'[0:1] say "does not".
		{"tuple-receiver", NewTuple(ints(1, 2)), NewInt(0), NewInt(1), None, "",
			"TypeError: 'tuple' object does not support item deletion"},
		{"str-receiver", NewStr("ab"), NewInt(0), NewInt(1), None, "",
			"TypeError: 'str' object does not support item deletion"},
	}
	for _, tt := range tests {
		err := DelSlice(tt.o, tt.lo, tt.hi, tt.step)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, tt.o, err, tt.want)
	}
}
