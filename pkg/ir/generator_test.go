package ir

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// genEmit lowers a parsed generator through the bridge and prints it, returning
// the emitted Go source.
func genEmit(t *testing.T, src string) string {
	t.Helper()
	fn := parseFunc(t, src)
	g, err := LowerGenerator(fn)
	if err != nil {
		t.Fatalf("LowerGenerator: %v", err)
	}
	out, err := emit.EmitGenerator(g)
	if err != nil {
		t.Fatalf("EmitGenerator: %v", err)
	}
	return out
}

// TestLowerGeneratorStraightLine proves a flat two-yield generator lowers to the
// discriminant state machine: both parameters become saved fields in source
// order, and each yield is its own segment that advances the discriminant and
// returns the field with done false.
func TestLowerGeneratorStraightLine(t *testing.T) {
	got := genEmit(t, "def pairs(a: float, b: float):\n    yield a\n    yield b\n")
	want := `type pairs struct {
	state int
	a     float64
	b     float64
}

func (g *pairs) Next() (float64, bool, error) {
	switch g.state {
	case 0:
		g.state = 1
		return g.a, false, nil
	case 1:
		g.state = 2
		return g.b, false, nil
	}
	return 0.0, true, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("generator lowering mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLowerGeneratorWithinSegmentLocal proves a value computed and consumed
// inside one segment stays a `:=` local inside the case and is not lifted onto
// the saved-field struct, while the parameter it reads is a saved field. The
// parameter reference inside the local's initializer is rewritten to g.x.
func TestLowerGeneratorWithinSegmentLocal(t *testing.T) {
	got := genEmit(t, "def sq(x: float):\n    t = x * x\n    yield t\n")
	if !strings.Contains(got, "t := g.x * g.x") {
		t.Fatalf("a value live inside one segment should be a local reading g.x:\n%s", got)
	}
	structPart := got[:strings.Index(got, "func ")]
	if !strings.Contains(structPart, "\tx ") {
		t.Fatalf("the cross-yield value x should be a saved field:\n%s", structPart)
	}
	if strings.Contains(structPart, "\tt ") {
		t.Fatalf("the segment-local t must not be lifted onto the struct:\n%s", structPart)
	}
}

// TestLowerGeneratorUnreferencedParamNotSaved proves a parameter the body never
// reads across a suspension is not carried on the saved-field struct: the frame
// holds only the cross-yield live set.
func TestLowerGeneratorUnreferencedParamNotSaved(t *testing.T) {
	got := genEmit(t, "def one(a: int, b: int):\n    yield a\n")
	structPart := got[:strings.Index(got, "func ")]
	if !strings.Contains(structPart, "\ta ") {
		t.Fatalf("the yielded parameter a should be a saved field:\n%s", structPart)
	}
	if strings.Contains(structPart, "\tb ") {
		t.Fatalf("the unread parameter b must not be a saved field:\n%s", structPart)
	}
}

// TestLowerGeneratorLoopYield proves a `for i in range(n): yield i` lowers to a
// self-resuming loop segment: the induction i is a saved field after the parameter
// n, the bound reads g.n, and the segment yields g.i. The emitted machine matches
// the hand-built countGen golden exactly.
func TestLowerGeneratorLoopYield(t *testing.T) {
	got := genEmit(t, "def countGen(n: int):\n    for i in range(n):\n        yield i\n")
	want := `type countGen struct {
	state int
	n     int64
	i     int64
}

func (g *countGen) Next() (int64, bool, error) {
	switch g.state {
	case 0:
		if g.i < g.n {
			v := g.i
			g.i++
			return v, false, nil
		}
		g.state = 1
	}
	return 0, true, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("loop-yield lowering mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLowerGeneratorIfYield proves an `if flag: yield a` followed by a plain
// `yield b` lowers to a guarded segment that falls through to the following linear
// segment when the guard is false, matching the hand-built ifGen golden.
func TestLowerGeneratorIfYield(t *testing.T) {
	got := genEmit(t, "def ifGen(flag: bool, a: int, b: int):\n    if flag:\n        yield a\n    yield b\n")
	want := `type ifGen struct {
	state int
	flag  bool
	a     int64
	b     int64
}

func (g *ifGen) Next() (int64, bool, error) {
	switch g.state {
	case 0:
		if g.flag {
			g.state = 1
			return g.a, false, nil
		}
		fallthrough
	case 1:
		g.state = 2
		return g.b, false, nil
	}
	return 0, true, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("if-yield lowering mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLowerGeneratorIfYieldLast proves a guarded yield with no following segment
// advances straight to the done state on a false guard, with no fallthrough, since
// Go forbids a fallthrough in the final switch clause.
func TestLowerGeneratorIfYieldLast(t *testing.T) {
	got := genEmit(t, "def ifLast(flag: bool, a: int):\n    if flag:\n        yield a\n")
	if strings.Contains(got, "fallthrough") {
		t.Fatalf("a guarded last segment has nothing to fall through to:\n%s", got)
	}
	want := `func (g *ifLast) Next() (int64, bool, error) {
	switch g.state {
	case 0:
		if g.flag {
			g.state = 1
			return g.a, false, nil
		}
		g.state = 1
	}
	return 0, true, nil
}`
	if !strings.Contains(got, want) {
		t.Fatalf("a false guard on the last segment should advance to done:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLowerGeneratorLoopInductionSavedUnreferenced proves the induction is saved
// even when the yield never reads it, because the loop machinery drives the counter
// regardless: `for i in range(n): yield n` still carries i as a field.
func TestLowerGeneratorLoopInductionSavedUnreferenced(t *testing.T) {
	got := genEmit(t, "def c(n: int):\n    for i in range(n):\n        yield n\n")
	structPart := got[:strings.Index(got, "func ")]
	if !strings.Contains(structPart, "\ti ") {
		t.Fatalf("the loop counter should be a saved field even when the yield ignores it:\n%s", structPart)
	}
}

func TestLowerGeneratorRefusals(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"not a generator", "def f(a: int) -> int:\n    return a\n"},
		{"yield from", "def f(a: int):\n    yield from a\n"},
		{"bare yield", "def f(a: int):\n    yield\n"},
		{"cross-segment local", "def f(a: int):\n    t = a\n    yield a\n    yield t\n"},
		{"valued return", "def f(a: int):\n    yield a\n    return a\n"},
		{"async", "async def f(a: int):\n    yield a\n"},
		{"mixed element type", "def f(a: int, b: float):\n    yield a\n    yield b\n"},
		{"non-bool guard", "def f(a: int):\n    if a:\n        yield a\n"},
		{"if-else guard", "def f(flag: bool, a: int, b: int):\n    if flag:\n        yield a\n    else:\n        yield b\n"},
		{"two-arg range", "def f(n: int):\n    for i in range(1, n):\n        yield i\n"},
		{"non-range loop", "def f(xs: list):\n    for x in xs:\n        yield x\n"},
		{"for-else", "def f(n: int):\n    for i in range(n):\n        yield i\n    else:\n        yield n\n"},
		{"multi-statement loop body", "def f(n: int):\n    for i in range(n):\n        t = i\n        yield t\n"},
		{"multi-statement guard body", "def f(flag: bool, a: int):\n    if flag:\n        t = a\n        yield t\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fn := parseFunc(t, c.src)
			if _, err := LowerGenerator(fn); err == nil {
				t.Fatalf("LowerGenerator accepted %s; want a refusal", c.name)
			}
		})
	}
}

// TestIsGenerator proves the detector: a yield anywhere marks a def a generator,
// a plain scalar function is not one.
func TestIsGenerator(t *testing.T) {
	if !IsGenerator(parseFunc(t, "def f(a: int):\n    yield a\n")) {
		t.Fatal("a def with a yield should be a generator")
	}
	if IsGenerator(parseFunc(t, "def f(a: int) -> int:\n    return a\n")) {
		t.Fatal("a plain function is not a generator")
	}
}
