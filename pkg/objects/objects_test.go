package objects

import (
	"math"
	"testing"
)

func L(elts ...Object) Object { return NewList(elts) }
func T(elts ...Object) Object { return NewTuple(elts) }

func D(t *testing.T, pairs ...Object) Object {
	t.Helper()
	var keys, vals []Object
	for i := 0; i < len(pairs); i += 2 {
		keys = append(keys, pairs[i])
		vals = append(vals, pairs[i+1])
	}
	d, err := NewDict(keys, vals)
	if err != nil {
		t.Fatalf("NewDict: %v", err)
	}
	return d
}

func checkErr(t *testing.T, name string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error %q, got nil", name, want)
		return
	}
	if err.Error() != want {
		t.Errorf("%s: error = %q, want %q", name, err.Error(), want)
	}
}

func checkRepr(t *testing.T, name string, o Object, err error, want string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: unexpected error %v", name, err)
		return
	}
	if got := Repr(o); got != want {
		t.Errorf("%s: repr = %q, want %q", name, got, want)
	}
}

type binFn func(a, b Object) (Object, error)

func TestArithmetic(t *testing.T) {
	tests := []struct {
		name    string
		fn      binFn
		a, b    Object
		want    string
		wantErr string
	}{
		{"int+int", Add, NewInt(1), NewInt(2), "3", ""},
		{"int+float", Add, NewInt(1), NewFloat(2.5), "3.5", ""},
		{"float+int", Add, NewFloat(0.5), NewInt(2), "2.5", ""},
		{"bool+int", Add, True, NewInt(1), "2", ""},
		{"bool+bool", Add, True, True, "2", ""},
		{"str+str", Add, NewStr("ab"), NewStr("cd"), "'abcd'", ""},
		{"str+int", Add, NewStr("a"), NewInt(1), "", `TypeError: can only concatenate str (not "int") to str`},
		{"list+list", Add, L(NewInt(1)), L(NewInt(2)), "[1, 2]", ""},
		{"list+tuple", Add, L(NewInt(1)), T(NewInt(2)), "", `TypeError: can only concatenate list (not "tuple") to list`},
		{"tuple+tuple", Add, T(NewInt(1)), T(NewInt(2)), "(1, 2)", ""},
		{"tuple+list", Add, T(NewInt(1)), L(NewInt(2)), "", `TypeError: can only concatenate tuple (not "list") to tuple`},
		{"int+str", Add, NewInt(1), NewStr("a"), "", "TypeError: unsupported operand type(s) for +: 'int' and 'str'"},
		{"none+int", Add, None, NewInt(1), "", "TypeError: unsupported operand type(s) for +: 'NoneType' and 'int'"},
		{"int-int", Sub, NewInt(7), NewInt(9), "-2", ""},
		{"float-int", Sub, NewFloat(2.5), NewInt(1), "1.5", ""},
		{"str-str", Sub, NewStr("a"), NewStr("b"), "", "TypeError: unsupported operand type(s) for -: 'str' and 'str'"},
		{"int*int", Mul, NewInt(6), NewInt(7), "42", ""},
		{"int*float", Mul, NewInt(2), NewFloat(1.5), "3.0", ""},
		{"str*int", Mul, NewStr("ab"), NewInt(3), "'ababab'", ""},
		{"int*str", Mul, NewInt(2), NewStr("xy"), "'xyxy'", ""},
		{"str*neg", Mul, NewStr("ab"), NewInt(-1), "''", ""},
		{"str*bool", Mul, NewStr("ab"), True, "'ab'", ""},
		{"list*int", Mul, L(NewInt(1), NewInt(2)), NewInt(2), "[1, 2, 1, 2]", ""},
		{"tuple*int", Mul, T(NewInt(1)), NewInt(2), "(1, 1)", ""},
		{"str*float", Mul, NewStr("ab"), NewFloat(1), "", "TypeError: can't multiply sequence by non-int of type 'float'"},
		{"float*list", Mul, NewFloat(1), L(NewInt(1)), "", "TypeError: can't multiply sequence by non-int of type 'float'"},
		{"int/int", TrueDiv, NewInt(7), NewInt(2), "3.5", ""},
		{"int/int-exact", TrueDiv, NewInt(8), NewInt(2), "4.0", ""},
		{"int/zero", TrueDiv, NewInt(1), NewInt(0), "", "ZeroDivisionError: division by zero"},
		{"float/zero", TrueDiv, NewFloat(1), NewInt(0), "", "ZeroDivisionError: division by zero"},
		{"str/int", TrueDiv, NewStr("a"), NewInt(2), "", "TypeError: unsupported operand type(s) for /: 'str' and 'int'"},
		{"floordiv", FloorDiv, NewInt(7), NewInt(2), "3", ""},
		{"floordiv-neg", FloorDiv, NewInt(-7), NewInt(2), "-4", ""},
		{"floordiv-negdiv", FloorDiv, NewInt(7), NewInt(-2), "-4", ""},
		{"floordiv-float", FloorDiv, NewFloat(-7), NewInt(2), "-4.0", ""},
		{"floordiv-zero", FloorDiv, NewInt(7), NewInt(0), "", "ZeroDivisionError: division by zero"},
		{"floordiv-fzero", FloorDiv, NewFloat(7), NewFloat(0), "", "ZeroDivisionError: division by zero"},
		{"mod", Mod, NewInt(7), NewInt(2), "1", ""},
		{"mod-neg", Mod, NewInt(-7), NewInt(2), "1", ""},
		{"mod-negdiv", Mod, NewInt(7), NewInt(-2), "-1", ""},
		{"mod-float", Mod, NewFloat(-7.5), NewInt(2), "0.5", ""},
		{"mod-zero", Mod, NewInt(7), NewInt(0), "", "ZeroDivisionError: division by zero"},
		{"mod-fzero", Mod, NewFloat(7.5), NewInt(0), "", "ZeroDivisionError: division by zero"},
		{"pow", Pow, NewInt(2), NewInt(10), "1024", ""},
		{"pow-zeroexp", Pow, NewInt(5), NewInt(0), "1", ""},
		{"pow-negexp", Pow, NewInt(2), NewInt(-1), "0.5", ""},
		{"pow-float", Pow, NewFloat(2), NewInt(3), "8.0", ""},
		{"pow-zeroneg", Pow, NewInt(0), NewInt(-1), "", "ZeroDivisionError: zero to a negative power"},
		{"pow-str", Pow, NewStr("a"), NewInt(2), "", "TypeError: unsupported operand type(s) for ** or pow(): 'str' and 'int'"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.a, tt.b)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestUnary(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(Object) (Object, error)
		a       Object
		want    string
		wantErr string
	}{
		{"neg-int", Neg, NewInt(5), "-5", ""},
		{"neg-float", Neg, NewFloat(3.5), "-3.5", ""},
		{"neg-bool", Neg, True, "-1", ""},
		{"neg-str", Neg, NewStr("a"), "", "TypeError: bad operand type for unary -: 'str'"},
		{"pos-int", Pos, NewInt(-5), "-5", ""},
		{"pos-bool", Pos, True, "1", ""},
		{"pos-list", Pos, L(), "", "TypeError: bad operand type for unary +: 'list'"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.a)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		op      CmpOp
		a, b    Object
		want    Object
		wantErr string
	}{
		{"int-eq", OpEq, NewInt(1), NewInt(1), True, ""},
		{"int-float-eq", OpEq, NewInt(1), NewFloat(1.0), True, ""},
		{"bool-int-eq", OpEq, True, NewInt(1), True, ""},
		{"false-zero-eq", OpEq, False, NewInt(0), True, ""},
		{"int-str-eq", OpEq, NewInt(1), NewStr("1"), False, ""},
		{"int-str-ne", OpNe, NewInt(1), NewStr("1"), True, ""},
		{"none-eq", OpEq, None, None, True, ""},
		{"none-zero", OpEq, None, NewInt(0), False, ""},
		{"str-eq", OpEq, NewStr("a"), NewStr("a"), True, ""},
		{"list-eq", OpEq, L(NewInt(1), NewInt(2)), L(NewInt(1), NewFloat(2)), True, ""},
		{"list-tuple-eq", OpEq, L(NewInt(1)), T(NewInt(1)), False, ""},
		{"nested-eq", OpEq, L(T(NewInt(1))), L(T(NewInt(1))), True, ""},
		{"dict-eq", OpEq, mustDict(NewStr("a"), NewInt(1)), mustDict(NewStr("a"), NewInt(1)), True, ""},
		{"int-lt", OpLt, NewInt(1), NewInt(2), True, ""},
		{"int-float-lt", OpLt, NewInt(1), NewFloat(1.5), True, ""},
		{"bool-lt", OpLt, True, NewInt(2), True, ""},
		{"int-le", OpLe, NewInt(2), NewInt(2), True, ""},
		{"int-gt", OpGt, NewInt(3), NewInt(2), True, ""},
		{"int-ge", OpGe, NewInt(2), NewInt(3), False, ""},
		{"str-lt", OpLt, NewStr("apple"), NewStr("banana"), True, ""},
		{"str-gt", OpGt, NewStr("b"), NewStr("a"), True, ""},
		{"list-lt", OpLt, L(NewInt(1), NewInt(2)), L(NewInt(1), NewInt(3)), True, ""},
		{"list-lt-prefix", OpLt, L(NewInt(1)), L(NewInt(1), NewInt(2)), True, ""},
		{"list-le-diff", OpLe, L(NewInt(1), NewInt(3)), L(NewInt(1), NewInt(2)), False, ""},
		{"tuple-lt", OpLt, T(NewInt(1), NewInt(3)), T(NewInt(2)), True, ""},
		{"int-str-lt", OpLt, NewInt(1), NewStr("a"), nil, "TypeError: '<' not supported between instances of 'int' and 'str'"},
		{"str-int-ge", OpGe, NewStr("a"), NewInt(1), nil, "TypeError: '>=' not supported between instances of 'str' and 'int'"},
		{"none-lt", OpLt, None, None, nil, "TypeError: '<' not supported between instances of 'NoneType' and 'NoneType'"},
		{"list-elem-order-err", OpLt, L(NewInt(1)), L(NewStr("a")), nil, "TypeError: '<' not supported between instances of 'int' and 'str'"},
	}
	for _, tt := range tests {
		got, err := Compare(tt.op, tt.a, tt.b)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, Repr(got), Repr(tt.want))
		}
	}
}

func mustDict(pairs ...Object) Object {
	var keys, vals []Object
	for i := 0; i < len(pairs); i += 2 {
		keys = append(keys, pairs[i])
		vals = append(vals, pairs[i+1])
	}
	d, err := NewDict(keys, vals)
	if err != nil {
		panic(err)
	}
	return d
}

func TestTruth(t *testing.T) {
	rng := NewRange(0, 3, 1)
	empty := NewRange(0, 0, 1)
	tests := []struct {
		name string
		o    Object
		want bool
	}{
		{"none", None, false},
		{"true", True, true},
		{"false", False, false},
		{"zero", NewInt(0), false},
		{"int", NewInt(-3), true},
		{"zero-float", NewFloat(0), false},
		{"float", NewFloat(0.1), true},
		{"empty-str", NewStr(""), false},
		{"str", NewStr("x"), true},
		{"empty-list", L(), false},
		{"list", L(NewInt(0)), true},
		{"empty-tuple", T(), false},
		{"tuple", T(False), true},
		{"empty-dict", mustDict(), false},
		{"dict", mustDict(NewInt(1), NewInt(2)), true},
		{"range", rng, true},
		{"empty-range", empty, false},
		{"ellipsis", Ellipsis, true},
	}
	for _, tt := range tests {
		if got := Truth(tt.o); got != tt.want {
			t.Errorf("Truth(%s) = %v, want %v", tt.name, got, tt.want)
		}
		if got := Not(tt.o); got != NewBool(!tt.want) {
			t.Errorf("Not(%s) = %s", tt.name, Repr(got))
		}
	}
}

func TestEllipsisSingleton(t *testing.T) {
	if got := Repr(Ellipsis); got != "Ellipsis" {
		t.Errorf("Repr(Ellipsis) = %q, want Ellipsis", got)
	}
	if got := Ellipsis.TypeName(); got != "ellipsis" {
		t.Errorf("TypeName = %q, want ellipsis", got)
	}
	// The singleton is unique, so `... is ...` is true.
	if Is(Ellipsis, Ellipsis) != True {
		t.Error("Ellipsis is not identical to itself")
	}
	// It is hashable and keys a dict stably against itself.
	d := mustDict(Ellipsis, NewStr("e"))
	got, err := d.(*dictObject).get(Ellipsis)
	if err != nil || Repr(got) != "'e'" {
		t.Errorf("dict[Ellipsis] = %v err=%v, want 'e'", got, err)
	}
}

func TestFloatRepr(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{3.0, "3.0"},
		{3.5, "3.5"},
		{-2.0, "-2.0"},
		{0.0, "0.0"},
		{math.Copysign(0, -1), "-0.0"},
		{1e16, "1e+16"},
		{1.5e16, "1.5e+16"},
		{1e15, "1000000000000000.0"},
		{123456789012345.0, "123456789012345.0"},
		{1234.5, "1234.5"},
		{0.1, "0.1"},
		{0.0001, "0.0001"},
		{1e-5, "1e-05"},
		{9.999e-05, "9.999e-05"},
		{2.5e-10, "2.5e-10"},
		{1e100, "1e+100"},
		{math.Inf(1), "inf"},
		{math.Inf(-1), "-inf"},
		{math.NaN(), "nan"},
	}
	for _, tt := range tests {
		if got := Repr(NewFloat(tt.v)); got != tt.want {
			t.Errorf("Repr(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestRepr(t *testing.T) {
	tests := []struct {
		name string
		o    Object
		want string
	}{
		{"none", None, "None"},
		{"true", True, "True"},
		{"false", False, "False"},
		{"int", NewInt(-42), "-42"},
		{"str", NewStr("abc"), "'abc'"},
		{"str-squote", NewStr("it's"), `"it's"`},
		{"str-both-quotes", NewStr(`a'b"c`), `'a\'b"c'`},
		{"str-escapes", NewStr("a\nb\tc\\d"), `'a\nb\tc\\d'`},
		{"str-control", NewStr("\x00"), `'\x00'`},
		{"str-unicode", NewStr("héllo"), "'héllo'"},
		{"empty-list", L(), "[]"},
		{"list", L(NewInt(1), NewInt(2), NewInt(3)), "[1, 2, 3]"},
		{"nested", L(NewInt(1), NewStr("a"), T(NewInt(2)), L(NewFloat(3))), "[1, 'a', (2,), [3.0]]"},
		{"empty-tuple", T(), "()"},
		{"one-tuple", T(NewInt(1)), "(1,)"},
		{"tuple", T(NewInt(1), NewInt(2)), "(1, 2)"},
		{"empty-dict", mustDict(), "{}"},
		{"dict", mustDict(NewStr("a"), NewInt(1), NewStr("b"), NewInt(2)), "{'a': 1, 'b': 2}"},
		{"range-step1", NewRange(0, 5, 1), "range(0, 5)"},
		{"range-step2", NewRange(1, 10, 2), "range(1, 10, 2)"},
		{"builtin-func", NewFunc("len", -1, nil), "<built-in function len>"},
		{"builtin-type", NewFunc("list", -1, nil), "<class 'list'>"},
	}
	for _, tt := range tests {
		if got := Repr(tt.o); got != tt.want {
			t.Errorf("%s: repr = %q, want %q", tt.name, got, tt.want)
		}
	}
	if got := Str(NewStr("a'b")); got != "a'b" {
		t.Errorf("Str raw = %q", got)
	}
	if got := Str(None); got != "None" {
		t.Errorf("Str(None) = %q", got)
	}
	if got := Str(L(NewStr("a"))); got != "['a']" {
		t.Errorf("Str(list) = %q", got)
	}
}

func TestDict(t *testing.T) {
	d := mustDict(NewStr("b"), NewInt(2), NewStr("a"), NewInt(1))
	if got := Repr(d); got != "{'b': 2, 'a': 1}" {
		t.Errorf("insertion order repr = %q", got)
	}

	// Numeric keys that compare equal share a slot, first key object wins.
	d2 := mustDict(NewInt(1), NewStr("int"))
	if err := SetItem(d2, NewFloat(1.0), NewStr("float")); err != nil {
		t.Fatalf("SetItem: %v", err)
	}
	if err := SetItem(d2, True, NewStr("bool")); err != nil {
		t.Fatalf("SetItem: %v", err)
	}
	if got := Repr(d2); got != "{1: 'bool'}" {
		t.Errorf("numeric key unification repr = %q", got)
	}
	v, err := GetItem(d2, NewFloat(1.0))
	if err != nil || Str(v) != "bool" {
		t.Errorf("d[1.0] = %v, %v", v, err)
	}

	_, err = GetItem(d, NewStr("x"))
	checkErr(t, "missing key", err, "KeyError: 'x'")
	_, err = GetItem(d, NewInt(3))
	checkErr(t, "missing int key", err, "KeyError: 3")
	_, err = GetItem(d, L())
	checkErr(t, "unhashable get", err, "TypeError: cannot use 'list' as a dict key (unhashable type: 'list')")
	err = SetItem(d, mustDict(), NewInt(1))
	checkErr(t, "unhashable set", err, "TypeError: cannot use 'dict' as a dict key (unhashable type: 'dict')")
	_, err = NewDict([]Object{L()}, []Object{None})
	checkErr(t, "unhashable NewDict", err, "TypeError: cannot use 'list' as a dict key (unhashable type: 'list')")

	// Tuple keys work when their elements are hashable.
	tk := mustDict(T(NewInt(1), NewStr("a")), NewInt(9))
	v, err = GetItem(tk, T(NewInt(1), NewStr("a")))
	if err != nil || Repr(v) != "9" {
		t.Errorf("tuple key lookup = %v, %v", v, err)
	}
	_, err = NewDict([]Object{T(L())}, []Object{None})
	checkErr(t, "tuple with list", err, "TypeError: cannot use 'tuple' as a dict key (unhashable type: 'list')")

	got, err := Contains(d, NewStr("a"))
	if err != nil || got != True {
		t.Errorf("'a' in d = %v, %v", got, err)
	}
	got, err = Contains(d, NewStr("z"))
	if err != nil || got != False {
		t.Errorf("'z' in d = %v, %v", got, err)
	}
	_, err = Contains(d, L())
	checkErr(t, "unhashable in", err, "TypeError: cannot use 'list' as a dict key (unhashable type: 'list')")

	// Iteration yields keys in insertion order.
	it, err := Iter(d)
	if err != nil {
		t.Fatalf("Iter(dict): %v", err)
	}
	var keys []string
	for {
		k, ok, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		keys = append(keys, Str(k))
	}
	if len(keys) != 2 || keys[0] != "b" || keys[1] != "a" {
		t.Errorf("dict iteration keys = %v", keys)
	}
}

func TestDictMethods(t *testing.T) {
	d := mustDict(NewStr("a"), NewInt(1), NewStr("b"), NewInt(2))

	v, err := CallMethod(d, "get", []Object{NewStr("a")})
	if err != nil || Repr(v) != "1" {
		t.Errorf("get 1-arg = %v, %v", v, err)
	}
	v, err = CallMethod(d, "get", []Object{NewStr("z")})
	if err != nil || v != None {
		t.Errorf("get missing = %v, %v", v, err)
	}
	v, err = CallMethod(d, "get", []Object{NewStr("z"), NewInt(9)})
	if err != nil || Repr(v) != "9" {
		t.Errorf("get default = %v, %v", v, err)
	}

	keys, err := CallMethod(d, "keys", nil)
	if err != nil || Repr(keys) != "dict_keys(['a', 'b'])" {
		t.Errorf("keys = %s, %v", Repr(keys), err)
	}
	if keys.TypeName() != "dict_keys" {
		t.Errorf("keys type = %s", keys.TypeName())
	}
	vals, err := CallMethod(d, "values", nil)
	if err != nil || Repr(vals) != "dict_values([1, 2])" {
		t.Errorf("values = %s, %v", Repr(vals), err)
	}
	if vals.TypeName() != "dict_values" {
		t.Errorf("values type = %s", vals.TypeName())
	}
	items, err := CallMethod(d, "items", nil)
	if err != nil || Repr(items) != "dict_items([('a', 1), ('b', 2)])" {
		t.Errorf("items = %s, %v", Repr(items), err)
	}
	if items.TypeName() != "dict_items" {
		t.Errorf("items type = %s", items.TypeName())
	}

	// items iterates as tuples.
	it, err := Iter(items)
	if err != nil {
		t.Fatalf("Iter(items): %v", err)
	}
	first, ok, err := it.Next()
	if err != nil || !ok || Repr(first) != "('a', 1)" {
		t.Errorf("items first = %v, %v, %v", first, ok, err)
	}
	if n, err := Len(items); err != nil || n != 2 {
		t.Errorf("Len(items) = %d, %v", n, err)
	}

	v, err = CallMethod(d, "pop", []Object{NewStr("a")})
	if err != nil || Repr(v) != "1" {
		t.Errorf("pop = %v, %v", v, err)
	}
	if got := Repr(d); got != "{'b': 2}" {
		t.Errorf("after pop = %q", got)
	}
	_, err = CallMethod(d, "pop", []Object{NewStr("a")})
	checkErr(t, "pop missing", err, "KeyError: 'a'")
	v, err = CallMethod(d, "pop", []Object{NewStr("a"), NewInt(0)})
	if err != nil || Repr(v) != "0" {
		t.Errorf("pop default = %v, %v", v, err)
	}

	_, err = CallMethod(d, "frobnicate", nil)
	checkErr(t, "dict attr", err, "AttributeError: 'dict' object has no attribute 'frobnicate'")
}

func TestStrMethods(t *testing.T) {
	call := func(s, name string, args ...Object) (Object, error) {
		return CallMethod(NewStr(s), name, args)
	}
	tests := []struct {
		name    string
		got     func() (Object, error)
		want    string
		wantErr string
	}{
		{"upper", func() (Object, error) { return call("aBc", "upper") }, "'ABC'", ""},
		{"lower", func() (Object, error) { return call("AbC", "lower") }, "'abc'", ""},
		{"strip", func() (Object, error) { return call("  hi \n", "strip") }, "'hi'", ""},
		{"strip-cutset", func() (Object, error) { return call("xxhixx", "strip", NewStr("x")) }, "'hi'", ""},
		{"split-ws", func() (Object, error) { return call("  a  b c ", "split") }, "['a', 'b', 'c']", ""},
		{"split-empty", func() (Object, error) { return call("   ", "split") }, "[]", ""},
		{"split-sep", func() (Object, error) { return call("a,b,,c", "split", NewStr(",")) }, "['a', 'b', '', 'c']", ""},
		{"split-empty-sep", func() (Object, error) { return call("ab", "split", NewStr("")) }, "", "ValueError: empty separator"},
		{"join", func() (Object, error) { return call(", ", "join", L(NewStr("a"), NewStr("b"))) }, "'a, b'", ""},
		{"join-tuple", func() (Object, error) { return call("-", "join", T(NewStr("x"))) }, "'x'", ""},
		{"join-nonstr", func() (Object, error) { return call(",", "join", L(NewInt(1))) }, "", "TypeError: sequence item 0: expected str instance, int found"},
		{"startswith", func() (Object, error) { return call("hello", "startswith", NewStr("he")) }, "True", ""},
		{"startswith-no", func() (Object, error) { return call("hello", "startswith", NewStr("lo")) }, "False", ""},
		{"startswith-int", func() (Object, error) { return call("hello", "startswith", NewInt(1)) }, "", "TypeError: startswith first arg must be str or a tuple of str, not int"},
		{"endswith", func() (Object, error) { return call("hello", "endswith", NewStr("lo")) }, "True", ""},
		{"replace", func() (Object, error) { return call("aaa", "replace", NewStr("a"), NewStr("b")) }, "'bbb'", ""},
		{"replace-int", func() (Object, error) { return call("aaa", "replace", NewInt(1), NewStr("b")) }, "", "TypeError: replace() argument 1 must be str, not int"},
		{"find", func() (Object, error) { return call("hello", "find", NewStr("ll")) }, "2", ""},
		{"find-missing", func() (Object, error) { return call("hello", "find", NewStr("z")) }, "-1", ""},
		{"find-unicode", func() (Object, error) { return call("héllo", "find", NewStr("llo")) }, "2", ""},
		{"upper-args", func() (Object, error) { return call("a", "upper", NewInt(1)) }, "", "TypeError: str.upper() takes no arguments (1 given)"},
		{"str-attr", func() (Object, error) { return call("a", "nope") }, "", "AttributeError: 'str' object has no attribute 'nope'"},
	}
	for _, tt := range tests {
		got, err := tt.got()
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestListMethods(t *testing.T) {
	l := L(NewInt(1), NewInt(2))
	if _, err := CallMethod(l, "append", []Object{NewInt(3)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if got := Repr(l); got != "[1, 2, 3]" {
		t.Errorf("after append = %q", got)
	}

	v, err := CallMethod(l, "pop", nil)
	if err != nil || Repr(v) != "3" {
		t.Errorf("pop = %v, %v", v, err)
	}
	v, err = CallMethod(l, "pop", []Object{NewInt(0)})
	if err != nil || Repr(v) != "1" {
		t.Errorf("pop(0) = %v, %v", v, err)
	}
	_, err = CallMethod(l, "pop", []Object{NewInt(5)})
	checkErr(t, "pop oob", err, "IndexError: pop index out of range")
	_, err = CallMethod(L(), "pop", nil)
	checkErr(t, "pop empty", err, "IndexError: pop from empty list")

	l = L(NewInt(1), NewInt(3))
	if _, err := CallMethod(l, "insert", []Object{NewInt(1), NewInt(2)}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := Repr(l); got != "[1, 2, 3]" {
		t.Errorf("after insert = %q", got)
	}
	if _, err := CallMethod(l, "insert", []Object{NewInt(100), NewInt(4)}); err != nil {
		t.Fatalf("insert clamp: %v", err)
	}
	if got := Repr(l); got != "[1, 2, 3, 4]" {
		t.Errorf("after clamped insert = %q", got)
	}

	if _, err := CallMethod(l, "remove", []Object{NewInt(4)}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := Repr(l); got != "[1, 2, 3]" {
		t.Errorf("after remove = %q", got)
	}
	_, err = CallMethod(l, "remove", []Object{NewInt(9)})
	checkErr(t, "remove missing", err, "ValueError: list.remove(x): x not in list")

	v, err = CallMethod(l, "index", []Object{NewInt(2)})
	if err != nil || Repr(v) != "1" {
		t.Errorf("index = %v, %v", v, err)
	}
	_, err = CallMethod(l, "index", []Object{NewStr("a")})
	// Probed on 3.14: the message no longer names the value.
	checkErr(t, "index missing", err, "ValueError: list.index(x): x not in list")

	l2 := L(NewInt(1), NewInt(2), NewInt(1), NewFloat(1))
	v, err = CallMethod(l2, "count", []Object{NewInt(1)})
	if err != nil || Repr(v) != "3" {
		t.Errorf("count = %v, %v", v, err)
	}

	if _, err := CallMethod(l, "extend", []Object{T(NewInt(4), NewInt(5))}); err != nil {
		t.Fatalf("extend: %v", err)
	}
	if got := Repr(l); got != "[1, 2, 3, 4, 5]" {
		t.Errorf("after extend = %q", got)
	}
	_, err = CallMethod(l, "extend", []Object{NewInt(1)})
	checkErr(t, "extend int", err, "TypeError: 'int' object is not iterable")

	if _, err := CallMethod(l, "reverse", nil); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if got := Repr(l); got != "[5, 4, 3, 2, 1]" {
		t.Errorf("after reverse = %q", got)
	}

	if _, err := CallMethod(l, "sort", nil); err != nil {
		t.Fatalf("sort: %v", err)
	}
	if got := Repr(l); got != "[1, 2, 3, 4, 5]" {
		t.Errorf("after sort = %q", got)
	}
	ls := L(NewStr("b"), NewStr("a"), NewStr("c"))
	if _, err := CallMethod(ls, "sort", nil); err != nil {
		t.Fatalf("sort strs: %v", err)
	}
	if got := Repr(ls); got != "['a', 'b', 'c']" {
		t.Errorf("after str sort = %q", got)
	}
	_, err = CallMethod(L(NewStr("a"), NewInt(1)), "sort", nil)
	checkErr(t, "mixed sort", err, "TypeError: '<' not supported between instances of 'int' and 'str'")

	_, err = CallMethod(l, "frobnicate", nil)
	checkErr(t, "list attr", err, "AttributeError: 'list' object has no attribute 'frobnicate'")
	_, err = CallMethod(NewInt(1), "append", nil)
	checkErr(t, "int attr", err, "AttributeError: 'int' object has no attribute 'append'")
}

func TestItems(t *testing.T) {
	l := L(NewInt(10), NewInt(20), NewInt(30))
	v, err := GetItem(l, NewInt(-1))
	if err != nil || Repr(v) != "30" {
		t.Errorf("l[-1] = %v, %v", v, err)
	}
	_, err = GetItem(l, NewInt(3))
	checkErr(t, "list oob", err, "IndexError: list index out of range")
	_, err = GetItem(l, NewStr("a"))
	// Probed on 3.14: list, tuple and range index messages spell the type
	// bare; only the string-index message quotes it.
	checkErr(t, "list str index", err, "TypeError: list indices must be integers or slices, not str")

	if err := SetItem(l, NewInt(-3), NewInt(0)); err != nil {
		t.Fatalf("SetItem: %v", err)
	}
	if got := Repr(l); got != "[0, 20, 30]" {
		t.Errorf("after setitem = %q", got)
	}
	err = SetItem(l, NewInt(5), NewInt(0))
	checkErr(t, "list assign oob", err, "IndexError: list assignment index out of range")

	tp := T(NewInt(1), NewInt(2))
	v, err = GetItem(tp, NewInt(1))
	if err != nil || Repr(v) != "2" {
		t.Errorf("t[1] = %v, %v", v, err)
	}
	_, err = GetItem(tp, NewInt(-3))
	checkErr(t, "tuple oob", err, "IndexError: tuple index out of range")
	err = SetItem(tp, NewInt(0), NewInt(9))
	checkErr(t, "tuple setitem", err, "TypeError: 'tuple' object does not support item assignment")

	s := NewStr("héllo")
	v, err = GetItem(s, NewInt(1))
	if err != nil || Str(v) != "é" {
		t.Errorf("s[1] = %v, %v", v, err)
	}
	v, err = GetItem(s, NewInt(-1))
	if err != nil || Str(v) != "o" {
		t.Errorf("s[-1] = %v, %v", v, err)
	}
	_, err = GetItem(s, NewInt(5))
	checkErr(t, "str oob", err, "IndexError: string index out of range")
	_, err = GetItem(s, NewStr("a"))
	checkErr(t, "str str index", err, "TypeError: string indices must be integers, not 'str'")

	_, err = GetItem(NewInt(1), NewInt(0))
	checkErr(t, "int subscript", err, "TypeError: 'int' object is not subscriptable")
	err = SetItem(NewInt(1), NewInt(0), None)
	checkErr(t, "int setitem", err, "TypeError: 'int' object does not support item assignment")

	if n, err := Len(s); err != nil || n != 5 {
		t.Errorf("Len(str) = %d, %v", n, err)
	}
	if n, err := Len(l); err != nil || n != 3 {
		t.Errorf("Len(list) = %d, %v", n, err)
	}
	_, err = Len(NewInt(1))
	checkErr(t, "len int", err, "TypeError: object of type 'int' has no len()")
}

func TestContains(t *testing.T) {
	got, err := Contains(NewStr("hello"), NewStr("ell"))
	if err != nil || got != True {
		t.Errorf("substring = %v, %v", got, err)
	}
	got, err = Contains(NewStr("hello"), NewStr("z"))
	if err != nil || got != False {
		t.Errorf("substring missing = %v, %v", got, err)
	}
	_, err = Contains(NewStr("hello"), NewInt(1))
	checkErr(t, "str in int", err, "TypeError: 'in <string>' requires string as left operand, not int")

	got, err = Contains(L(NewInt(1), NewInt(2)), NewFloat(2))
	if err != nil || got != True {
		t.Errorf("2.0 in list = %v, %v", got, err)
	}
	got, err = Contains(T(NewInt(1)), NewInt(3))
	if err != nil || got != False {
		t.Errorf("3 in tuple = %v, %v", got, err)
	}
	_, err = Contains(NewInt(5), NewInt(1))
	checkErr(t, "in int", err, "TypeError: argument of type 'int' is not iterable")
}

func TestIdentity(t *testing.T) {
	tests := []struct {
		name string
		a, b Object
		want Object
	}{
		{"small-int", NewInt(5), NewInt(5), True},
		{"small-int-max", NewInt(256), NewInt(256), True},
		{"small-int-min", NewInt(-5), NewInt(-5), True},
		{"big-int", NewInt(257), NewInt(257), False},
		{"neg-int", NewInt(-6), NewInt(-6), False},
		{"none", None, None, True},
		{"bool", NewBool(true), True, True},
		{"bool-false", NewBool(false), False, True},
		{"floats", NewFloat(1), NewFloat(1), False},
		{"none-false", None, False, False},
	}
	for _, tt := range tests {
		if got := Is(tt.a, tt.b); got != tt.want {
			t.Errorf("%s: Is = %s, want %s", tt.name, Repr(got), Repr(tt.want))
		}
	}
	l := L(NewInt(1))
	if Is(l, l) != True {
		t.Error("list is not itself")
	}
	if Is(L(), L()) != False {
		t.Error("distinct lists are identical")
	}
}

func TestIterAndUnpack(t *testing.T) {
	it, err := Iter(NewStr("ab"))
	if err != nil {
		t.Fatalf("Iter(str): %v", err)
	}
	v, ok, _ := it.Next()
	if !ok || Str(v) != "a" {
		t.Errorf("str iter first = %v", v)
	}
	v, ok, _ = it.Next()
	if !ok || Str(v) != "b" {
		t.Errorf("str iter second = %v", v)
	}
	if _, ok, _ = it.Next(); ok {
		t.Error("str iter not exhausted")
	}

	_, err = Iter(NewInt(3))
	checkErr(t, "iter int", err, "TypeError: 'int' object is not iterable")

	vals, err := Unpack(T(NewInt(1), NewInt(2)), 2)
	if err != nil || len(vals) != 2 || Repr(vals[1]) != "2" {
		t.Errorf("Unpack tuple = %v, %v", vals, err)
	}
	vals, err = Unpack(NewStr("xy"), 2)
	if err != nil || Str(vals[0]) != "x" {
		t.Errorf("Unpack str = %v, %v", vals, err)
	}
	_, err = Unpack(NewInt(1), 2)
	checkErr(t, "unpack int", err, "TypeError: cannot unpack non-iterable int object")
	_, err = Unpack(L(NewInt(1), NewInt(2), NewInt(3)), 2)
	checkErr(t, "too many", err, "ValueError: too many values to unpack (expected 2, got 3)")
	_, err = Unpack(L(NewInt(1), NewInt(2)), 3)
	checkErr(t, "not enough", err, "ValueError: not enough values to unpack (expected 3, got 2)")
}

func TestUnpackEx(t *testing.T) {
	tests := []struct {
		name          string
		o             Object
		before, after int
		want          []string
		wantErr       string
	}{
		// Probed on 3.14: a, *b, c = [1, 2, 3, 4, 5] -> (1, [2, 3, 4], 5).
		{"middle", L(NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5)), 1, 1,
			[]string{"1", "[2, 3, 4]", "5"}, ""},
		// Probed on 3.14: a, *b, c = range(3) -> (0, [1], 2).
		{"range", NewRange(0, 3, 1), 1, 1, []string{"0", "[1]", "2"}, ""},
		// Probed on 3.14: *a, = [1, 2] -> a == [1, 2].
		{"star-only", L(NewInt(1), NewInt(2)), 0, 0, []string{"[1, 2]"}, ""},
		{"empty-middle", T(NewInt(1), NewInt(2)), 1, 1, []string{"1", "[]", "2"}, ""},
		{"trailing-star", L(NewInt(1), NewInt(2), NewInt(3)), 1, 0, []string{"1", "[2, 3]"}, ""},
		{"str", NewStr("abc"), 0, 1, []string{"['a', 'b']", "'c'"}, ""},
		// Probed on 3.14: a, b, *c = [1] and a, *b, c, d = [1, 2].
		{"not-enough", L(NewInt(1)), 2, 0, nil,
			"ValueError: not enough values to unpack (expected at least 2, got 1)"},
		{"not-enough-tail", L(NewInt(1), NewInt(2)), 1, 2, nil,
			"ValueError: not enough values to unpack (expected at least 3, got 2)"},
		// Probed on 3.14: a, *b = 1.
		{"non-iterable", NewInt(1), 1, 0, nil, "TypeError: cannot unpack non-iterable int object"},
	}
	for _, tt := range tests {
		vals, err := UnpackEx(tt.o, tt.before, tt.after)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if len(vals) != len(tt.want) {
			t.Errorf("%s: got %d values, want %d", tt.name, len(vals), len(tt.want))
			continue
		}
		for i, w := range tt.want {
			if got := Repr(vals[i]); got != w {
				t.Errorf("%s: vals[%d] = %s, want %s", tt.name, i, got, w)
			}
		}
	}
}

func TestCall(t *testing.T) {
	f := NewFunc("f", 2, func(args []Object) (Object, error) {
		return Add(args[0], args[1])
	})
	if f.TypeName() != "function" {
		t.Errorf("func type = %s", f.TypeName())
	}
	v, err := Call(f, []Object{NewInt(1), NewInt(2)})
	if err != nil || Repr(v) != "3" {
		t.Errorf("call = %v, %v", v, err)
	}
	_, err = Call(f, []Object{NewInt(1), NewInt(2), NewInt(3)})
	checkErr(t, "too many args", err, "TypeError: f() takes 2 positional arguments but 3 were given")
	_, err = Call(f, []Object{NewInt(1)})
	checkErr(t, "one given", err, "TypeError: f() takes 2 positional arguments but 1 was given")
	g := NewFunc("g", 1, func(args []Object) (Object, error) { return args[0], nil })
	_, err = Call(g, nil)
	checkErr(t, "zero given", err, "TypeError: g() takes 1 positional argument but 0 were given")
	_, err = Call(NewInt(1), nil)
	checkErr(t, "not callable", err, "TypeError: 'int' object is not callable")
}

func TestExceptionAndTypeNames(t *testing.T) {
	e := Raise(ValueError, "bad %s %d", "thing", 7)
	if e.Kind != "ValueError" || e.Text() != "bad thing 7" {
		t.Errorf("Raise fields = %q, %q", e.Kind, e.Text())
	}
	if e.Error() != "ValueError: bad thing 7" {
		t.Errorf("Error() = %q", e.Error())
	}
	if got := e.TypeName(); got != "ValueError" {
		t.Errorf("TypeName() = %q", got)
	}

	names := map[Object]string{
		None:                 "NoneType",
		True:                 "bool",
		NewInt(1):            "int",
		NewFloat(1):          "float",
		NewStr(""):           "str",
		L():                  "list",
		T():                  "tuple",
		mustDict():           "dict",
		NewRange(0, 1, 1):    "range",
		NewFunc("f", 0, nil): "function",
	}
	for o, want := range names {
		if got := o.TypeName(); got != want {
			t.Errorf("TypeName = %q, want %q", got, want)
		}
	}
}
