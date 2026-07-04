package objects

import "testing"

// varFn builds a *args/**kw echo function so unpacking tests can watch the
// merged argument groups arrive.
func varFn(qual string) Object {
	return NewFunction(qual, []Param{
		{Name: "args", Kind: ParamStar},
		{Name: "kw", Kind: ParamStarStar},
	}, []Object{nil, nil}, func(args []Object) (Object, error) {
		return NewTuple(args), nil
	})
}

func TestFunctionStr(t *testing.T) {
	f := varFn("f")
	if s := FunctionStr(f); s != "__main__.f()" {
		t.Errorf("function spelled %q", s)
	}
	lam := varFn("outer.<locals>.<lambda>")
	if s := FunctionStr(lam); s != "__main__.outer.<locals>.<lambda>()" {
		t.Errorf("lambda spelled %q", s)
	}
	b := NewFunc("len", 1, func(args []Object) (Object, error) { return None, nil })
	if s := FunctionStr(b); s != "len()" {
		t.Errorf("builtin spelled %q", s)
	}
	// Non-callables render as their str with no parentheses.
	if s := FunctionStr(NewInt(3)); s != "3" {
		t.Errorf("int spelled %q", s)
	}
	if s := FunctionStr(None); s != "None" {
		t.Errorf("None spelled %q", s)
	}
	if s := FunctionStr(NewList([]Object{NewInt(1)})); s != "[1]" {
		t.Errorf("list spelled %q", s)
	}
}

func TestExtendStar(t *testing.T) {
	pos, err := ExtendStar([]Object{NewInt(1)}, NewTuple([]Object{NewInt(2), NewInt(3)}))
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	if s := Repr(NewTuple(pos)); s != "(1, 2, 3)" {
		t.Errorf("extended to %s", s)
	}
	// The in-position wording carries no callee name.
	_, err = ExtendStar(nil, NewInt(7))
	checkErr(t, "extend non-iterable", err, "TypeError: Value after * must be an iterable, not int")
}

func TestStarArgsFor(t *testing.T) {
	args, err := StarArgsFor("ValueError()", NewStr("ab"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if s := Repr(NewTuple(args)); s != "('a', 'b')" {
		t.Errorf("converted to %s", s)
	}
	_, err = StarArgsFor("ValueError()", NewInt(3))
	checkErr(t, "static funcstr", err,
		"TypeError: ValueError() argument after * must be an iterable, not int")
}

func TestKwSetAndMerge(t *testing.T) {
	f := varFn("f")

	kw, err := KwSet(f, nil, "x", NewInt(1))
	if err != nil {
		t.Fatalf("first set: %v", err)
	}
	_, err = KwSet(f, kw, "x", NewInt(2))
	checkErr(t, "literal after merge collision", err,
		"TypeError: __main__.f() got multiple values for keyword argument 'x'")

	m, err2 := NewDict([]Object{NewStr("x")}, []Object{NewInt(3)})
	if err2 != nil {
		t.Fatal(err2)
	}
	_, err = KwMerge(f, kw, m)
	checkErr(t, "merge collides with literal", err,
		"TypeError: __main__.f() got multiple values for keyword argument 'x'")

	_, err = KwMerge(f, nil, NewInt(3))
	checkErr(t, "merge non-mapping", err,
		"TypeError: __main__.f() argument after ** must be a mapping, not int")
	_, err = KwMerge(f, nil, NewList(nil))
	checkErr(t, "merge list", err,
		"TypeError: __main__.f() argument after ** must be a mapping, not list")
}

func TestKwSplitRejectsNonStrKeysAtCallTime(t *testing.T) {
	f := varFn("f")
	m, err := NewDict([]Object{NewInt(1)}, []Object{NewInt(2)})
	if err != nil {
		t.Fatal(err)
	}
	// The merge itself accepts object keys; only the call rejects them.
	kw, err := KwMerge(f, nil, m)
	if err != nil {
		t.Fatalf("merge with int key: %v", err)
	}
	_, err = CallEx(f, nil, kw)
	checkErr(t, "int key at call", err, "TypeError: keywords must be strings")
}

func TestCallExRouting(t *testing.T) {
	f := varFn("f")

	got, err := CallEx(f, []Object{NewInt(1), NewInt(2)}, nil)
	if err != nil {
		t.Fatalf("positional only: %v", err)
	}
	if s := Repr(got); s != "((1, 2), {})" {
		t.Errorf("positional only bound %s", s)
	}

	kw, err := KwSet(f, nil, "k", NewInt(9))
	if err != nil {
		t.Fatal(err)
	}
	got, err = CallEx(f, []Object{NewInt(1)}, kw)
	if err != nil {
		t.Fatalf("mixed: %v", err)
	}
	if s := Repr(got); s != "((1,), {'k': 9})" {
		t.Errorf("mixed bound %s", s)
	}
}

func TestCallStarEx(t *testing.T) {
	f := varFn("f")

	got, err := CallStarEx(f, NewTuple([]Object{NewInt(1), NewInt(2)}), nil)
	if err != nil {
		t.Fatalf("lone star: %v", err)
	}
	if s := Repr(got); s != "((1, 2), {})" {
		t.Errorf("lone star bound %s", s)
	}

	// The deferred conversion names the callee.
	_, err = CallStarEx(f, NewInt(3), nil)
	checkErr(t, "lone star non-iterable", err,
		"TypeError: __main__.f() argument after * must be an iterable, not int")

	// Conversion outranks the keyword str check: probed with f(*3, **{1: 2}).
	m, err2 := NewDict([]Object{NewInt(1)}, []Object{NewInt(2)})
	if err2 != nil {
		t.Fatal(err2)
	}
	kw, err := KwMerge(f, nil, m)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CallStarEx(f, NewInt(3), kw)
	checkErr(t, "star error before key check", err,
		"TypeError: __main__.f() argument after * must be an iterable, not int")
}

func TestCallMethodStar(t *testing.T) {
	lst := NewList([]Object{NewInt(3)})
	if _, err := CallMethodStar(lst, "append", NewTuple([]Object{NewInt(4)})); err != nil {
		t.Fatalf("append: %v", err)
	}
	if s := Repr(lst); s != "[3, 4]" {
		t.Errorf("list after append %s", s)
	}
	_, err := CallMethodStar(lst, "append", NewInt(5))
	checkErr(t, "method star non-iterable", err,
		"TypeError: list.append() argument after * must be an iterable, not int")
}

func TestKwSetAndMergeMethod(t *testing.T) {
	recv := NewStr("x")

	kw, err := KwSetM(recv, "join", nil, "a", NewInt(1))
	if err != nil {
		t.Fatalf("first set: %v", err)
	}
	// A builtin-typed receiver spells the merge error as type.method().
	_, err = KwSetM(recv, "join", kw, "a", NewInt(2))
	checkErr(t, "method literal collision", err,
		"TypeError: str.join() got multiple values for keyword argument 'a'")

	m, err2 := NewDict([]Object{NewStr("a")}, []Object{NewInt(3)})
	if err2 != nil {
		t.Fatal(err2)
	}
	_, err = KwMergeM(recv, "join", kw, m)
	checkErr(t, "method merge collision", err,
		"TypeError: str.join() got multiple values for keyword argument 'a'")

	_, err = KwMergeM(recv, "join", nil, NewList(nil))
	checkErr(t, "method merge non-mapping", err,
		"TypeError: str.join() argument after ** must be a mapping, not list")
}

func TestCallMethodEx(t *testing.T) {
	// An empty keyword dict falls through to the plain method dispatch.
	lst := NewList([]Object{NewInt(1)})
	if _, err := CallMethodEx(lst, "append", []Object{NewInt(2)}, nil); err != nil {
		t.Fatalf("append: %v", err)
	}
	if s := Repr(lst); s != "[1, 2]" {
		t.Errorf("list after append %s", s)
	}

	// A non-str key survives the merge and is only rejected at call time.
	m, err := NewDict([]Object{NewInt(1)}, []Object{NewInt(2)})
	if err != nil {
		t.Fatal(err)
	}
	kw, err := KwMergeM(lst, "append", nil, m)
	if err != nil {
		t.Fatalf("merge int key: %v", err)
	}
	_, err = CallMethodEx(lst, "append", nil, kw)
	checkErr(t, "int key at call", err, "TypeError: keywords must be strings")
}

func TestCallMethodStarEx(t *testing.T) {
	// The star conversion error names the receiver method, and outranks the
	// keyword str check the same way CallStarEx does.
	lst := NewList(nil)
	m, err := NewDict([]Object{NewInt(1)}, []Object{NewInt(2)})
	if err != nil {
		t.Fatal(err)
	}
	kw, err := KwMergeM(lst, "append", nil, m)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CallMethodStarEx(lst, "append", NewInt(3), kw)
	checkErr(t, "method star error before key check", err,
		"TypeError: list.append() argument after * must be an iterable, not int")
}
