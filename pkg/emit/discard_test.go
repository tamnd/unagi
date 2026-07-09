package emit

import (
	"strings"
	"testing"
)

// TestDiscardCallGolden pins the bare-expression-statement lowering of doc 06
// section 6 (statements): a call whose value is thrown away still runs and still
// threads the D14 error, so an exception it raises propagates even though the
// result is unused. The value binds to `_` rather than a temp, since nothing reads
// it, and the error check returns the caller's zero on a raise.
func TestDiscardCallGolden(t *testing.T) {
	_, iR, _ := reprs()
	f := Func{
		Name:   "run",
		Params: []Param{{Name: "x", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Discard{Value: Call{Name: "log", Args: []Expr{Var{Name: "x", Repr: iR}}, Ret: iR}},
			Return{Value: Var{Name: "x", Repr: iR}},
		},
	}
	got, err := EmitFunc(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `func run(x int64) (int64, error) {
	_, exc0 := log(x)
	if exc0 != nil {
		return 0, exc0
	}
	return x, nil
}`
	if strings.TrimSpace(got) != want {
		t.Fatalf("discard call emit mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// The discarded call binds to the blank, never to a value temp that would go
	// unread, and it never returns the value on the success path.
	if strings.Contains(got, "t0") {
		t.Errorf("a discarded call should bind to _, not a value temp:\n%s", got)
	}
}
