package objects

import "testing"

// S builds a set for tests, failing on unhashable elements.
func S(t *testing.T, elts ...Object) Object {
	t.Helper()
	s, err := NewSet(elts)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	return s
}

// FS builds a frozenset for tests.
func FS(t *testing.T, elts ...Object) Object {
	t.Helper()
	f, err := NewFrozenset(elts)
	if err != nil {
		t.Fatalf("NewFrozenset: %v", err)
	}
	return f
}

func TestSetConstructors(t *testing.T) {
	tests := []struct {
		name    string
		elts    []Object
		frozen  bool
		want    string
		wantErr string
	}{
		{"empty set", nil, false, "set()", ""},
		{"one", []Object{NewInt(1)}, false, "{1}", ""},
		{"order kept", []Object{NewInt(3), NewInt(1), NewInt(2)}, false, "{3, 1, 2}", ""},
		{"dedup", []Object{NewInt(3), NewInt(1), NewInt(2), NewInt(1)}, false, "{3, 1, 2}", ""},
		{"dedup 1 True 1.0", []Object{NewInt(1), True, NewFloat(1)}, false, "{1}", ""},
		{"dedup str", []Object{NewStr("a"), NewStr("a")}, false, "{'a'}", ""},
		{"unhashable list", []Object{L()}, false, "",
			"TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		{"unhashable dict", []Object{&dictObject{index: map[string]int{}}}, false, "",
			"TypeError: cannot use 'dict' as a set element (unhashable type: 'dict')"},
		{"unhashable tuple with list", []Object{T(NewInt(1), L())}, false, "",
			"TypeError: cannot use 'tuple' as a set element (unhashable type: 'list')"},
		{"empty frozenset", nil, true, "frozenset()", ""},
		{"frozen one", []Object{NewInt(1)}, true, "frozenset({1})", ""},
		{"frozen two", []Object{NewInt(1), NewInt(2)}, true, "frozenset({1, 2})", ""},
		{"frozen unhashable", []Object{L()}, true, "",
			"TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
	}
	for _, tt := range tests {
		var got Object
		var err error
		if tt.frozen {
			got, err = NewFrozenset(tt.elts)
		} else {
			got, err = NewSet(tt.elts)
		}
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestSetAdd(t *testing.T) {
	s := S(t)
	for _, v := range []Object{NewInt(2), NewInt(1), NewInt(2), True} {
		if err := SetAdd(s, v); err != nil {
			t.Fatalf("SetAdd(%s): %v", Repr(v), err)
		}
	}
	if got := Repr(s); got != "{2, 1}" {
		t.Errorf("after adds: got %s, want {2, 1}", got)
	}
	err := SetAdd(s, L())
	checkErr(t, "add unhashable", err,
		"TypeError: cannot use 'list' as a set element (unhashable type: 'list')")
	if got := Repr(s); got != "{2, 1}" {
		t.Errorf("after failed add: got %s, want {2, 1}", got)
	}
}

func TestSetAsElementAndKey(t *testing.T) {
	// set is unhashable as an element or dict key.
	if _, err := NewSet([]Object{S(t, NewInt(1))}); err == nil ||
		err.Error() != "TypeError: cannot use 'set' as a set element (unhashable type: 'set')" {
		t.Errorf("set element: got %v", err)
	}
	if _, err := NewDict([]Object{S(t)}, []Object{NewInt(1)}); err == nil ||
		err.Error() != "TypeError: cannot use 'set' as a dict key (unhashable type: 'set')" {
		t.Errorf("set dict key: got %v", err)
	}

	// frozenset hashes order-independently: {1,2} and {2,1} collide.
	fs12 := FS(t, NewInt(1), NewInt(2))
	fs21 := FS(t, NewInt(2), NewInt(1))
	s, err := NewSet([]Object{fs12, fs21})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	if n, _ := Len(s); n != 1 {
		t.Errorf("frozenset dedup: len = %d, want 1", n)
	}

	// frozenset works as a dict key, looked up by a reordered twin.
	d, err := NewDict([]Object{fs12}, []Object{NewStr("x")})
	if err != nil {
		t.Fatalf("NewDict: %v", err)
	}
	v, err := GetItem(d, fs21)
	checkRepr(t, "frozenset dict key", v, err, "'x'")

	// nested repr uses element reprs.
	nest := S(t, fs12)
	if got := Repr(nest); got != "{frozenset({1, 2})}" {
		t.Errorf("nested repr = %q", got)
	}
}

func TestBitwiseInt(t *testing.T) {
	tests := []struct {
		name     string
		fn       binFn
		a, b     Object
		want     string
		wantType string
		wantErr  string
	}{
		{"int|int", BitOr, NewInt(1), NewInt(2), "3", "int", ""},
		{"int^int", BitXor, NewInt(3), NewInt(1), "2", "int", ""},
		{"int&int", BitAnd, NewInt(3), NewInt(1), "1", "int", ""},
		// Probed: True | False is bool True; & and ^ on bool pairs stay bool.
		{"bool|bool", BitOr, True, False, "True", "bool", ""},
		{"bool&bool", BitAnd, True, True, "True", "bool", ""},
		{"bool^bool", BitXor, True, False, "True", "bool", ""},
		{"bool^bool false", BitXor, True, True, "False", "bool", ""},
		// Probed: mixing bool with int decays to int.
		{"bool|int", BitOr, True, NewInt(1), "1", "int", ""},
		{"int|bool", BitOr, NewInt(2), True, "3", "int", ""},
		{"lshift", LShift, NewInt(1), NewInt(3), "8", "int", ""},
		{"lshift zero", LShift, NewInt(0), NewInt(0), "0", "int", ""},
		// Probed: True << True is int 2, shifts never stay bool.
		{"bool<<bool", LShift, True, True, "2", "int", ""},
		{"rshift", RShift, NewInt(8), NewInt(2), "2", "int", ""},
		{"rshift neg value", RShift, NewInt(-1), NewInt(10), "-1", "int", ""},
		{"rshift big count", RShift, NewInt(-8), NewInt(200), "-1", "int", ""},
		{"lshift big count", LShift, NewInt(1), NewInt(200), "1606938044258990275541962092341162602522202993782792835301376", "int", ""},
		{"lshift neg count", LShift, NewInt(1), NewInt(-1), "", "", "ValueError: negative shift count"},
		{"rshift neg count", RShift, NewInt(1), NewInt(-1), "", "", "ValueError: negative shift count"},
		{"int|str", BitOr, NewInt(1), NewStr("a"), "", "", "TypeError: unsupported operand type(s) for |: 'int' and 'str'"},
		{"str|int", BitOr, NewStr("a"), NewInt(1), "", "", "TypeError: unsupported operand type(s) for |: 'str' and 'int'"},
		{"int^str", BitXor, NewInt(1), NewStr("a"), "", "", "TypeError: unsupported operand type(s) for ^: 'int' and 'str'"},
		{"int&str", BitAnd, NewInt(1), NewStr("a"), "", "", "TypeError: unsupported operand type(s) for &: 'int' and 'str'"},
		{"int<<str", LShift, NewInt(1), NewStr("a"), "", "", "TypeError: unsupported operand type(s) for <<: 'int' and 'str'"},
		{"str<<int", LShift, NewStr("a"), NewInt(1), "", "", "TypeError: unsupported operand type(s) for <<: 'str' and 'int'"},
		{"int>>str", RShift, NewInt(1), NewStr("a"), "", "", "TypeError: unsupported operand type(s) for >>: 'int' and 'str'"},
		{"str>>int", RShift, NewStr("a"), NewInt(1), "", "", "TypeError: unsupported operand type(s) for >>: 'str' and 'int'"},
		// Probed: floats never take bitwise ops.
		{"int|float", BitOr, NewInt(1), NewFloat(1), "", "", "TypeError: unsupported operand type(s) for |: 'int' and 'float'"},
		{"float|int", BitOr, NewFloat(1), NewInt(1), "", "", "TypeError: unsupported operand type(s) for |: 'float' and 'int'"},
		{"int<<float", LShift, NewInt(1), NewFloat(1), "", "", "TypeError: unsupported operand type(s) for <<: 'int' and 'float'"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.a, tt.b)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
		if err == nil && got.TypeName() != tt.wantType {
			t.Errorf("%s: type = %s, want %s", tt.name, got.TypeName(), tt.wantType)
		}
	}
}

func TestInvert(t *testing.T) {
	tests := []struct {
		name    string
		o       Object
		want    string
		wantErr string
	}{
		{"~5", NewInt(5), "-6", ""},
		{"~0", NewInt(0), "-1", ""},
		{"~-1", NewInt(-1), "0", ""},
		// Probed: ~True is int -2, ~False is int -1.
		{"~True", True, "-2", ""},
		{"~False", False, "-1", ""},
		{"~float", NewFloat(1.5), "", "TypeError: bad operand type for unary ~: 'float'"},
		{"~str", NewStr("a"), "", "TypeError: bad operand type for unary ~: 'str'"},
		{"~list", L(), "", "TypeError: bad operand type for unary ~: 'list'"},
		{"~None", None, "", "TypeError: bad operand type for unary ~: 'NoneType'"},
	}
	for _, tt := range tests {
		got, err := Invert(tt.o)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
	if got, _ := Invert(True); got.TypeName() != "int" {
		t.Errorf("~True type = %s, want int", got.TypeName())
	}
}

func TestSetOperators(t *testing.T) {
	tests := []struct {
		name     string
		fn       binFn
		a, b     Object
		want     string
		wantType string
		wantErr  string
	}{
		{"set|set", BitOr, S(t, NewInt(1), NewInt(2)), S(t, NewInt(2), NewInt(3)), "{1, 2, 3}", "set", ""},
		{"set&set", BitAnd, S(t, NewInt(3), NewInt(1), NewInt(2)), S(t, NewInt(2), NewInt(3)), "{3, 2}", "set", ""},
		{"set^set", BitXor, S(t, NewInt(1), NewInt(2), NewInt(3)), S(t, NewInt(3), NewInt(4)), "{1, 2, 4}", "set", ""},
		{"set-set", Sub, S(t, NewInt(1), NewInt(2)), S(t, NewInt(2)), "{1}", "set", ""},
		// Result type follows the left operand; probed both directions.
		{"set|frozenset", BitOr, S(t, NewInt(1)), FS(t, NewInt(2)), "{1, 2}", "set", ""},
		{"frozenset|set", BitOr, FS(t, NewInt(1)), S(t, NewInt(2)), "frozenset({1, 2})", "frozenset", ""},
		{"set&frozenset", BitAnd, S(t, NewInt(1), NewInt(2)), FS(t, NewInt(2)), "{2}", "set", ""},
		{"frozenset&set", BitAnd, FS(t, NewInt(1), NewInt(2)), S(t, NewInt(2)), "frozenset({2})", "frozenset", ""},
		{"set^frozenset", BitXor, S(t, NewInt(1), NewInt(2)), FS(t, NewInt(2)), "{1}", "set", ""},
		{"frozenset-set", Sub, FS(t, NewInt(1), NewInt(2)), S(t, NewInt(2)), "frozenset({1})", "frozenset", ""},
		{"set-frozenset", Sub, S(t, NewInt(1), NewInt(2)), FS(t, NewInt(2)), "{1}", "set", ""},
		{"set|list", BitOr, S(t, NewInt(1)), L(NewInt(2)), "", "", "TypeError: unsupported operand type(s) for |: 'set' and 'list'"},
		{"list|set", BitOr, L(NewInt(1)), S(t, NewInt(2)), "", "", "TypeError: unsupported operand type(s) for |: 'list' and 'set'"},
		{"set&list", BitAnd, S(t, NewInt(1)), L(), "", "", "TypeError: unsupported operand type(s) for &: 'set' and 'list'"},
		{"set^list", BitXor, S(t, NewInt(1)), L(), "", "", "TypeError: unsupported operand type(s) for ^: 'set' and 'list'"},
		{"set-list", Sub, S(t, NewInt(1)), L(), "", "", "TypeError: unsupported operand type(s) for -: 'set' and 'list'"},
		{"int-set", Sub, NewInt(1), S(t, NewInt(1)), "", "", "TypeError: unsupported operand type(s) for -: 'int' and 'set'"},
		{"set|int", BitOr, S(t, NewInt(1)), NewInt(1), "", "", "TypeError: unsupported operand type(s) for |: 'set' and 'int'"},
		{"frozenset|list", BitOr, FS(t, NewInt(1)), L(), "", "", "TypeError: unsupported operand type(s) for |: 'frozenset' and 'list'"},
		{"set<<int", LShift, S(t, NewInt(1)), NewInt(1), "", "", "TypeError: unsupported operand type(s) for <<: 'set' and 'int'"},
		{"int<<set", LShift, NewInt(1), S(t, NewInt(1)), "", "", "TypeError: unsupported operand type(s) for <<: 'int' and 'set'"},
		{"set>>int", RShift, S(t, NewInt(1)), NewInt(1), "", "", "TypeError: unsupported operand type(s) for >>: 'set' and 'int'"},
		{"set+set", Add, S(t, NewInt(1)), S(t, NewInt(2)), "", "", "TypeError: unsupported operand type(s) for +: 'set' and 'set'"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.a, tt.b)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
		if err == nil && got.TypeName() != tt.wantType {
			t.Errorf("%s: type = %s, want %s", tt.name, got.TypeName(), tt.wantType)
		}
	}
}

func TestSetCompare(t *testing.T) {
	tests := []struct {
		name    string
		op      CmpOp
		a, b    Object
		want    string
		wantErr string
	}{
		{"set==set", OpEq, S(t, NewInt(1), NewInt(2)), S(t, NewInt(2), NewInt(1)), "True", ""},
		// Probed: a set equals a frozenset with the same elements.
		{"set==frozenset", OpEq, S(t, NewInt(1), NewInt(2)), FS(t, NewInt(1), NewInt(2)), "True", ""},
		{"frozenset==set", OpEq, FS(t, NewInt(1)), S(t, NewInt(1)), "True", ""},
		{"set==list", OpEq, S(t, NewInt(1)), L(NewInt(1)), "False", ""},
		{"set==int", OpEq, S(t, NewInt(1)), NewInt(1), "False", ""},
		{"set!=set", OpNe, S(t, NewInt(1)), S(t, NewInt(2)), "True", ""},
		{"proper subset", OpLt, S(t, NewInt(1)), S(t, NewInt(1), NewInt(2)), "True", ""},
		{"not proper subset", OpLt, S(t, NewInt(1), NewInt(2)), S(t, NewInt(1), NewInt(2)), "False", ""},
		{"subset equal", OpLe, S(t, NewInt(1), NewInt(2)), S(t, NewInt(1), NewInt(2)), "True", ""},
		{"not subset", OpLe, S(t, NewInt(1), NewInt(3)), S(t, NewInt(1), NewInt(2)), "False", ""},
		{"proper superset", OpGt, S(t, NewInt(1), NewInt(2)), S(t, NewInt(1)), "True", ""},
		{"superset equal", OpGe, S(t, NewInt(1), NewInt(2)), S(t, NewInt(1), NewInt(2)), "True", ""},
		{"set<frozenset", OpLt, S(t, NewInt(1)), FS(t, NewInt(1), NewInt(2)), "True", ""},
		{"frozenset<set", OpLt, FS(t, NewInt(1)), S(t, NewInt(1), NewInt(2)), "True", ""},
		{"set<int", OpLt, S(t, NewInt(1)), NewInt(1), "", "TypeError: '<' not supported between instances of 'set' and 'int'"},
		{"set<list", OpLt, S(t, NewInt(1)), L(NewInt(1)), "", "TypeError: '<' not supported between instances of 'set' and 'list'"},
		{"int<set", OpLt, NewInt(1), S(t, NewInt(1)), "", "TypeError: '<' not supported between instances of 'int' and 'set'"},
		{"set>str", OpGt, S(t, NewInt(1)), NewStr("a"), "", "TypeError: '>' not supported between instances of 'set' and 'str'"},
		{"set<=list", OpLe, S(t, NewInt(1)), L(NewInt(1)), "", "TypeError: '<=' not supported between instances of 'set' and 'list'"},
		{"set>=int", OpGe, S(t, NewInt(1)), NewInt(1), "", "TypeError: '>=' not supported between instances of 'set' and 'int'"},
		{"frozenset<int", OpLt, FS(t, NewInt(1)), NewInt(1), "", "TypeError: '<' not supported between instances of 'frozenset' and 'int'"},
	}
	for _, tt := range tests {
		got, err := Compare(tt.op, tt.a, tt.b)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestSetContainsIterLenTruth(t *testing.T) {
	s := S(t, NewInt(3), NewInt(1), NewInt(2))
	f := FS(t, NewInt(1))

	tests := []struct {
		name      string
		container Object
		item      Object
		want      string
		wantErr   string
	}{
		{"1 in {1}", s, NewInt(1), "True", ""},
		{"4 in {..}", s, NewInt(4), "False", ""},
		{"True in {1}", s, True, "True", ""},
		{"1.0 in {1}", s, NewFloat(1), "True", ""},
		{"1 in frozenset", f, NewInt(1), "True", ""},
		{"list in set", s, L(), "", "TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		{"list in frozenset", f, L(), "", "TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		// Probed: a plain set is looked up as its frozen twin.
		{"set() in {frozenset()}", S(t, FS(t)), S(t), "True", ""},
		{"frozenset() in {frozenset()}", S(t, FS(t)), FS(t), "True", ""},
	}
	for _, tt := range tests {
		got, err := Contains(tt.container, tt.item)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}

	// Iteration follows insertion order, our deliberate divergence from
	// CPython's hash order.
	it, err := Iter(s)
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	var order []string
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		order = append(order, Repr(v))
	}
	if len(order) != 3 || order[0] != "3" || order[1] != "1" || order[2] != "2" {
		t.Errorf("iter order = %v, want [3 1 2]", order)
	}

	if n, err := Len(s); err != nil || n != 3 {
		t.Errorf("Len(set) = %d, %v", n, err)
	}
	if n, err := Len(f); err != nil || n != 1 {
		t.Errorf("Len(frozenset) = %d, %v", n, err)
	}
	if Truth(S(t)) || Truth(FS(t)) {
		t.Error("empty set/frozenset should be falsy")
	}
	if !Truth(s) || !Truth(f) {
		t.Error("non-empty set/frozenset should be truthy")
	}
}

func TestSetMutatingMethods(t *testing.T) {
	// add, dedup, unhashable.
	s := S(t, NewInt(1))
	if _, err := CallMethod(s, "add", []Object{NewInt(2)}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := CallMethod(s, "add", []Object{NewInt(1)}); err != nil {
		t.Fatalf("add dup: %v", err)
	}
	if got := Repr(s); got != "{1, 2}" {
		t.Errorf("after add: %q", got)
	}
	_, err := CallMethod(s, "add", []Object{L()})
	checkErr(t, "add unhashable", err, "TypeError: cannot use 'list' as a set element (unhashable type: 'list')")
	_, err = CallMethod(s, "add", []Object{NewInt(1), NewInt(2)})
	checkErr(t, "add two args", err, "TypeError: set.add() takes exactly one argument (2 given)")
	_, err = CallMethod(s, "add", nil)
	checkErr(t, "add no args", err, "TypeError: set.add() takes exactly one argument (0 given)")

	// remove: hit, miss (KeyError carries the element), unhashable.
	if _, err := CallMethod(s, "remove", []Object{NewInt(2)}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_, err = CallMethod(s, "remove", []Object{NewInt(2)})
	checkErr(t, "remove missing int", err, "KeyError: 2")
	_, err = CallMethod(s, "remove", []Object{NewStr("x")})
	checkErr(t, "remove missing str", err, "KeyError: 'x'")
	_, err = CallMethod(s, "remove", []Object{L()})
	checkErr(t, "remove unhashable", err, "TypeError: cannot use 'list' as a set element (unhashable type: 'list')")

	// remove/discard accept a plain set where a frozenset element sits.
	fs := S(t, FS(t))
	if _, err := CallMethod(fs, "remove", []Object{S(t)}); err != nil {
		t.Fatalf("remove set-as-frozenset: %v", err)
	}
	if got := Repr(fs); got != "set()" {
		t.Errorf("after remove: %q", got)
	}
	_, err = CallMethod(S(t, NewInt(1)), "remove", []Object{S(t)})
	checkErr(t, "remove missing set", err, "KeyError: set()")

	// discard: silent on missing, raises on unhashable.
	if _, err := CallMethod(s, "discard", []Object{NewInt(9)}); err != nil {
		t.Fatalf("discard missing: %v", err)
	}
	_, err = CallMethod(s, "discard", []Object{L()})
	checkErr(t, "discard unhashable", err, "TypeError: cannot use 'list' as a set element (unhashable type: 'list')")
	_, err = CallMethod(s, "discard", nil)
	checkErr(t, "discard no args", err, "TypeError: set.discard() takes exactly one argument (0 given)")

	// pop: first inserted for us, KeyError on empty.
	p := S(t, NewInt(3), NewInt(1))
	v, err := CallMethod(p, "pop", nil)
	checkRepr(t, "pop first", v, err, "3")
	if got := Repr(p); got != "{1}" {
		t.Errorf("after pop: %q", got)
	}
	_, err = CallMethod(S(t), "pop", nil)
	checkErr(t, "pop empty", err, "KeyError: 'pop from an empty set'")
	_, err = CallMethod(p, "pop", []Object{NewInt(1)})
	checkErr(t, "pop with arg", err, "TypeError: set.pop() takes no arguments (1 given)")

	// clear.
	c := S(t, NewInt(1), NewInt(2))
	if _, err := CallMethod(c, "clear", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := Repr(c); got != "set()" {
		t.Errorf("after clear: %q", got)
	}
	_, err = CallMethod(c, "clear", []Object{NewInt(1)})
	checkErr(t, "clear with arg", err, "TypeError: set.clear() takes no arguments (1 given)")

	// update: varargs, iterables, dedup, zero args is a no-op.
	u := S(t, NewInt(1))
	if _, err := CallMethod(u, "update", []Object{L(NewInt(2)), T(NewInt(3), NewInt(1))}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := Repr(u); got != "{1, 2, 3}" {
		t.Errorf("after update: %q", got)
	}
	if _, err := CallMethod(u, "update", nil); err != nil {
		t.Fatalf("update zero args: %v", err)
	}
	_, err = CallMethod(u, "update", []Object{NewInt(1)})
	checkErr(t, "update non-iterable", err, "TypeError: 'int' object is not iterable")
	_, err = CallMethod(u, "update", []Object{L(L())})
	checkErr(t, "update unhashable elt", err, "TypeError: cannot use 'list' as a set element (unhashable type: 'list')")

	// intersection_update keeps receiver order; bare hash error, probed.
	iu := S(t, NewInt(3), NewInt(1), NewInt(2))
	if _, err := CallMethod(iu, "intersection_update", []Object{L(NewInt(2), NewInt(3))}); err != nil {
		t.Fatalf("intersection_update: %v", err)
	}
	if got := Repr(iu); got != "{3, 2}" {
		t.Errorf("after intersection_update: %q", got)
	}
	_, err = CallMethod(iu, "intersection_update", []Object{L(L())})
	checkErr(t, "intersection_update unhashable", err, "TypeError: unhashable type: 'list'")

	// difference_update; wrapped error, probed.
	du := S(t, NewInt(1), NewInt(2))
	if _, err := CallMethod(du, "difference_update", []Object{L(NewInt(2))}); err != nil {
		t.Fatalf("difference_update: %v", err)
	}
	if got := Repr(du); got != "{1}" {
		t.Errorf("after difference_update: %q", got)
	}
	_, err = CallMethod(du, "difference_update", []Object{L(L())})
	checkErr(t, "difference_update unhashable", err, "TypeError: cannot use 'list' as a set element (unhashable type: 'list')")

	// symmetric_difference_update: exactly one argument.
	su := S(t, NewInt(1), NewInt(2))
	if _, err := CallMethod(su, "symmetric_difference_update", []Object{L(NewInt(2), NewInt(3))}); err != nil {
		t.Fatalf("symmetric_difference_update: %v", err)
	}
	if got := Repr(su); got != "{1, 3}" {
		t.Errorf("after symmetric_difference_update: %q", got)
	}
	_, err = CallMethod(su, "symmetric_difference_update", []Object{L(), L()})
	checkErr(t, "sd_update two args", err,
		"TypeError: set.symmetric_difference_update() takes exactly one argument (2 given)")

	// self-referential updates stay safe.
	self := S(t, NewInt(1), NewInt(2))
	if _, err := CallMethod(self, "difference_update", []Object{self}); err != nil {
		t.Fatalf("difference_update self: %v", err)
	}
	if got := Repr(self); got != "set()" {
		t.Errorf("after difference_update self: %q", got)
	}
}

func TestSetQueryMethods(t *testing.T) {
	s := S(t, NewInt(3), NewInt(1), NewInt(2))

	// copy: set copies are independent, frozenset copy is the same object.
	cp, err := CallMethod(s, "copy", nil)
	checkRepr(t, "copy", cp, err, "{3, 1, 2}")
	if cp == s {
		t.Error("set.copy() should not return the receiver")
	}
	fz := FS(t, NewInt(1))
	fcp, err := CallMethod(fz, "copy", nil)
	if err != nil || fcp != fz {
		t.Errorf("frozenset.copy() should return the receiver, got %v, %v", fcp, err)
	}
	_, err = CallMethod(s, "copy", []Object{NewInt(1)})
	checkErr(t, "copy with arg", err, "TypeError: set.copy() takes no arguments (1 given)")

	tests := []struct {
		name     string
		recv     Object
		method   string
		args     []Object
		want     string
		wantType string
		wantErr  string
	}{
		{"union zero args", s, "union", nil, "{3, 1, 2}", "set", ""},
		{"union list", S(t, NewInt(1)), "union", []Object{L(NewInt(2), NewInt(3))}, "{1, 2, 3}", "set", ""},
		{"union multi", S(t, NewInt(1)), "union", []Object{L(NewInt(2)), T(NewInt(3)), L(NewInt(1))}, "{1, 2, 3}", "set", ""},
		{"union str", S(t, NewInt(1)), "union", []Object{NewStr("ab")}, "{1, 'a', 'b'}", "set", ""},
		{"union frozenset arg", S(t, NewInt(1)), "union", []Object{FS(t, NewInt(2))}, "{1, 2}", "set", ""},
		{"union non-iterable", s, "union", []Object{NewInt(1)}, "", "", "TypeError: 'int' object is not iterable"},
		{"union unhashable elt", s, "union", []Object{L(L())}, "", "",
			"TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		{"frozenset union", FS(t, NewInt(1)), "union", []Object{L(NewInt(2))}, "frozenset({1, 2})", "frozenset", ""},
		{"intersection zero args", s, "intersection", nil, "{3, 1, 2}", "set", ""},
		{"intersection multi", s, "intersection", []Object{L(NewInt(1), NewInt(2)), L(NewInt(2), NewInt(3))}, "{2}", "set", ""},
		{"intersection bare unhashable", s, "intersection", []Object{L(L())}, "", "", "TypeError: unhashable type: 'list'"},
		{"intersection non-iterable", s, "intersection", []Object{NewInt(1)}, "", "", "TypeError: 'int' object is not iterable"},
		{"frozenset intersection", FS(t, NewInt(1), NewInt(2)), "intersection", []Object{L(NewInt(2))}, "frozenset({2})", "frozenset", ""},
		{"difference zero args", s, "difference", nil, "{3, 1, 2}", "set", ""},
		{"difference multi", s, "difference", []Object{L(NewInt(1)), L(NewInt(2))}, "{3}", "set", ""},
		{"difference wrapped unhashable", s, "difference", []Object{L(L())}, "", "",
			"TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		{"symmetric_difference", S(t, NewInt(1), NewInt(2)), "symmetric_difference", []Object{L(NewInt(2), NewInt(3))}, "{1, 3}", "set", ""},
		{"symmetric_difference zero args", s, "symmetric_difference", nil, "", "",
			"TypeError: set.symmetric_difference() takes exactly one argument (0 given)"},
		{"symmetric_difference two args", s, "symmetric_difference", []Object{L(), L()}, "", "",
			"TypeError: set.symmetric_difference() takes exactly one argument (2 given)"},
		{"issubset true", S(t, NewInt(1)), "issubset", []Object{L(NewInt(1), NewInt(2))}, "True", "bool", ""},
		{"issubset false", S(t, NewInt(1), NewInt(3)), "issubset", []Object{L(NewInt(1), NewInt(2))}, "False", "bool", ""},
		{"issubset set arg", S(t, NewInt(1)), "issubset", []Object{S(t, NewInt(1), NewInt(2))}, "True", "bool", ""},
		{"issubset zero args", s, "issubset", nil, "", "",
			"TypeError: set.issubset() takes exactly one argument (0 given)"},
		{"issubset bare unhashable", s, "issubset", []Object{L(L())}, "", "", "TypeError: unhashable type: 'list'"},
		{"issuperset true", S(t, NewInt(1), NewInt(2)), "issuperset", []Object{L(NewInt(1))}, "True", "bool", ""},
		{"issuperset false", S(t, NewInt(1)), "issuperset", []Object{L(NewInt(2))}, "False", "bool", ""},
		{"issuperset two args", s, "issuperset", []Object{L(), L()}, "", "",
			"TypeError: set.issuperset() takes exactly one argument (2 given)"},
		{"issuperset wrapped unhashable", s, "issuperset", []Object{L(L())}, "", "",
			"TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		{"isdisjoint true", S(t, NewInt(1)), "isdisjoint", []Object{L(NewInt(2))}, "True", "bool", ""},
		{"isdisjoint false", S(t, NewInt(1)), "isdisjoint", []Object{L(NewInt(1))}, "False", "bool", ""},
		{"isdisjoint empty empty", S(t), "isdisjoint", []Object{L()}, "True", "bool", ""},
		{"isdisjoint zero args", s, "isdisjoint", nil, "", "",
			"TypeError: set.isdisjoint() takes exactly one argument (0 given)"},
		{"isdisjoint wrapped unhashable", s, "isdisjoint", []Object{L(L())}, "", "",
			"TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		{"frozenset issubset msg", fz, "issubset", nil, "", "",
			"TypeError: frozenset.issubset() takes exactly one argument (0 given)"},
	}
	for _, tt := range tests {
		got, err := CallMethod(tt.recv, tt.method, tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
		if err == nil && got.TypeName() != tt.wantType {
			t.Errorf("%s: type = %s, want %s", tt.name, got.TypeName(), tt.wantType)
		}
	}

	// union returns a fresh object even with zero args; probed with is.
	u, err := CallMethod(s, "union", nil)
	if err != nil || u == s {
		t.Errorf("union() should copy, got same object: %v", err)
	}
}

func TestFrozensetNoMutators(t *testing.T) {
	f := FS(t, NewInt(1))
	for _, name := range []string{
		"add", "remove", "discard", "pop", "clear",
		"update", "intersection_update", "difference_update", "symmetric_difference_update",
	} {
		_, err := CallMethod(f, name, []Object{NewInt(1)})
		checkErr(t, "frozenset."+name, err,
			"AttributeError: 'frozenset' object has no attribute '"+name+"'")
	}
	// Unknown names too.
	_, err := CallMethod(f, "nosuch", nil)
	checkErr(t, "frozenset unknown", err, "AttributeError: 'frozenset' object has no attribute 'nosuch'")
	_, err = CallMethod(S(t), "nosuch", nil)
	checkErr(t, "set unknown", err, "AttributeError: 'set' object has no attribute 'nosuch'")
}

func TestSetMisc(t *testing.T) {
	s := S(t, NewInt(1))
	// Not subscriptable, no item assignment.
	_, err := GetItem(s, NewInt(0))
	checkErr(t, "set subscript", err, "TypeError: 'set' object is not subscriptable")
	err = SetItem(s, NewInt(0), NewInt(1))
	checkErr(t, "set item assign", err, "TypeError: 'set' object does not support item assignment")
	// Unary ops reject sets.
	_, err = Neg(s)
	checkErr(t, "neg set", err, "TypeError: bad operand type for unary -: 'set'")
	_, err = Invert(s)
	checkErr(t, "invert set", err, "TypeError: bad operand type for unary ~: 'set'")
	_, err = Mod(s, S(t, NewInt(2)))
	checkErr(t, "mod set", err, "TypeError: unsupported operand type(s) for %: 'set' and 'set'")

	// hash(set) is unhashable via the tuple path too.
	_, err = NewSet([]Object{T(S(t))})
	checkErr(t, "tuple of set elt", err, "TypeError: cannot use 'tuple' as a set element (unhashable type: 'set')")
}
