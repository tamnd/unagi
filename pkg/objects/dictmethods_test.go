package objects

import "testing"

// All expected values and messages in this file were probed on
// CPython 3.14.6.

func TestDictClearCopy(t *testing.T) {
	d := D(t, NewStr("a"), NewInt(1), NewStr("b"), NewInt(2))
	_, err := CallMethod(d, "clear", []Object{NewInt(1)})
	checkErr(t, "clear 1 arg", err, "TypeError: dict.clear() takes no arguments (1 given)")
	if _, err := CallMethod(d, "clear", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := Repr(d); got != "{}" {
		t.Errorf("after clear: repr = %q, want %q", got, "{}")
	}
	// The cleared dict must accept fresh inserts.
	if err := d.(*dictObject).set(NewStr("x"), NewInt(9)); err != nil {
		t.Fatalf("set after clear: %v", err)
	}
	if got := Repr(d); got != "{'x': 9}" {
		t.Errorf("insert after clear: repr = %q", got)
	}

	_, err = CallMethod(d, "copy", []Object{NewInt(1)})
	checkErr(t, "copy 1 arg", err, "TypeError: dict.copy() takes no arguments (1 given)")

	// Shallow copy with independent storage. Probed: c={'x':[1]};
	// cc=c.copy(); cc['y']=2; c['x'].append(3) leaves
	// c={'x': [1, 3]} and cc={'x': [1, 3], 'y': 2}.
	inner := L(NewInt(1))
	c := D(t, NewStr("x"), inner)
	ccObj, err := CallMethod(c, "copy", nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	cc := ccObj.(*dictObject)
	if err := cc.set(NewStr("y"), NewInt(2)); err != nil {
		t.Fatalf("set on copy: %v", err)
	}
	if _, err := CallMethod(inner, "append", []Object{NewInt(3)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if got := Repr(c); got != "{'x': [1, 3]}" {
		t.Errorf("orig after copy mutations: %q", got)
	}
	if got := Repr(cc); got != "{'x': [1, 3], 'y': 2}" {
		t.Errorf("copy after mutations: %q", got)
	}
	// And the other direction: inserting into the original does not
	// show up in the copy.
	if err := c.(*dictObject).set(NewStr("z"), NewInt(5)); err != nil {
		t.Fatalf("set on orig: %v", err)
	}
	if got := Repr(cc); got != "{'x': [1, 3], 'y': 2}" {
		t.Errorf("copy sees orig insert: %q", got)
	}
}

func TestDictFromkeys(t *testing.T) {
	recv := D(t, NewStr("z"), NewInt(9))
	tests := []struct {
		name    string
		args    []Object
		want    string
		wantErr string
	}{
		{"default None", []Object{NewStr("ab")}, "{'a': None, 'b': None}", ""},
		{"explicit value", []Object{L(NewInt(1), NewInt(2)), NewInt(0)}, "{1: 0, 2: 0}", ""},
		{"dup keys", []Object{L(NewInt(1), NewInt(2), NewInt(1)), NewStr("v")}, "{1: 'v', 2: 'v'}", ""},
		// Probed: the receiver's own contents never leak into the result.
		{"ignores receiver", []Object{L(NewInt(1))}, "{1: None}", ""},
		{"unhashable elt", []Object{L(L(NewInt(1)))}, "",
			"TypeError: cannot use 'list' as a dict key (unhashable type: 'list')"},
		{"later unhashable", []Object{L(NewInt(1), L(NewInt(2)))}, "",
			"TypeError: cannot use 'list' as a dict key (unhashable type: 'list')"},
		{"non-iterable", []Object{NewInt(1)}, "", "TypeError: 'int' object is not iterable"},
		{"no args", nil, "", "TypeError: fromkeys expected at least 1 argument, got 0"},
		{"three args", []Object{L(NewInt(1)), NewInt(2), NewInt(3)}, "",
			"TypeError: fromkeys expected at most 2 arguments, got 3"},
	}
	for _, tt := range tests {
		got, err := CallMethod(recv, "fromkeys", tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}

	// A dict argument iterates its keys.
	src := D(t, NewStr("k"), NewInt(1), NewStr("j"), NewInt(2))
	got, err := CallMethod(recv, "fromkeys", []Object{src})
	checkRepr(t, "dict arg", got, err, "{'k': None, 'j': None}")
	// The receiver stayed untouched throughout.
	if got := Repr(recv); got != "{'z': 9}" {
		t.Errorf("receiver mutated: %q", got)
	}
}

func TestDictPopitem(t *testing.T) {
	d := D(t, NewStr("a"), NewInt(1), NewStr("b"), NewInt(2))
	_, err := CallMethod(d, "popitem", []Object{NewInt(1)})
	checkErr(t, "1 arg", err, "TypeError: dict.popitem() takes no arguments (1 given)")

	// LIFO: last inserted comes off first, as a 2-tuple.
	got, err := CallMethod(d, "popitem", nil)
	checkRepr(t, "first pop", got, err, "('b', 2)")
	got, err = CallMethod(d, "popitem", nil)
	checkRepr(t, "second pop", got, err, "('a', 1)")
	if got := Repr(d); got != "{}" {
		t.Errorf("after pops: repr = %q", got)
	}
	_, err = CallMethod(d, "popitem", nil)
	checkErr(t, "empty", err, "KeyError: 'popitem(): dictionary is empty'")

	// The popped key is really gone from the index too.
	if err := d.(*dictObject).set(NewStr("b"), NewInt(7)); err != nil {
		t.Fatalf("reinsert: %v", err)
	}
	if got := Repr(d); got != "{'b': 7}" {
		t.Errorf("reinsert after popitem: %q", got)
	}
}

func TestDictSetdefault(t *testing.T) {
	d := D(t, NewStr("a"), NewInt(1))
	tests := []struct {
		name     string
		args     []Object
		want     string
		wantDict string
		wantErr  string
	}{
		{"hit keeps value", []Object{NewStr("a"), NewInt(9)}, "1", "{'a': 1}", ""},
		{"miss inserts", []Object{NewStr("b"), NewInt(9)}, "9", "{'a': 1, 'b': 9}", ""},
		{"miss default None", []Object{NewStr("c")}, "None", "{'a': 1, 'b': 9, 'c': None}", ""},
		{"unhashable key", []Object{L(), NewInt(1)}, "", "",
			"TypeError: cannot use 'list' as a dict key (unhashable type: 'list')"},
		{"no args", nil, "", "", "TypeError: setdefault expected at least 1 argument, got 0"},
		{"three args", []Object{NewInt(1), NewInt(2), NewInt(3)}, "", "",
			"TypeError: setdefault expected at most 2 arguments, got 3"},
	}
	for _, tt := range tests {
		got, err := CallMethod(d, "setdefault", tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
		if gotD := Repr(d); gotD != tt.wantDict {
			t.Errorf("%s: dict = %q, want %q", tt.name, gotD, tt.wantDict)
		}
	}
}

func TestDictUpdate(t *testing.T) {
	// Zero arguments is a no-op.
	d := D(t, NewStr("a"), NewInt(1))
	got, err := CallMethod(d, "update", nil)
	checkRepr(t, "no args", got, err, "None")
	if gotD := Repr(d); gotD != "{'a': 1}" {
		t.Errorf("no args mutated: %q", gotD)
	}
	_, err = CallMethod(d, "update", []Object{D(t), D(t)})
	checkErr(t, "two args", err, "TypeError: update expected at most 1 argument, got 2")

	// dict argument: new keys append, existing keys keep position.
	if _, err := CallMethod(d, "update", []Object{D(t, NewStr("b"), NewInt(2))}); err != nil {
		t.Fatalf("update dict: %v", err)
	}
	if gotD := Repr(d); gotD != "{'a': 1, 'b': 2}" {
		t.Errorf("update dict: %q", gotD)
	}

	// iterable of pairs, overwriting one existing key.
	pairs := L(T(NewStr("c"), NewInt(3)), T(NewStr("a"), NewInt(9)))
	if _, err := CallMethod(d, "update", []Object{pairs}); err != nil {
		t.Fatalf("update pairs: %v", err)
	}
	if gotD := Repr(d); gotD != "{'a': 9, 'b': 2, 'c': 3}" {
		t.Errorf("update pairs: %q", gotD)
	}

	// a two-char string works as a pair; probed d.update(['xy']).
	if _, err := CallMethod(d, "update", []Object{L(NewStr("xy"))}); err != nil {
		t.Fatalf("update str pair: %v", err)
	}
	if gotD := Repr(d); gotD != "{'a': 9, 'b': 2, 'c': 3, 'x': 'y'}" {
		t.Errorf("update str pair: %q", gotD)
	}

	// items view of another dict.
	src := D(t, NewStr("d"), NewInt(4))
	items, err := CallMethod(src, "items", nil)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if _, err := CallMethod(d, "update", []Object{items}); err != nil {
		t.Fatalf("update items view: %v", err)
	}
	if gotD := Repr(d); gotD != "{'a': 9, 'b': 2, 'c': 3, 'x': 'y', 'd': 4}" {
		t.Errorf("update items view: %q", gotD)
	}

	// self update is a no-op that must not loop or corrupt.
	if _, err := CallMethod(d, "update", []Object{d}); err != nil {
		t.Fatalf("self update: %v", err)
	}
	if gotD := Repr(d); gotD != "{'a': 9, 'b': 2, 'c': 3, 'x': 'y', 'd': 4}" {
		t.Errorf("self update: %q", gotD)
	}

	errTests := []struct {
		name    string
		arg     Object
		wantErr string
	}{
		{"pair too long", L(T(NewInt(1), NewInt(2), NewInt(3))),
			"ValueError: dictionary update sequence element #0 has length 3; 2 is required"},
		{"pair too short", L(T(NewStr("x"))),
			"ValueError: dictionary update sequence element #0 has length 1; 2 is required"},
		{"empty pair", L(T()),
			"ValueError: dictionary update sequence element #0 has length 0; 2 is required"},
		{"second pair bad", L(T(NewStr("q"), NewInt(5)), T(NewInt(1), NewInt(2), NewInt(3))),
			"ValueError: dictionary update sequence element #1 has length 3; 2 is required"},
		// Probed: the element error is the bare "object is not iterable",
		// no type name, unlike the whole-argument error below.
		{"element not iterable", L(NewInt(1)), "TypeError: object is not iterable"},
		{"arg not iterable", NewInt(1), "TypeError: 'int' object is not iterable"},
		{"unhashable pair key", L(T(L(NewInt(1)), NewInt(2))),
			"TypeError: cannot use 'list' as a dict key (unhashable type: 'list')"},
	}
	for _, tt := range errTests {
		e := D(t)
		_, err := CallMethod(e, "update", []Object{tt.arg})
		checkErr(t, tt.name, err, tt.wantErr)
	}

	// Pairs before the failing element stay merged; probed:
	// {}.update([('a',1),(2,3,4)]) leaves {'a': 1} behind the ValueError.
	partial := D(t)
	_, err = CallMethod(partial, "update", []Object{L(
		T(NewStr("a"), NewInt(1)),
		T(NewInt(2), NewInt(3), NewInt(4)),
	)})
	checkErr(t, "partial merge error", err,
		"ValueError: dictionary update sequence element #1 has length 3; 2 is required")
	if gotD := Repr(partial); gotD != "{'a': 1}" {
		t.Errorf("partial merge: %q", gotD)
	}
}
