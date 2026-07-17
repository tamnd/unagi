package lower

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// getXModule is a one-def module whose body returns its parameter and calls the
// def once, the shape the entry shim routes: def get_x(p): return p; get_x("z").
// The body and the call argument are irrelevant to the shim, which is built from
// the static entry and the def's name and arity.
func getXModule() *frontend.Module {
	return &frontend.Module{Body: []frontend.Stmt{
		&frontend.FuncDef{
			Name:   "get_x",
			Params: params("p"),
			Body:   []frontend.Stmt{&frontend.Return{Value: &frontend.Name{Id: "p"}}},
		},
		&frontend.ExprStmt{X: call("get_x", str("z"))},
	}}
}

// pointEntry is a static entry whose one parameter is a fixed-shape Point with an
// int slot x and a float slot y, returning the int field.
func pointEntry() map[string]StaticEntry {
	return map[string]StaticEntry{
		"get_x": {
			Static: "static_get_x",
			Params: []StaticParam{{Shape: &StaticShape{
				Name: "Point",
				Fields: []StaticShapeField{
					{Name: "x", Scalar: StaticInt},
					{Name: "y", Scalar: StaticFloat},
				},
			}}},
			Ret: StaticInt,
		},
	}
}

// TestShapeShimMaterializesStructFromInstance checks the shape entry shim: it
// guards the receiver's exact class, reads each slot out of the boxed instance,
// unboxes it to its native scalar, assembles the Go struct, enters the static
// form on it, and reboxes the scalar result. Any miss on the class, a slot, or an
// unbox falls back to the boxed form, which is always correct.
func TestShapeShimMaterializesStructFromInstance(t *testing.T) {
	src, err := ModuleStatic(getXModule(), "point.py", nil, nil, pointEntry())
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"func entry0_get_x(p0 objects.Object) (objects.Object, error)",
		`if p0.TypeName() != "Point" {`,
		"return def0_get_x(p0)",
		`s0_0, err := objects.LoadAttr(p0, "x")`,
		"f0_0, sok0_0 := objects.AsInt(s0_0)",
		`s0_1, err := objects.LoadAttr(p0, "y")`,
		"f0_1, sok0_1 := objects.AsFloat(s0_1)",
		"x0 := Point{x: f0_0, y: f0_1}",
		"r, err := static_get_x(x0)",
		"return objects.NewInt(r), nil",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("shape shim missing %q:\n%s", want, got)
		}
	}
}

// TestShapeShimFallsBackOnEverySlotMiss checks that each slot read and each unbox
// guards its own fallback, so a shim reading two slots has one class guard and two
// per-slot fallbacks, all returning the boxed form on the original instance. The
// count keeps the materialization from ever building a partial struct.
func TestShapeShimFallsBackOnEverySlotMiss(t *testing.T) {
	src, err := ModuleStatic(getXModule(), "point.py", nil, nil, pointEntry())
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	if n := strings.Count(got, "return def0_get_x(p0)"); n != 5 {
		t.Errorf("want five boxed fallbacks (one class guard, two slot reads, two unboxes), got %d:\n%s", n, got)
	}
	if strings.Contains(got, "if err != nil {") && !strings.Contains(got, "return def0_get_x(p0)") {
		t.Errorf("slot-read error should fall back to the boxed form:\n%s", got)
	}
}
