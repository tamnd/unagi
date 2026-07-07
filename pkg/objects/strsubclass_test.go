package objects

import "testing"

// strBaseValue is the str builtin as a class statement names it: a funcObject
// spelled "str" that renders its first argument, the conversion a value subclass
// runs to build its payload. builtinBaseName keys off the name.
func strBaseValue() Object {
	return NewFunc("str", -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return NewStr(""), nil
		}
		return NewStr(Str(args[0])), nil
	})
}

// buildStrSubclass builds `class Name(str): <names>` through the same builder a
// lowered class statement uses.
func buildStrSubclass(t *testing.T, name string, names []string, vals []Object) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{strBaseValue()}, names, vals, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "str" {
		t.Fatalf("build %s: builtinBase = %q, want str", name, cc.builtinBase)
	}
	return cc
}

func mustStrInstance(t *testing.T, c *classObject, val string) Object {
	t.Helper()
	inst, err := Instantiate(c, []Object{NewStr(val)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate %s(%q): %v", c.name, val, err)
	}
	return inst
}

func TestStrSubclassConcatReturnsPlainStr(t *testing.T) {
	c := buildStrSubclass(t, "MyStr", nil, nil)
	a := mustStrInstance(t, c, "hello")

	got, err := Add(a, NewStr(" world"))
	if err != nil || Str(got) != "hello world" {
		t.Fatalf("a + ' world' = %v, %v; want hello world", got, err)
	}
	if _, isInst := got.(*instanceObject); isInst {
		t.Fatalf("concat kept the subclass; want a plain str")
	}
	if got.TypeName() != "str" {
		t.Fatalf("type(a + s) = %q, want str", got.TypeName())
	}

	// A plain str on the left accepts a subclass instance on the right.
	pre, err := Add(NewStr("say "), a)
	if err != nil || Str(pre) != "say hello" {
		t.Fatalf("'say ' + a = %v, %v; want say hello", pre, err)
	}
}

func TestStrSubclassComparisonAndHash(t *testing.T) {
	c := buildStrSubclass(t, "MyStr", nil, nil)
	a := mustStrInstance(t, c, "hello")

	for _, tc := range []struct {
		op   CmpOp
		rhs  Object
		want Object
	}{
		{OpEq, NewStr("hello"), True},
		{OpLt, NewStr("z"), True},
		{OpNe, NewStr("hello"), False},
	} {
		got, err := Compare(tc.op, a, tc.rhs)
		if err != nil || got != tc.want {
			t.Fatalf("compare %d = %v, %v; want %v", tc.op, got, err, tc.want)
		}
	}
	// Reversed: a plain str on the left compares equal to the instance.
	eq, err := Compare(OpEq, NewStr("hello"), a)
	if err != nil || eq != True {
		t.Fatalf("'hello' == a = %v, %v; want True", eq, err)
	}

	ha, err := PyHash(a)
	if err != nil {
		t.Fatalf("hash(a): %v", err)
	}
	h, err := PyHash(NewStr("hello"))
	if err != nil {
		t.Fatalf("hash('hello'): %v", err)
	}
	if ha != h {
		t.Fatalf("hash(a) = %d, hash('hello') = %d; want equal", ha, h)
	}
}

func TestStrSubclassKeysLikeItsValue(t *testing.T) {
	c := buildStrSubclass(t, "MyStr", nil, nil)
	d := &dictObject{index: map[string]int{}}
	if err := d.set(mustStrInstance(t, c, "k"), NewInt(1)); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, found, err := d.lookup(NewStr("k"))
	if err != nil || !found || Str(v) != "1" {
		t.Fatalf("d['k'] = %v, found %v, %v; want 1", v, found, err)
	}
}

func TestStrSubclassContainerOps(t *testing.T) {
	c := buildStrSubclass(t, "MyStr", nil, nil)
	a := mustStrInstance(t, c, "hello")

	n, err := Len(a)
	if err != nil || n != 5 {
		t.Fatalf("len(a) = %d, %v; want 5", n, err)
	}
	in, err := Contains(a, NewStr("ell"))
	if err != nil || in != True {
		t.Fatalf("'ell' in a = %v, %v; want True", in, err)
	}
	ch, err := GetItem(a, NewInt(0))
	if err != nil || Str(ch) != "h" {
		t.Fatalf("a[0] = %v, %v; want h", ch, err)
	}
	it, err := Iter(a)
	if err != nil {
		t.Fatalf("iter(a): %v", err)
	}
	var runes []string
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !ok {
			break
		}
		runes = append(runes, Str(v))
	}
	if len(runes) != 5 || runes[0] != "h" || runes[4] != "o" {
		t.Fatalf("iter(a) = %v; want the 5 characters", runes)
	}
}

func TestStrSubclassInheritedMethods(t *testing.T) {
	c := buildStrSubclass(t, "MyStr", nil, nil)
	a := mustStrInstance(t, c, "hello")

	up, err := CallMethod(a, "upper", nil)
	if err != nil || Str(up) != "HELLO" {
		t.Fatalf("a.upper() = %v, %v; want HELLO", up, err)
	}
	if up.TypeName() != "str" {
		t.Fatalf("type(a.upper()) = %q, want str", up.TypeName())
	}
	rep, err := CallMethod(a, "replace", []Object{NewStr("l"), NewStr("L")})
	if err != nil || Str(rep) != "heLLo" {
		t.Fatalf("a.replace(l, L) = %v, %v; want heLLo", rep, err)
	}
	// A name that is not a str method still raises AttributeError.
	if _, err := LoadAttr(a, "no_such_method"); err == nil || !isAttrError(err) {
		t.Fatalf("a.no_such_method should be AttributeError, got %v", err)
	}
}

func TestStrSubclassStrIsUnquoted(t *testing.T) {
	c := buildStrSubclass(t, "MyStr", nil, nil)
	a := mustStrInstance(t, c, "hello")

	if Str(a) != "hello" {
		t.Fatalf("str(a) = %q, want hello", Str(a))
	}
	if r := Repr(a); r != "'hello'" {
		t.Fatalf("repr(a) = %q, want 'hello'", r)
	}
	t2, err := TruthOf(mustStrInstance(t, c, ""))
	if err != nil || t2 {
		t.Fatalf("bool(MyStr('')) = %v, %v; want false", t2, err)
	}
}
