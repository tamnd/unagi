package objects

import "testing"

// aliasBuiltin fakes a builtin container type: a funcObject whose name is a
// subscriptable builtin, so typeReprForAlias prints it bare and calling it runs
// the recorded body.
func aliasBuiltin(name string, fn func(args []Object) (Object, error)) Object {
	return NewFunc(name, -1, fn)
}

func TestGenericAliasReprAndArgs(t *testing.T) {
	list := aliasBuiltin("list", func([]Object) (Object, error) { return NewList(nil), nil })
	dict := aliasBuiltin("dict", func([]Object) (Object, error) { return NewList(nil), nil })
	intFn := aliasBuiltin("int", func([]Object) (Object, error) { return NewInt(0), nil })
	strFn := aliasBuiltin("str", func([]Object) (Object, error) { return NewStr(""), nil })

	// A single argument reprs bare and lands in a one-tuple __args__.
	one := NewGenericAlias(list, intFn)
	if got, _ := ReprE(one); got != "list[int]" {
		t.Fatalf("repr(list[int]) = %q, want list[int]", got)
	}
	if one.TypeName() != "types.GenericAlias" {
		t.Fatalf("TypeName = %q, want types.GenericAlias", one.TypeName())
	}
	origin, err := LoadAttr(one, "__origin__")
	if err != nil || origin != list {
		t.Fatalf("__origin__ = %v, %v; want the list origin", origin, err)
	}
	args, _ := LoadAttr(one, "__args__")
	if tup, ok := args.(*tupleObject); !ok || len(tup.elts) != 1 || tup.elts[0] != intFn {
		t.Fatalf("__args__ = %v, want (int,)", args)
	}

	// A tuple argument spreads into __args__, so dict[str, int] carries two.
	two := NewGenericAlias(dict, NewTuple([]Object{strFn, intFn}))
	if got, _ := ReprE(two); got != "dict[str, int]" {
		t.Fatalf("repr(dict[str, int]) = %q, want dict[str, int]", got)
	}
}

func TestGenericAliasCallConstructsOrigin(t *testing.T) {
	called := false
	list := aliasBuiltin("list", func([]Object) (Object, error) {
		called = true
		return NewList(nil), nil
	})
	intFn := aliasBuiltin("int", func([]Object) (Object, error) { return NewInt(0), nil })

	ga := NewGenericAlias(list, intFn)
	if !Callable(ga) {
		t.Fatalf("Callable(list[int]) = false, want true")
	}
	if _, err := Call(ga, nil); err != nil {
		t.Fatalf("call list[int](): %v", err)
	}
	if !called {
		t.Fatalf("calling the alias did not construct its origin")
	}
}

func TestGenericAliasHashAndEqualByOriginArgs(t *testing.T) {
	list := aliasBuiltin("list", func([]Object) (Object, error) { return NewList(nil), nil })
	intFn := aliasBuiltin("int", func([]Object) (Object, error) { return NewInt(0), nil })
	strFn := aliasBuiltin("str", func([]Object) (Object, error) { return NewStr(""), nil })

	a := NewGenericAlias(list, intFn)
	b := NewGenericAlias(list, intFn)
	c := NewGenericAlias(list, strFn)

	if !equals(a, b) {
		t.Fatalf("list[int] != list[int]")
	}
	if equals(a, c) {
		t.Fatalf("list[int] == list[str]")
	}
	ha, err := PyHash(a)
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	hb, _ := PyHash(b)
	if ha != hb {
		t.Fatalf("equal aliases hash differently: %d vs %d", ha, hb)
	}

	// Two equal aliases share a set slot; a distinct one keeps its own.
	set, err := NewSet([]Object{a, b, c})
	if err != nil {
		t.Fatalf("build set: %v", err)
	}
	n, err := Len(set)
	if err != nil {
		t.Fatalf("len set: %v", err)
	}
	if n != 2 {
		t.Fatalf("set of {list[int], list[int], list[str]} has %d, want 2", n)
	}
}
