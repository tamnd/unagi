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
		"area": {Static: "static_area", Params: []StaticParam{ScalarParam(StaticFloat), ScalarParam(StaticFloat)}, Ret: StaticFloat},
	}
	src, err := ModuleStatic(areaModule(), "area.py", nil, nil, statics)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"func entry0_area(t *runtime.Thread, p0 objects.Object, p1 objects.Object) (objects.Object, error)",
		"x0, ok0 := objects.AsFloat(p0)",
		"x1, ok1 := objects.AsFloat(p1)",
		`p0.TypeName() != "float"`,
		`p1.TypeName() != "float"`,
		"return def0_area(t, p0, p1)",
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
		"area": {Static: "static_area", Params: []StaticParam{ScalarParam(StaticFloat), ScalarParam(StaticFloat)}, Ret: StaticFloat},
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

// TestDeoptEntryEmitsHandlerAndUnwrapsSentinel checks the deopt-target shim: the
// build marks the static entry Deopt, so the shim emits the hand-off that reboxes
// the entry parameters into the boxed twin and returns the sentinel, and the shim
// itself unwraps that sentinel into the boxed result rather than surfacing it as a
// raised error. A real exception still propagates unchanged.
func TestDeoptEntryEmitsHandlerAndUnwrapsSentinel(t *testing.T) {
	statics := map[string]StaticEntry{
		"area": {Static: "static_area", Params: []StaticParam{ScalarParam(StaticFloat), ScalarParam(StaticFloat)}, Ret: StaticFloat, Deopt: true},
	}
	src, err := ModuleStatic(areaModule(), "area.py", nil, nil, statics)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"func static_area_deopt(p0 float64, p1 float64) (float64, error)",
		"r, err := def0_area(runtime.NewMainThread(), objects.NewFloat(p0), objects.NewFloat(p1))",
		"return 0, &objects.Deopt{Value: r}",
		"if d, ok := err.(*objects.Deopt); ok",
		"return d.Value, nil",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("deopt shim missing %q:\n%s", want, got)
		}
	}
}

// TestNoDeoptEntrySkipsHandler checks that a guard-free static entry emits no
// hand-off and no sentinel unwrap, so the deopt machinery is inert unless the
// unit can actually deopt.
func TestNoDeoptEntrySkipsHandler(t *testing.T) {
	statics := map[string]StaticEntry{
		"area": {Static: "static_area", Params: []StaticParam{ScalarParam(StaticFloat), ScalarParam(StaticFloat)}, Ret: StaticFloat},
	}
	src, err := ModuleStatic(areaModule(), "area.py", nil, nil, statics)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, unwant := range []string{"static_area_deopt", "objects.Deopt"} {
		if strings.Contains(got, unwant) {
			t.Errorf("guard-free entry should not emit %q:\n%s", unwant, got)
		}
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
