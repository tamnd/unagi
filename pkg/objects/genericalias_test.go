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

// TestGenericAliasSubclassable proves a class can derive from types.GenericAlias:
// the base is recognized, an instance wraps the parameterized generic as its
// payload, and __origin__/__args__ read through it. This is the _CallableGeneric-
// Alias enabler _collections_abc leans on.
func TestGenericAliasSubclassable(t *testing.T) {
	gaType := TypeSingleton("types.GenericAlias")
	if name, ok := builtinBaseName(gaType); !ok || name != "types.GenericAlias" {
		t.Fatalf("builtinBaseName(GenericAlias) = %q, %v; want types.GenericAlias, true", name, ok)
	}

	subObj, err := NewClass("Sub", "Sub", []Object{gaType}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("subclass GenericAlias: %v", err)
	}
	sub := subObj.(*classObject)
	if sub.builtinBase != "types.GenericAlias" {
		t.Fatalf("subclass builtinBase = %q, want types.GenericAlias", sub.builtinBase)
	}

	list := aliasBuiltin("list", func([]Object) (Object, error) { return NewList(nil), nil })
	intFn := aliasBuiltin("int", func([]Object) (Object, error) { return NewInt(0), nil })
	strFn := aliasBuiltin("str", func([]Object) (Object, error) { return NewStr(""), nil })

	// Sub(list, (int, str)) wraps list[int, str]; __args__ spreads the tuple.
	inst, err := Instantiate(sub, []Object{list, NewTuple([]Object{intFn, strFn})}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate Sub: %v", err)
	}
	origin, err := LoadAttr(inst, "__origin__")
	if err != nil || origin != list {
		t.Fatalf("__origin__ = %v, %v; want the list origin", origin, err)
	}
	args, err := LoadAttr(inst, "__args__")
	if err != nil {
		t.Fatalf("__args__: %v", err)
	}
	if tup, ok := args.(*tupleObject); !ok || len(tup.elts) != 2 || tup.elts[0] != intFn || tup.elts[1] != strFn {
		t.Fatalf("__args__ = %v, want (int, str)", args)
	}

	// The wrong argument count is the same TypeError the explicit constructor gives.
	if _, err := Instantiate(sub, []Object{list}, nil, nil); err == nil {
		t.Fatalf("Sub(list) with one argument did not raise")
	}
}
