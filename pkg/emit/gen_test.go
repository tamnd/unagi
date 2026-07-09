package emit

import (
	"strings"
	"testing"
)

// TestGeneratorGolden proves the straight-line state machine: a struct with the
// discriminant and the saved fields, and a Next that switches on the discriminant,
// advances it, and returns each yield with done false, then reports done.
func TestGeneratorGolden(t *testing.T) {
	fR, _, _ := reprs()
	gen := Generator{
		Name: "pairGen",
		Elem: fR,
		Fields: []GenField{
			{Name: "a", Repr: fR},
			{Name: "b", Repr: fR},
		},
		Segments: []Segment{
			{Yield: Recv{Name: "a", Repr: fR}},
			{Yield: Recv{Name: "b", Repr: fR}},
		},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
	want := `type pairGen struct {
	state int
	a     float64
	b     float64
}

func (g *pairGen) Next() (float64, bool, error) {
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
		t.Fatalf("generator emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestGeneratorLocalNotSaved proves a value the machine computes and consumes
// inside one segment, never crossing a suspension, stays a local inside Next and
// is not lifted onto the struct. The saved field set is exactly the cross-yield
// live set (the complement of TestGeneratorGolden's assertion): x outlives the
// suspension so it is a field, t does not so it is a `:=` local in the case.
func TestGeneratorLocalNotSaved(t *testing.T) {
	fR, _, _ := reprs()
	gen := Generator{
		Name:   "sqOnce",
		Elem:   fR,
		Fields: []GenField{{Name: "x", Repr: fR}},
		Segments: []Segment{
			{
				Pre:   []Stmt{Define{Name: "t", Value: Bin{Op: OpMul, L: Recv{Name: "x", Repr: fR}, R: Recv{Name: "x", Repr: fR}}}},
				Yield: Var{Name: "t", Repr: fR},
			},
		},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "t := g.x * g.x") {
		t.Fatalf("a value live inside one segment should be a local, not a saved field:\n%s", got)
	}
	structPart := got[:strings.Index(got, "func ")]
	if !strings.Contains(structPart, "\tx ") {
		t.Fatalf("the cross-yield value x should be a saved field:\n%s", structPart)
	}
	if strings.Contains(structPart, "\tt") {
		t.Fatalf("the segment-local t must not be lifted onto the struct:\n%s", structPart)
	}
}

// TestGeneratorLoopYield proves a yield inside a `for i in range(n)` loop: the
// induction variable is a saved field, and the segment re-enters its own state so
// resumption picks up at the saved counter, never restarting the loop. The counter
// advances only after the yield is captured, so the value returned is the one from
// before the bump, and the machine leaves the loop state only when the counter
// reaches the bound.
func TestGeneratorLoopYield(t *testing.T) {
	_, iR, _ := reprs()
	gen := Generator{
		Name: "countGen",
		Elem: iR,
		Fields: []GenField{
			{Name: "n", Repr: iR},
			{Name: "i", Repr: iR},
		},
		Segments: []Segment{
			{
				Loop:  &LoopYield{Induction: "i", Bound: Recv{Name: "n", Repr: iR}},
				Yield: Recv{Name: "i", Repr: iR},
			},
		},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("loop-yield generator emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestGeneratorLoopInductionIsField proves the loop counter lives on the struct,
// not as a `:=` local in the case, because it is live across the suspension the
// yield introduces.
func TestGeneratorLoopInductionIsField(t *testing.T) {
	_, iR, _ := reprs()
	gen := Generator{
		Name:   "c",
		Elem:   iR,
		Fields: []GenField{{Name: "n", Repr: iR}, {Name: "i", Repr: iR}},
		Segments: []Segment{
			{
				Loop:  &LoopYield{Induction: "i", Bound: Recv{Name: "n", Repr: iR}},
				Yield: Recv{Name: "i", Repr: iR},
			},
		},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
	structPart := got[:strings.Index(got, "func ")]
	if !strings.Contains(structPart, "\ti ") {
		t.Fatalf("the loop counter should be a saved field:\n%s", structPart)
	}
	if strings.Contains(got, "i :=") {
		t.Fatalf("the loop counter must not be a case-local:\n%s", got)
	}
}

// TestGeneratorIfYield proves a yield inside an `if`: the guarded segment yields
// and advances only when the guard holds, and falls through to the following
// segment in the same call when it does not, so a false guard produces the next
// value with no wasted Next and the saved state still tracks the machine exactly.
func TestGeneratorIfYield(t *testing.T) {
	_, iR, _ := reprs()
	gen := Generator{
		Name: "ifGen",
		Elem: iR,
		Fields: []GenField{
			{Name: "flag", Repr: boolRepr()},
			{Name: "a", Repr: iR},
			{Name: "b", Repr: iR},
		},
		Segments: []Segment{
			{Guard: Recv{Name: "flag", Repr: boolRepr()}, Yield: Recv{Name: "a", Repr: iR}},
			{Yield: Recv{Name: "b", Repr: iR}},
		},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("if-yield generator emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestGeneratorIfYieldLast proves the last-segment case of a guarded yield: with no
// following segment to fall through to, a false guard advances straight to the done
// state, because Go forbids a fallthrough in the final clause and the machine has
// nothing left to yield.
func TestGeneratorIfYieldLast(t *testing.T) {
	_, iR, _ := reprs()
	gen := Generator{
		Name: "ifLast",
		Elem: iR,
		Fields: []GenField{
			{Name: "flag", Repr: boolRepr()},
			{Name: "a", Repr: iR},
		},
		Segments: []Segment{
			{Guard: Recv{Name: "flag", Repr: boolRepr()}, Yield: Recv{Name: "a", Repr: iR}},
		},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
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

// TestGeneratorDriveLoop proves the consume side: a static `for x in gen()` lowers
// to a Go loop that calls Next until the done flag, propagating the D14 error and
// binding the yielded element at its unboxed representation. The accumulator uses a
// float element so the add is total and the loop body carries no guard, keeping the
// drive shape itself the whole assertion.
func TestGeneratorDriveLoop(t *testing.T) {
	fR, _, _ := reprs()
	genR := Repr{Go: "*floatGen", Scalar: NotScalar}
	got, err := EmitFunc(Func{
		Name:   "sumGen",
		Params: []Param{{Name: "it", Repr: genR}},
		Ret:    fR,
		Body: []Stmt{
			Define{Name: "total", Value: Float{V: 0}},
			ForGen{
				Bind: "x",
				Elem: fR,
				Gen:  Var{Name: "it", Repr: genR},
				Body: []Stmt{AugAssign{Name: "total", Repr: fR, Value: Var{Name: "x", Repr: fR}}},
			},
			Return{Value: Var{Name: "total", Repr: fR}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"for {",
		"x, done, err := it.Next()",
		"if err != nil {",
		"if done {",
		"break",
		"total += x",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("a static generator consume should drive Next to done, missing %q:\n%s", want, got)
		}
	}
	// The element x is consumed at the unboxed float representation, so nothing at the
	// consume boundary reaches the boxed object model.
	if strings.Contains(got, "objects.") {
		t.Fatalf("a scalar generator consumed by a static for must not box the element:\n%s", got)
	}
}

// TestGeneratorComputedYield proves a segment that computes before it yields: the
// pre-statements and any guards flush inside the case, ahead of the advance and
// the return.
func TestGeneratorComputedYield(t *testing.T) {
	fR, _, _ := reprs()
	gen := Generator{
		Name:   "sqGen",
		Elem:   fR,
		Fields: []GenField{{Name: "x", Repr: fR}},
		Segments: []Segment{
			{Yield: Bin{Op: OpMul, L: Recv{Name: "x", Repr: fR}, R: Recv{Name: "x", Repr: fR}}},
		},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "return g.x * g.x, false, nil") {
		t.Fatalf("computed yield should read saved fields:\n%s", got)
	}
}

// TestGeneratorIntYieldsCoerceToFloat proves an int-valued yield coerces into a
// float element.
func TestGeneratorIntYieldsCoerceToFloat(t *testing.T) {
	fR, iR, _ := reprs()
	gen := Generator{
		Name:     "cnt",
		Elem:     fR,
		Fields:   []GenField{{Name: "n", Repr: iR}},
		Segments: []Segment{{Yield: Recv{Name: "n", Repr: iR}}},
	}
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "return float64(g.n), false, nil") {
		t.Fatalf("an int yield into a float generator should coerce:\n%s", got)
	}
}

// TestGeneratorTrailer proves trailing statements run in their own state after the
// last yield before the machine reports done.
func TestGeneratorTrailer(t *testing.T) {
	fR, _, _ := reprs()
	gen := Generator{
		Name:     "one",
		Elem:     fR,
		Fields:   []GenField{{Name: "a", Repr: fR}},
		Segments: []Segment{{Yield: Recv{Name: "a", Repr: fR}}},
		Trailer:  []Stmt{Define{Name: "last", Value: Recv{Name: "a", Repr: fR}}},
	}
	// The trailer runs in its own case after the last yield; the point is the extra
	// case and the done state past it.
	got, err := EmitGenerator(gen)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "case 1:") || !strings.Contains(got, "g.state = 2") {
		t.Fatalf("a trailer should add a case that advances to the done state:\n%s", got)
	}
}

func TestGeneratorYieldTypeMismatch(t *testing.T) {
	fR, _, _ := reprs()
	gen := Generator{
		Name:     "bad",
		Elem:     fR,
		Fields:   []GenField{{Name: "s", Repr: strR()}},
		Segments: []Segment{{Yield: Recv{Name: "s", Repr: strR()}}},
	}
	if _, err := EmitGenerator(gen); err == nil {
		t.Fatal("yielding a string into a float generator should be refused")
	}
}
