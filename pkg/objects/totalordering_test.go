package objects

import "testing"

// numClass builds a class with an integer field v plus __eq__ and the one
// ordering method named by root, so a test can hand it to TotalOrdering and
// watch the rest fill in. The instances compare on v.
func numClass(t *testing.T, root string) (*classObject, func(int64) *instanceObject) {
	t.Helper()
	c := mkclass(t, "Num")
	field := func(o Object) int64 {
		inst := o.(*instanceObject)
		v, _ := inst.attrGet("v")
		n, _ := AsInt(v)
		return n
	}
	c.setAttr("__eq__", NewFunction("Num.__eq__",
		[]Param{{Name: "self", Kind: ParamPlain}, {Name: "other", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) { return NewBool(field(a[0]) == field(a[1])), nil }).(*functionObject))
	cmp := map[string]func(int64, int64) bool{
		"__lt__": func(x, y int64) bool { return x < y },
		"__le__": func(x, y int64) bool { return x <= y },
		"__gt__": func(x, y int64) bool { return x > y },
		"__ge__": func(x, y int64) bool { return x >= y },
	}[root]
	c.setAttr(root, NewFunction("Num."+root,
		[]Param{{Name: "self", Kind: ParamPlain}, {Name: "other", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) { return NewBool(cmp(field(a[0]), field(a[1]))), nil }).(*functionObject))
	mk := func(v int64) *instanceObject {
		inst := &instanceObject{cls: c, attrs: newAttrs()}
		inst.attrSet("v", NewInt(v))
		return inst
	}
	return c, mk
}

func TestTotalOrderingFillsFromEachRoot(t *testing.T) {
	for _, root := range []string{"__lt__", "__le__", "__gt__", "__ge__"} {
		c, mk := numClass(t, root)
		if _, err := TotalOrdering(c); err != nil {
			t.Fatalf("TotalOrdering from %s: %v", root, err)
		}
		a, b, eq := mk(1), mk(2), mk(1)
		checks := []struct {
			op   CmpOp
			l, r *instanceObject
			want bool
		}{
			{OpLt, a, b, true}, {OpLt, b, a, false}, {OpLt, a, eq, false},
			{OpLe, a, eq, true}, {OpLe, b, a, false},
			{OpGt, b, a, true}, {OpGt, a, b, false},
			{OpGe, a, eq, true}, {OpGe, a, b, false},
		}
		for _, ck := range checks {
			got, err := Compare(ck.op, ck.l, ck.r)
			if err != nil {
				t.Fatalf("root %s compare %v: %v", root, ck.op, err)
			}
			if Truth(got) != ck.want {
				t.Errorf("root %s: compare op %v = %v, want %v", root, ck.op, Truth(got), ck.want)
			}
		}
	}
}

func TestTotalOrderingNoOrderingOp(t *testing.T) {
	c := mkclass(t, "Bad")
	c.setAttr("__eq__", NewFunction("Bad.__eq__",
		[]Param{{Name: "self", Kind: ParamPlain}, {Name: "other", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) { return True, nil }).(*functionObject))
	_, err := TotalOrdering(c)
	if err == nil {
		t.Fatal("TotalOrdering on a class with no ordering op should raise")
	}
	if e, ok := err.(*Exception); !ok || e.Text() != "must define at least one ordering operation: < > <= >=" {
		t.Fatalf("error = %v, want the ValueError", err)
	}
}

func TestTotalOrderingKeepsDefinedOps(t *testing.T) {
	// A class defining both __lt__ and __gt__ keeps its own __gt__ rather than
	// deriving one, so the root preference never overwrites a real method.
	c, mk := numClass(t, "__lt__")
	marker := NewFunction("Num.__gt__",
		[]Param{{Name: "self", Kind: ParamPlain}, {Name: "other", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) { return True, nil }).(*functionObject)
	c.setAttr("__gt__", marker)
	if _, err := TotalOrdering(c); err != nil {
		t.Fatalf("TotalOrdering: %v", err)
	}
	got, _ := c.lookup("__gt__")
	if got != marker {
		t.Fatal("TotalOrdering overwrote a user-defined __gt__")
	}
	// The user __gt__ always returns True, even where 1 > 2 is false.
	res, err := Compare(OpGt, mk(1), mk(2))
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if !Truth(res) {
		t.Error("kept __gt__ did not run")
	}
}
