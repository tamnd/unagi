package ir

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// parseFunc parses a single top-level def and returns it, failing the test if
// the source does not parse to exactly one function.
func parseFunc(t *testing.T, src string) *frontend.FuncDef {
	t.Helper()
	mod, err := frontend.Parse([]byte(src), "test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(mod.Body) != 1 {
		t.Fatalf("want one top-level statement, got %d", len(mod.Body))
	}
	fn, ok := mod.Body[0].(*frontend.FuncDef)
	if !ok {
		t.Fatalf("top-level statement is %T, want *frontend.FuncDef", mod.Body[0])
	}
	return fn
}

// emitOf lowers a parsed function through the bridge and prints it, returning the
// emitted Go source.
func emitOf(t *testing.T, src string) string {
	t.Helper()
	fn := parseFunc(t, src)
	f, err := LowerFunc(fn)
	if err != nil {
		t.Fatalf("LowerFunc: %v", err)
	}
	out, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	return out
}

func TestLowerIntArithmeticGuardsOverflow(t *testing.T) {
	src := "def add(a: int, b: int) -> int:\n    return a + b\n"
	got := emitOf(t, src)
	// Two ints go through the overflow-checked helper, and the failure edge tail
	// calls the unit's deopt handler.
	for _, want := range []string{
		"func add(a int64, b int64) (int64, error)",
		"rt.AddInt64(a, b)",
		"add_deopt0(a, b)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted int add is missing %q:\n%s", want, got)
		}
	}
}

func TestLowerFloatArithmeticIsTotal(t *testing.T) {
	src := "def fadd(a: float, b: float) -> float:\n    return a + b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "func fadd(a float64, b float64) (float64, error)") {
		t.Errorf("float signature missing:\n%s", got)
	}
	if !strings.Contains(got, "return a + b, nil") {
		t.Errorf("float add should be a bare total operator:\n%s", got)
	}
	if strings.Contains(got, "AddInt64") || strings.Contains(got, "deopt") {
		t.Errorf("float add must not guard overflow or deopt:\n%s", got)
	}
}

func TestLowerTrueDivisionGuardsZero(t *testing.T) {
	src := "def q(a: int, b: int) -> float:\n    return a / b\n"
	got := emitOf(t, src)
	// True division is float, so the int operands coerce, and a zero divisor is a
	// semantic ZeroDivisionError, not a deopt.
	for _, want := range []string{
		"(float64, error)",
		"float64(a) / float64(b)",
		"ZeroDivisionError",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted division is missing %q:\n%s", want, got)
		}
	}
}

func TestLowerMixedArithmeticPromotesToFloat(t *testing.T) {
	// An int local added to a float parameter promotes the result to float, so the
	// function's return type is float64.
	src := "def mix(x: float) -> float:\n    n = 2\n    return x + n\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "(float64, error)") {
		t.Errorf("mixed arithmetic should return float64:\n%s", got)
	}
	if !strings.Contains(got, "n := int64(2)") {
		t.Errorf("the int local should bind as int64:\n%s", got)
	}
}

func TestLowerAugAssignAccumulates(t *testing.T) {
	src := "def acc(a: int, b: int) -> int:\n    a += b\n    return a\n"
	got := emitOf(t, src)
	// += on an int target accumulates through the guarded add, so the overflow
	// helper appears and the result rebinds a.
	if !strings.Contains(got, "rt.AddInt64(a, b)") {
		t.Errorf("int += should route through the overflow helper:\n%s", got)
	}
	if !strings.Contains(got, "a = ") {
		t.Errorf("int += should rebind the target:\n%s", got)
	}
}

// TestLowerPassEmitsNothing proves 06 line 54: `pass` is a no-op that lowers to no
// statement at all, so it changes no control flow. A function whose body is a `pass`
// followed by a return emits exactly the return, with no artifact standing in for the
// pass.
func TestLowerPassEmitsNothing(t *testing.T) {
	got := emitOf(t, "def f(n: int) -> int:\n    pass\n    return n\n")
	if !strings.Contains(got, "return n, nil") {
		t.Fatalf("the body should lower to its return:\n%s", got)
	}
	// The only statement in the body is the return; a stray pass artifact would add a
	// second line, so the emitted body carries exactly one statement.
	if n := strings.Count(got, "\n\treturn n, nil"); n != 1 {
		t.Fatalf("pass should emit nothing, leaving only the return:\n%s", got)
	}
	if strings.Contains(got, "pass") || strings.Contains(got, "_ = ") {
		t.Fatalf("pass should leave no artifact in the emitted Go:\n%s", got)
	}
}

func TestLowerStringConcatenation(t *testing.T) {
	src := "def greet(a: str, b: str) -> str:\n    return a + b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "(string, error)") {
		t.Errorf("string concat should return string:\n%s", got)
	}
	if !strings.Contains(got, "return a + b, nil") {
		t.Errorf("string concat should be a total +:\n%s", got)
	}
}

func TestLowerReturnTypeInferredFromBody(t *testing.T) {
	// No return annotation: the result type comes from the returned expression.
	src := "def f(a: int, b: int):\n    return a * b\n"
	fn := parseFunc(t, src)
	f, err := LowerFunc(fn)
	if err != nil {
		t.Fatalf("LowerFunc: %v", err)
	}
	if f.Ret.Scalar != emit.SInt {
		t.Errorf("inferred return repr = %s, want int", f.Ret.Scalar)
	}
}

func TestLowerComparison(t *testing.T) {
	// A numeric comparison is a total Go operator yielding bool, no guard even for
	// int operands: a compare reads values, it never produces a new int to overflow.
	src := "def lt(a: int, b: int) -> bool:\n    return a < b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "(bool, error)") {
		t.Errorf("comparison should return bool:\n%s", got)
	}
	if !strings.Contains(got, "return a < b, nil") {
		t.Errorf("int comparison should be a bare operator:\n%s", got)
	}
	if strings.Contains(got, "AddInt64") || strings.Contains(got, "deopt") {
		t.Errorf("a comparison must not guard overflow or deopt:\n%s", got)
	}
}

func TestLowerChainedComparison(t *testing.T) {
	// Python expands a < b < c into the left-to-right conjunction a < b and b < c.
	src := "def between(a: int, b: int, c: int) -> bool:\n    return a < b < c\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "a < b && b < c") {
		t.Errorf("chained comparison should expand to a conjunction:\n%s", got)
	}
}

func TestLowerConnectivePrecedence(t *testing.T) {
	// and binds tighter than or, so the emitted form parenthesizes the and to keep
	// Python's precedence when Go reparses the tree (05, line 25).
	src := "def f(a: bool, b: bool, c: bool) -> bool:\n    return a or b and c\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "a || (b && c)") {
		t.Errorf("or-of-and should parenthesize the and:\n%s", got)
	}
}

func TestLowerNotPrecedence(t *testing.T) {
	// not is lower than ==, so `not a == b` is `not (a == b)`, and the emitted !
	// parenthesizes the comparison so it does not read as `!a == b` (05, line 26).
	src := "def f(a: int, b: int) -> bool:\n    return not a == b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "!(a == b)") {
		t.Errorf("not of a comparison should parenthesize it:\n%s", got)
	}
}

func TestLowerRefusesBoolOrdering(t *testing.T) {
	// Ordering two bools has no static form: True > False is a CPython coercion, not
	// a Go bool operator, so the unit stays boxed (05, line 12).
	src := "def f(a: bool, b: bool) -> bool:\n    return a < b\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("ordering bools should be refused, keeping the unit boxed")
	}
}

func TestLowerValueReturningOrOnInts(t *testing.T) {
	// Python `x or y` on two ints returns an int operand, not a bool: `a or b` is a
	// when a is truthy and b otherwise. When both operands share the int scalar the
	// static tier selects through the runtime helper and the result is an int (05,
	// line 28).
	src := "def f(a: int, b: int) -> int:\n    return a or b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "rt.OrInt64(a, b)") {
		t.Fatalf("value-returning or on ints should select through the int helper:\n%s", got)
	}
}

func TestLowerValueReturningAndOnFloats(t *testing.T) {
	// `a and b` on two floats returns a float: a when a is falsy and b otherwise.
	src := "def f(a: float, b: float) -> float:\n    return a and b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "rt.AndFloat64(a, b)") {
		t.Fatalf("value-returning and on floats should select through the float helper:\n%s", got)
	}
}

func TestLowerValueReturningOrOnStrings(t *testing.T) {
	// `a or b` on two strings returns a string: a when a is non-empty and b otherwise.
	src := "def f(a: str, b: str) -> str:\n    return a or b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "rt.OrStr(a, b)") {
		t.Fatalf("value-returning or on strings should select through the string helper:\n%s", got)
	}
}

func TestLowerRefusesMixedValueConnective(t *testing.T) {
	// `a or b` with an int and a string has no single static type, so the unit stays
	// boxed rather than force one operand's type onto the other (05, line 28).
	src := "def f(a: int, b: str) -> int:\n    return a or b\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a mixed-type value connective should be refused, keeping the unit boxed")
	}
}

func TestLowerRebindsExistingNameWithAssign(t *testing.T) {
	// The first binding of x declares it; the second binds the same name again, which
	// lowers to a plain assignment, not a second declaration (06, line 9).
	src := "def f(a: float, b: float) -> float:\n    x = a * 2.0\n    x = x + b\n    return x\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "x := a * 2") {
		t.Fatalf("the first binding should declare with :=\n%s", got)
	}
	if !strings.Contains(got, "x = x + b") {
		t.Fatalf("the rebinding should reassign with a plain =\n%s", got)
	}
	if strings.Count(got, "x :=") != 1 {
		t.Fatalf("x should be declared exactly once:\n%s", got)
	}
}

func TestLowerRefusesTypeChangingRebind(t *testing.T) {
	// Python lets x hold an int and then a string, but Go fixes a variable's type at
	// its declaration, so a rebinding to a different scalar has no static form and the
	// unit stays boxed (06, line 9).
	src := "def f(a: int, b: str) -> str:\n    x = a\n    x = b\n    return x\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a type-changing rebind should be refused, keeping the unit boxed")
	}
}

func TestLowerTupleUnpackDeclaresInParallel(t *testing.T) {
	// `x, y = a, b` binds two fresh names from a tuple, lowering to Go's parallel
	// declaration (06, line 11).
	src := "def f(a: float, b: float) -> float:\n    x, y = a, b\n    return x + y\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "x, y := a, b") {
		t.Fatalf("a fresh tuple unpack should declare both names in one parallel :=\n%s", got)
	}
}

func TestLowerTupleSwapReassignsInParallel(t *testing.T) {
	// `x, y = y, x` on two already-bound names swaps them through Go's parallel
	// assignment, which evaluates the whole right side before binding, so no temp is
	// needed and each value is read once (06, line 11).
	src := "def f(a: float, b: float) -> float:\n    x = a\n    y = b\n    x, y = y, x\n    return x\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "x, y = y, x") {
		t.Fatalf("a rebinding tuple unpack should reassign both names in one parallel =\n%s", got)
	}
}

func TestLowerRefusesTupleUnpackOfNonTuple(t *testing.T) {
	// `x, y = xs` unpacks an iterable value, which has no static form at M4, so the
	// unit stays boxed (06, line 11).
	src := "def f(xs: list) -> float:\n    x, y = xs\n    return x\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("unpacking a non-tuple value should be refused, keeping the unit boxed")
	}
}

func TestLowerRefusesTupleUnpackLengthMismatch(t *testing.T) {
	// A three-name target for a two-value tuple is a Python unpack error; it has no
	// static form and stays boxed (06, line 11).
	src := "def f(a: float, b: float) -> float:\n    x, y, z = a, b\n    return x\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a length-mismatched tuple unpack should be refused")
	}
}

func TestLowerRejects(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"unannotated param", "def f(a, b: int) -> int:\n    return b\n"},
		{"non-scalar annotation", "def f(a: list) -> int:\n    return 1\n"},
		{"default-valued param", "def f(a: int = 1) -> int:\n    return a\n"},
		{"star args", "def f(a: int, *args: int) -> int:\n    return a\n"},
		{"double star kwargs", "def f(a: int, **kw: int) -> int:\n    return a\n"},
		{"keyword-only param", "def f(a: int, *, b: int) -> int:\n    return b\n"},
		{"big int literal", "def f() -> int:\n    return 100000000000000000000\n"},
		{"call expression", "def f(a: int) -> int:\n    return g(a)\n"},
		{"first-class function value", "def f(a: int) -> int:\n    return g(f)\n"},
		{"floor division", "def f(a: int, b: int) -> int:\n    return a // b\n"},
		{"attribute access", "def f(a: int) -> int:\n    return a.bit_length\n"},
		{"async def", "async def f(a: int) -> int:\n    return a\n"},
		{"no return", "def f(a: int) -> int:\n    a += 1\n"},
		{"return annotation mismatch", "def f(a: float) -> int:\n    return a\n"},
		{"forward reference", "def f(a: int) -> int:\n    return c\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fn := parseFunc(t, c.src)
			if _, err := LowerFunc(fn); err == nil {
				t.Fatalf("%s: LowerFunc should have refused the unit", c.name)
			}
		})
	}
}
