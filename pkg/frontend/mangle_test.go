package frontend

import "testing"

// parseMod parses source for a mangle test, failing on a syntax error.
func parseMod(t *testing.T, src string) *Module {
	t.Helper()
	mod, err := Parse([]byte(src), "mangle_test.py")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return mod
}

// classBody returns the statements of the first top-level class named name.
func classBody(t *testing.T, mod *Module, name string) []Stmt {
	t.Helper()
	for _, s := range mod.Body {
		if c, ok := s.(*ClassDef); ok && c.Name == name {
			return c.Body
		}
	}
	t.Fatalf("class %s not found after mangling", name)
	return nil
}

func TestMangleEligible(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"__x", true},
		{"__x_", true},
		{"__spam", true},
		{"__dunder__", false},
		{"__", false},
		{"___", false},
		{"_x", false},
		{"x", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := mangleEligible(tc.name); got != tc.want {
			t.Errorf("mangleEligible(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestManglePrivateBodyIdentifiers(t *testing.T) {
	mod := parseMod(t, `
class Foo:
    __count = 0
    def __init__(self, __p):
        self.__x = __p
        global __g
        __g = self.__x
    def __helper(self):
        return self.__x
`)
	MangleClassPrivates(mod)
	body := classBody(t, mod, "Foo")

	// The class variable name is mangled.
	if a, ok := body[0].(*Assign); !ok || a.Targets[0].(*Name).Id != "_Foo__count" {
		t.Fatalf("class var target = %#v, want _Foo__count", body[0])
	}

	init := body[1].(*FuncDef)
	if init.Name != "__init__" {
		t.Errorf("dunder method name mangled to %q, want __init__ unchanged", init.Name)
	}
	if init.Params[1].Name != "_Foo__p" {
		t.Errorf("param name = %q, want _Foo__p", init.Params[1].Name)
	}
	// self.__x = __p: attribute target and bare-name value both mangle.
	asn := init.Body[0].(*Assign)
	attr := asn.Targets[0].(*Attribute)
	if attr.Name != "_Foo__x" {
		t.Errorf("attribute name = %q, want _Foo__x", attr.Name)
	}
	if v := asn.Value.(*Name).Id; v != "_Foo__p" {
		t.Errorf("value name = %q, want _Foo__p", v)
	}
	// global __g declares the mangled name.
	if g := init.Body[1].(*Global); g.Names[0] != "_Foo__g" {
		t.Errorf("global name = %q, want _Foo__g", g.Names[0])
	}

	helper := body[2].(*FuncDef)
	if helper.Name != "_Foo__helper" {
		t.Errorf("private method name = %q, want _Foo__helper", helper.Name)
	}
}

// A keyword argument name at a call site is left alone even though the
// parameter it targets mangles, which is the asymmetry CPython keeps.
func TestMangleLeavesKeywordArgName(t *testing.T) {
	mod := parseMod(t, `
class Foo:
    def m(self):
        return self.send(__a=1)
`)
	MangleClassPrivates(mod)
	body := classBody(t, mod, "Foo")
	ret := body[0].(*FuncDef).Body[0].(*Return)
	call := ret.Value.(*Call)
	if call.Args[0].Name != "__a" {
		t.Errorf("keyword arg name = %q, want __a unchanged", call.Args[0].Name)
	}
}

// A class whose name is only underscores has an empty private prefix, so
// nothing in its body mangles.
func TestMangleAllUnderscoreClassNoOp(t *testing.T) {
	mod := parseMod(t, `
class ___:
    def m(self):
        return self.__x
`)
	MangleClassPrivates(mod)
	body := classBody(t, mod, "___")
	ret := body[0].(*FuncDef).Body[0].(*Return)
	if name := ret.Value.(*Attribute).Name; name != "__x" {
		t.Errorf("attribute name = %q, want __x unchanged", name)
	}
}

// The class name's own leading underscores are stripped to form the prefix,
// so _Ledger yields _Ledger__entries, and a comprehension target inside the
// body mangles too.
func TestMangleStripsClassNameAndReachesComp(t *testing.T) {
	mod := parseMod(t, `
class _Ledger:
    def tags(self):
        return [__e for __e in self.__entries]
`)
	MangleClassPrivates(mod)
	body := classBody(t, mod, "_Ledger")
	ret := body[0].(*FuncDef).Body[0].(*Return)
	comp := ret.Value.(*Comp)
	if elt := comp.Elt.(*Name).Id; elt != "_Ledger__e" {
		t.Errorf("comp elt = %q, want _Ledger__e", elt)
	}
	if tgt := comp.Clauses[0].Target.(*Name).Id; tgt != "_Ledger__e" {
		t.Errorf("comp target = %q, want _Ledger__e", tgt)
	}
	if it := comp.Clauses[0].Iter.(*Attribute).Name; it != "_Ledger__entries" {
		t.Errorf("comp iter attr = %q, want _Ledger__entries", it)
	}
}

// Module-level private names are outside any class, so they never mangle.
func TestMangleModuleLevelUntouched(t *testing.T) {
	mod := parseMod(t, `
__top = 1
def f():
    return __top
`)
	MangleClassPrivates(mod)
	asn := mod.Body[0].(*Assign)
	if id := asn.Targets[0].(*Name).Id; id != "__top" {
		t.Errorf("module var = %q, want __top unchanged", id)
	}
}
