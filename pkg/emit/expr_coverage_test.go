package emit

import (
	"strings"
	"testing"
)

// This file fills the string and comparison checklists (milestones/M4/04 and 05):
// the left-associative concat chain, the augmented string concatenation form, and
// every comparison operator emitting its Go token in context yielding bool. The
// existing cmp_test.go covers single concats and one comparison each; these widen it
// to the chain and the full operator set.

func TestStringConcatChain(t *testing.T) {
	strR := Repr{Go: "string", Scalar: SStr, Total: true}
	src := emitOneReturn(t, "join3", strR,
		[]Param{{Name: "a", Repr: strR}, {Name: "b", Repr: strR}, {Name: "c", Repr: strR}},
		Bin{Op: OpAdd, L: Bin{Op: OpAdd, L: Var{Name: "a", Repr: strR}, R: Var{Name: "b", Repr: strR}}, R: Var{Name: "c", Repr: strR}})
	if !strings.Contains(src, "return a + b + c, nil") {
		t.Fatalf("a three-way concat should be one left-associative expression:\n%s", src)
	}
}

// TestStringAugAssignConcatenates checks an augmented string assignment lowers to an
// explicit rebind through concat (s = s + piece), the total str form, not the int
// guarded path and not a wrapping numeric +=.
func TestStringAugAssignConcatenates(t *testing.T) {
	strR := Repr{Go: "string", Scalar: SStr, Total: true}
	got, err := EmitFunc(Func{
		Name:   "acc",
		Params: []Param{{Name: "p", Repr: strR}},
		Ret:    strR,
		Body: []Stmt{
			Define{Name: "s", Value: Str{V: ""}},
			AugAssign{Name: "s", Repr: strR, Value: Var{Name: "p", Repr: strR}},
			Return{Value: Var{Name: "s", Repr: strR}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `s := ""`) || !strings.Contains(got, "s = s + p") {
		t.Fatalf("string accumulation should rebind through concat:\n%s", got)
	}
	if strings.Contains(got, "rt.") {
		t.Fatalf("string concatenation is total and must emit no guard:\n%s", got)
	}
}

// TestEveryComparisonOperatorEmits proves each comparison lowers to its Go token and
// yields bool. Ordering and equality operators alike compare two ints without a
// coercion or an overflow guard, because a comparison reads values, it does not
// produce a new int.
func TestEveryComparisonOperatorEmits(t *testing.T) {
	_, iR, _ := reprs()
	boolR := boolRepr()
	cases := map[CmpOp]string{
		CmpLt: "a < b", CmpLe: "a <= b", CmpGt: "a > b",
		CmpGe: "a >= b", CmpEq: "a == b", CmpNe: "a != b",
	}
	for op, want := range cases {
		src := emitOneReturn(t, "rel", boolR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
			Cmp{Op: op, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
		if !strings.Contains(src, "return "+want+", nil") {
			t.Fatalf("%s should lower to %q yielding bool:\n%s", op, want, src)
		}
		if strings.Contains(src, "rt.") || strings.Contains(src, "float64") {
			t.Fatalf("an int comparison needs no guard or coercion:\n%s", src)
		}
	}
}

// TestConnectiveShortCircuitForm checks or/and lower to the Go connectives with the
// operands intact, the value tests the boxed excursion path does not reach here.
func TestConnectiveShortCircuitForm(t *testing.T) {
	boolR := boolRepr()
	src := emitOneReturn(t, "both", boolR, []Param{{Name: "a", Repr: boolR}, {Name: "b", Repr: boolR}},
		And{L: Var{Name: "a", Repr: boolR}, R: Var{Name: "b", Repr: boolR}})
	if !strings.Contains(src, "return a && b, nil") {
		t.Fatalf("and should lower to the Go && connective:\n%s", src)
	}
}
