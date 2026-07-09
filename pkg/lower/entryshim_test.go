package lower

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// areaModule is a one-def module whose body calls the def with two arguments,
// the shape the entry shim routes: def area(w, h): return w; area(2.0, 3.0).
func areaModule() *frontend.Module {
	return &frontend.Module{Body: []frontend.Stmt{
		&frontend.FuncDef{
			Name:   "area",
			Params: params("w", "h"),
			Body:   []frontend.Stmt{&frontend.Return{Value: &frontend.Name{Id: "w"}}},
		},
		&frontend.ExprStmt{X: call("area",
			&frontend.FloatLit{Val: 2}, &frontend.FloatLit{Val: 3})},
	}}
}

// TestEntryShimGuardsUnboxesReboxes checks the shim the static tier routes boxed
// calls through: it unboxes each argument, guards the exact type, falls back to
// the boxed form on a mismatch, enters the static form, and reboxes the result.
func TestEntryShimGuardsUnboxesReboxes(t *testing.T) {
	statics := map[string]StaticEntry{
		"area": {Static: "static_area", Params: []StaticScalar{StaticFloat, StaticFloat}, Ret: StaticFloat},
	}
	src, err := ModuleStatic(areaModule(), "area.py", nil, nil, statics)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"func entry0_area(p0 objects.Object, p1 objects.Object) (objects.Object, error)",
		"x0, ok0 := objects.AsFloat(p0)",
		"x1, ok1 := objects.AsFloat(p1)",
		`p0.TypeName() != "float"`,
		`p1.TypeName() != "float"`,
		"return def0_area(p0, p1)",
		"r, err := static_area(x0, x1)",
		"return objects.NewFloat(r), nil",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("shim missing %q:\n%s", want, got)
		}
	}
}

// TestEntryShimRoutesBoxedCall checks that the boxed call site enters the shim
// rather than the boxed form directly, so the static tier is reached at runtime.
func TestEntryShimRoutesBoxedCall(t *testing.T) {
	statics := map[string]StaticEntry{
		"area": {Static: "static_area", Params: []StaticScalar{StaticFloat, StaticFloat}, Ret: StaticFloat},
	}
	src, err := ModuleStatic(areaModule(), "area.py", nil, nil, statics)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	if !strings.Contains(got, "entry0_area(") {
		t.Errorf("boxed call site does not route through the entry shim:\n%s", got)
	}
}

// TestNoStaticEntryKeepsBoxedCall checks the default: with no static entry for a
// def, the call site names the boxed form and no shim is emitted, so a build
// without a static tier lowers exactly as before.
func TestNoStaticEntryKeepsBoxedCall(t *testing.T) {
	src, err := ModuleStatic(areaModule(), "area.py", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	if strings.Contains(got, "entry0_area") {
		t.Errorf("no static entry, but an entry shim was emitted:\n%s", got)
	}
	if !strings.Contains(got, "def0_area(") {
		t.Errorf("boxed call site missing without a static entry:\n%s", got)
	}
}
