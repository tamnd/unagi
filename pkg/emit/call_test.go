package emit

import (
	"strings"
	"testing"
)

// TestStaticCallGolden proves the static-to-static shape of doc 06 section 11.3:
// a direct Go call binding a value and an error, an error check that propagates,
// and the value used in the next computation.
func TestStaticCallGolden(t *testing.T) {
	fR, _, _ := reprs()
	f := Func{
		Name:   "twice",
		Params: []Param{{Name: "x", Repr: fR}},
		Ret:    fR,
		Body: []Stmt{
			Return{Value: Bin{
				Op: OpAdd,
				L:  Call{Name: "work", Args: []Expr{Var{Name: "x", Repr: fR}}, Ret: fR},
				R:  Call{Name: "work", Args: []Expr{Var{Name: "x", Repr: fR}}, Ret: fR},
			}},
		},
	}
	got, err := EmitFunc(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `func twice(x float64) (float64, error) {
	t0, exc0 := work(x)
	if exc0 != nil {
		return 0.0, exc0
	}
	t1, exc1 := work(x)
	if exc1 != nil {
		return 0.0, exc1
	}
	return t0 + t1, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("static call emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestCallErrorZeroMatchesReturn(t *testing.T) {
	_, iR, _ := reprs()
	// The error path returns the caller's zero value, which for an int return is 0.
	src := emitOneReturn(t, "call", iR, []Param{{Name: "n", Repr: iR}},
		Call{Name: "f", Args: []Expr{Var{Name: "n", Repr: iR}}, Ret: iR})
	if !strings.Contains(src, "return 0, exc0") {
		t.Fatalf("the call error path should return the int zero:\n%s", src)
	}
}
