package emit

import (
	"strings"
	"testing"
)

// This file covers the value-returning form of and/or
// (05_bool_compare_connectives.md line 28). Python's `a or b` returns an operand,
// not a coerced bool: it is a when a is truthy and b otherwise, `a and b` is a
// when a is falsy and b otherwise. When both operands are the same non-bool scalar
// the static tier selects through a runtime helper and the result is that scalar;
// two bools stay Go's own && and ||; a mixed pair has no static form and is
// refused.

func TestValueOrOnIntsSelectsThroughHelper(t *testing.T) {
	_, iR, _ := reprs()
	// `a or b` on two ints returns an int, selected by a's truthiness.
	src := emitOneReturn(t, "f", iR,
		[]Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Or{L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "return rt.OrInt64(a, b), nil") {
		t.Fatalf("or on two ints should select through the int helper, result an int:\n%s", src)
	}
}

func TestValueAndOnFloatsSelectsThroughHelper(t *testing.T) {
	fR, _, _ := reprs()
	// `a and b` on two floats returns a float, selected by a's truthiness.
	src := emitOneReturn(t, "f", fR,
		[]Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}},
		And{L: Var{Name: "a", Repr: fR}, R: Var{Name: "b", Repr: fR}})
	if !strings.Contains(src, "return rt.AndFloat64(a, b), nil") {
		t.Fatalf("and on two floats should select through the float helper:\n%s", src)
	}
}

func TestValueOrOnStringsSelectsThroughHelper(t *testing.T) {
	sR := strR()
	// `a or b` on two strings returns a string, selected by a's emptiness.
	src := emitOneReturn(t, "f", sR,
		[]Param{{Name: "a", Repr: sR}, {Name: "b", Repr: sR}},
		Or{L: Var{Name: "a", Repr: sR}, R: Var{Name: "b", Repr: sR}})
	if !strings.Contains(src, "return rt.OrStr(a, b), nil") {
		t.Fatalf("or on two strings should select through the string helper:\n%s", src)
	}
}

func TestValueConnectiveOnBoolsStaysConnective(t *testing.T) {
	// Two bools keep Go's own && with a bool result; they never route through a
	// helper.
	src := emitOneReturn(t, "f", bR(),
		[]Param{{Name: "a", Repr: bR()}, {Name: "b", Repr: bR()}},
		And{L: Var{Name: "a", Repr: bR()}, R: Var{Name: "b", Repr: bR()}})
	if !strings.Contains(src, "return a && b, nil") {
		t.Fatalf("two bools should stay a plain &&:\n%s", src)
	}
	if strings.Contains(src, "rt.And") {
		t.Fatalf("a bool connective must not route through a value helper:\n%s", src)
	}
}

func TestMixedValueConnectiveIsRefused(t *testing.T) {
	_, iR, _ := reprs()
	sR := strR()
	// An int with a string has no single static type, so the connective is refused
	// and the unit stays boxed.
	_, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: sR}},
		Ret:    iR,
		Body:   []Stmt{Return{Value: Or{L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: sR}}}},
	})
	if err == nil {
		t.Fatal("a mixed int-and-string connective must be refused, not given a forced type")
	}
}
