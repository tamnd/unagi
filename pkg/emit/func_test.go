package emit

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

// floatR, intR, and listFloatR are the representations the model builders reuse.
func reprs() (float64R, intR, listFloatR Repr) {
	in := types.NewInterner()
	f, _ := Of(in.Float())
	i, _ := Of(in.Int())
	l, _ := Of(in.List(in.Float()))
	return f, i, l
}

// TestSumSqGolden is the canonical float loop of doc 06 section 2.1 without the
// trailing math.sqrt call, which is a static-to-static call from slice 9. It
// proves the total float path: bare Go operators, no guard, += accumulation.
func TestSumSqGolden(t *testing.T) {
	fR, _, listR := reprs()
	f := Func{
		Name:   "sumsq",
		Params: []Param{{Name: "xs", Repr: listR}},
		Ret:    fR,
		Body: []Stmt{
			Define{Name: "total", Value: Float{V: 0}},
			ForRange{
				Bind: "x",
				Over: Var{Name: "xs", Repr: listR},
				Body: []Stmt{
					AugAssign{Name: "total", Repr: fR, Value: Bin{Op: OpMul, L: Var{Name: "x", Repr: fR}, R: Var{Name: "x", Repr: fR}}},
				},
			},
			Return{Value: Var{Name: "total", Repr: fR}},
		},
	}
	got, err := EmitFunc(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `func sumsq(xs []float64) (float64, error) {
	total := 0.0
	for _, x := range xs {
		total += x * x
	}
	return total, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("sumsq emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestPolyOverflowGolden proves the guarded int path: a*a and +b each compute
// through an overflow-checked helper, and each failure edge routes to the unit's
// next deopt handler with the parameters replayed.
func TestPolyOverflowGolden(t *testing.T) {
	_, iR, _ := reprs()
	f := Func{
		Name:   "poly",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Return{Value: Bin{
				Op: OpAdd,
				L:  Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "a", Repr: iR}},
				R:  Var{Name: "b", Repr: iR},
			}},
		},
	}
	got, err := EmitFunc(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `func poly(a int64, b int64) (int64, error) {
	t0, ovf0 := rt.MulInt64(a, a)
	if ovf0 {
		return poly_deopt0(a, b)
	}
	t1, ovf1 := rt.AddInt64(t0, b)
	if ovf1 {
		return poly_deopt1(a, b)
	}
	return t1, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("poly emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestPolyDeoptHandlerGolden proves the build-wired deopt edge: with a named
// handler, every overflow guard tail-calls that one function with the unit's
// parameters replayed, in place of the per-site placeholder. For a straight-line
// unit the live state at each guard is the same parameters, so one handler serves
// both sites and the emitter no longer mints distinct deopt names.
func TestPolyDeoptHandlerGolden(t *testing.T) {
	_, iR, _ := reprs()
	f := Func{
		Name:         "poly",
		DeoptHandler: "static_poly_deopt",
		Params:       []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Ret:          iR,
		Body: []Stmt{
			Return{Value: Bin{
				Op: OpAdd,
				L:  Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "a", Repr: iR}},
				R:  Var{Name: "b", Repr: iR},
			}},
		},
	}
	got, err := EmitFunc(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `func poly(a int64, b int64) (int64, error) {
	d0, d1 := a, b
	t0, ovf0 := rt.MulInt64(a, a)
	if ovf0 {
		return static_poly_deopt(d0, d1)
	}
	t1, ovf1 := rt.AddInt64(t0, b)
	if ovf1 {
		return static_poly_deopt(d0, d1)
	}
	return t1, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("poly deopt-handler emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
