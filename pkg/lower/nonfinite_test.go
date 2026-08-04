package lower

import (
	"go/format"
	"go/token"
	"math"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// render prints a lowered expression the way the module writer does, so a test
// can assert the exact Go text a literal lowers to.
func render(t *testing.T, e any) string {
	t.Helper()
	node, ok := e.(interface{ Pos() token.Pos })
	if !ok {
		t.Fatalf("floatLit returned %T, not a go/ast node", e)
	}
	var buf strings.Builder
	if err := format.Node(&buf, token.NewFileSet(), node); err != nil {
		t.Fatalf("format floatLit output: %v", err)
	}
	return buf.String()
}

// TestFloatLitNonFinite pins the spelling of a non-finite float literal. Go has
// no literal for an infinity or a NaN (strconv prints +Inf/-Inf/NaN, none of
// which parse as a Go float), so the lowering emits an objects helper call that
// returns the float64 at run time; a finite value stays a plain literal.
func TestFloatLitNonFinite(t *testing.T) {
	cases := []struct {
		name string
		v    float64
		want string
	}{
		{"pos_inf", math.Inf(1), "objects.FloatInf(1)"},
		{"neg_inf", math.Inf(-1), "objects.FloatInf(-1)"},
		{"nan", math.NaN(), "objects.FloatNaN()"},
		{"finite", 1.5, "1.5"},
		// A bare 0 is fine here: floatLit always feeds objects.NewFloat/NewComplex,
		// whose float64 parameter converts the untyped constant, so this tier does
		// not force the decimal point the static-scalar tier needs.
		{"zero", 0, "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := render(t, floatLit(tc.v)); got != tc.want {
				t.Errorf("floatLit(%v) = %q, want %q", tc.v, got, tc.want)
			}
		})
	}
}

// TestOverflowFloatLiteralLowers checks a Python float literal that overflows
// the double range folds to an infinity at compile time and still lowers to a
// buildable module rather than an invalid Go +Inf literal, the regression that
// blocked test_math and test_fractions from compiling.
func TestOverflowFloatLiteralLowers(t *testing.T) {
	mod, err := frontend.Parse([]byte("x = 1e400\n"), "ovf.py")
	if err != nil {
		t.Fatal(err)
	}
	src, err := Module(mod, "ovf.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	if !strings.Contains(got, "objects.FloatInf(1)") {
		t.Errorf("overflow literal did not lower to objects.FloatInf(1):\n%s", got)
	}
	if strings.Contains(got, "+Inf") || strings.Contains(got, "NewFloat(Inf") {
		t.Errorf("emitted an invalid Go infinity literal:\n%s", got)
	}
}
