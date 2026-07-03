package objects

import "testing"

// All expected values and messages in this file were probed on
// CPython 3.14.6.

func TestListClearCopy(t *testing.T) {
	l := L(NewInt(1), NewInt(2), NewInt(3))
	if _, err := CallMethod(l, "clear", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := Repr(l); got != "[]" {
		t.Errorf("after clear: repr = %q, want %q", got, "[]")
	}
	// Probed arity shapes: list.clear() takes no arguments (1 given).
	_, err := CallMethod(l, "clear", []Object{NewInt(1)})
	checkErr(t, "clear 1 arg", err, "TypeError: list.clear() takes no arguments (1 given)")
	_, err = CallMethod(l, "clear", []Object{NewInt(1), NewInt(2)})
	checkErr(t, "clear 2 args", err, "TypeError: list.clear() takes no arguments (2 given)")
	_, err = CallMethod(l, "copy", []Object{NewInt(1)})
	checkErr(t, "copy 1 arg", err, "TypeError: list.copy() takes no arguments (1 given)")

	// Shallow copy: fresh backing array, shared element objects.
	// Probed: a=[[1],[2]]; b=a.copy(); b.append(9); a[0].append(7)
	// leaves a=[[1,7],[2]] and b=[[1,7],[2],9].
	inner := L(NewInt(1))
	orig := L(inner, L(NewInt(2)))
	cp, err := CallMethod(orig, "copy", nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if _, err := CallMethod(cp, "append", []Object{NewInt(9)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := CallMethod(inner, "append", []Object{NewInt(7)}); err != nil {
		t.Fatalf("append inner: %v", err)
	}
	if got := Repr(orig); got != "[[1, 7], [2]]" {
		t.Errorf("orig after copy mutations: %q", got)
	}
	if got := Repr(cp); got != "[[1, 7], [2], 9]" {
		t.Errorf("copy after mutations: %q", got)
	}
}

func TestListIndex(t *testing.T) {
	ints := func(vs ...int64) []Object {
		out := make([]Object, len(vs))
		for i, v := range vs {
			out[i] = NewInt(v)
		}
		return out
	}
	tests := []struct {
		name    string
		elts    []Object
		args    []Object
		want    string
		wantErr string
	}{
		{"basic", ints(1, 2, 3), ints(2), "1", ""},
		{"no args", ints(1), nil, "", "TypeError: index expected at least 1 argument, got 0"},
		{"four args", ints(1), ints(1, 2, 3, 4), "", "TypeError: index expected at most 3 arguments, got 4"},
		{"five args", ints(1), ints(1, 2, 3, 4, 5), "", "TypeError: index expected at most 3 arguments, got 5"},
		{"missing int", ints(1, 2, 3), ints(9), "", "ValueError: list.index(x): x not in list"},
		{"missing str", []Object{NewStr("a")}, []Object{NewStr("b")}, "", "ValueError: list.index(x): x not in list"},
		{"start skips first hit", ints(1, 2, 1, 2), ints(1, 1), "2", ""},
		{"start stop window", ints(1, 2, 1, 2), ints(2, 0, 2), "1", ""},
		{"stop is exclusive", ints(1, 2, 1, 2), ints(1, 1, 2), "", "ValueError: list.index(x): x not in list"},
		{"negative start", ints(1, 2, 1, 2), ints(2, -1), "3", ""},
		{"negative both", ints(1, 2, 1, 2), ints(1, -2, -1), "2", ""},
		{"start past end", ints(1, 2), ints(1, 100), "", "ValueError: list.index(x): x not in list"},
		{"stop past end ok", ints(1, 2), ints(2, 0, 100), "1", ""},
		{"start clamps to 0", ints(1, 2), ints(1, -100), "0", ""},
		{"stop clamps to 0", ints(1, 2), ints(1, 0, -100), "", "ValueError: list.index(x): x not in list"},
		{"start after stop", ints(1, 2), ints(1, 1, 0), "", "ValueError: list.index(x): x not in list"},
		{"bool start", ints(1, 2, 1), []Object{NewInt(1), True}, "2", ""},
		{"bool stop", ints(1, 2), []Object{NewInt(1), NewInt(0), True}, "0", ""},
		{"float start", ints(1, 2, 3), []Object{NewInt(1), NewFloat(1.5)}, "",
			"TypeError: slice indices must be integers or have an __index__ method"},
		{"str start", ints(1, 2, 3), []Object{NewInt(1), NewStr("a")}, "",
			"TypeError: slice indices must be integers or have an __index__ method"},
		// Bounds convert before the search, so a bad stop wins even
		// when the element sits at index 0.
		{"bad stop beats hit", ints(1), []Object{NewInt(1), NewInt(0), NewFloat(0.5)}, "",
			"TypeError: slice indices must be integers or have an __index__ method"},
		{"missing None", ints(1, 2, 3), []Object{None}, "", "ValueError: list.index(x): x not in list"},
	}
	for _, tt := range tests {
		got, err := CallMethod(NewList(tt.elts), "index", tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}
