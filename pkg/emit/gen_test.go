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
