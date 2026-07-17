package lower

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// collapseSpaces rewrites every run of spaces to a single space, so an assertion
// on a declaration reads the same whether or not gofmt padded the type column to
// align it with a longer sibling name.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// factorModule is a module that binds a scalar global FACTOR at module scope, so
// the module body store instruments the shadow, and reads it inside a def, the
// shape a static reader takes. The def is never called, so lowering it is enough
// to exercise the store instrumentation without needing a runnable program.
func factorModule() *frontend.Module {
	return &frontend.Module{Body: []frontend.Stmt{
		&frontend.Assign{
			Targets: []frontend.Expr{&frontend.Name{Id: "FACTOR"}},
			Value:   &frontend.IntLit{Text: "3"},
		},
		&frontend.FuncDef{
			Name:   "boost",
			Params: params("x"),
			Body:   []frontend.Stmt{&frontend.Return{Value: &frontend.Name{Id: "FACTOR"}}},
		},
	}}
}

// TestTrackedGlobalDeclaresShadowAndBumpsOnModuleStore checks the slice-4
// mechanism: a tracked global gets a typed shadow and a version counter at
// package level, and the module-scope store to it is followed by a Rebind that
// refreshes the pair. The shadow type follows the tracked scalar type, and the
// counter is int64 so the entry guard's compare is well typed.
func TestTrackedGlobalDeclaresShadowAndBumpsOnModuleStore(t *testing.T) {
	tracked := map[string]string{"FACTOR": "int"}
	src, err := ModuleStaticGlobals(factorModule(), "factor.py", nil, nil, nil, tracked)
	if err != nil {
		t.Fatal(err)
	}
	got := collapseSpaces(string(src))
	for _, want := range []string{
		"bshadow_FACTOR int64",
		"bver_FACTOR int64",
		"bshadow_FACTOR, bver_FACTOR = runtime.RebindInt(u_FACTOR)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tracked int global missing %q:\n%s", want, string(src))
		}
	}
}

// TestUntrackedModuleStoreEmitsNoShadow checks that a nil tracked map leaves the
// module untouched: the store to FACTOR emits no Rebind and no shadow declares,
// so a build that never lit the static tier over globals is byte-for-byte what it
// was before this slice.
func TestUntrackedModuleStoreEmitsNoShadow(t *testing.T) {
	src, err := ModuleStaticGlobals(factorModule(), "factor.py", nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, absent := range []string{"bshadow_FACTOR", "bver_FACTOR", "RebindInt"} {
		if strings.Contains(got, absent) {
			t.Errorf("untracked module unexpectedly emitted %q:\n%s", absent, got)
		}
	}
}

// TestFunctionLocalStoreDoesNotBumpShadow checks the module-global-write gate: a
// function assigns a name that collides with a tracked global but never declares
// it global, so the assignment binds a fresh local and must not touch the shadow.
// Only the module-scope store to the real global bumps the version, so exactly one
// Rebind is emitted.
func TestFunctionLocalStoreDoesNotBumpShadow(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.Assign{
			Targets: []frontend.Expr{&frontend.Name{Id: "FACTOR"}},
			Value:   &frontend.IntLit{Text: "3"},
		},
		&frontend.FuncDef{
			Name:   "shadowing",
			Params: params(),
			Body: []frontend.Stmt{
				// A bare assignment with no `global FACTOR` binds a local, a distinct
				// binding from the module global of the same name.
				&frontend.Assign{
					Targets: []frontend.Expr{&frontend.Name{Id: "FACTOR"}},
					Value:   &frontend.IntLit{Text: "9"},
				},
				&frontend.Return{Value: &frontend.Name{Id: "FACTOR"}},
			},
		},
	}}
	tracked := map[string]string{"FACTOR": "int"}
	src, err := ModuleStaticGlobals(mod, "factor.py", nil, nil, nil, tracked)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), "runtime.RebindInt(u_FACTOR)"); n != 1 {
		t.Errorf("RebindInt bump count = %d, want 1 (only the module store, not the function local):\n%s", n, string(src))
	}
}

// TestGlobalDeclaredWriteInFunctionBumpsShadow checks the other half of the gate:
// a function that declares `global FACTOR` and assigns it does rebind the module
// global, so its store is instrumented too. Both the module store and the
// function store bump the version, so two Rebinds are emitted.
func TestGlobalDeclaredWriteInFunctionBumpsShadow(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.Assign{
			Targets: []frontend.Expr{&frontend.Name{Id: "FACTOR"}},
			Value:   &frontend.IntLit{Text: "3"},
		},
		&frontend.FuncDef{
			Name:   "retune",
			Params: params(),
			Body: []frontend.Stmt{
				&frontend.Global{Names: []string{"FACTOR"}},
				&frontend.Assign{
					Targets: []frontend.Expr{&frontend.Name{Id: "FACTOR"}},
					Value:   &frontend.IntLit{Text: "4"},
				},
			},
		},
	}}
	tracked := map[string]string{"FACTOR": "int"}
	src, err := ModuleStaticGlobals(mod, "factor.py", nil, nil, nil, tracked)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), "runtime.RebindInt(u_FACTOR)"); n != 2 {
		t.Errorf("RebindInt bump count = %d, want 2 (module store and the global-declared function store):\n%s", n, string(src))
	}
}

// TestTrackedGlobalScalarTypesPickTheRightRebind checks that each scalar type
// selects its own shadow Go type and Rebind helper, so a float global's shadow is
// float64 refreshed by RebindFloat and never coerced through the int path.
func TestTrackedGlobalScalarTypesPickTheRightRebind(t *testing.T) {
	cases := []struct {
		scalar, goType, rebind string
		lit                    frontend.Expr
	}{
		{"float", "float64", "RebindFloat", &frontend.FloatLit{Val: 1.5}},
		{"str", "string", "RebindStr", &frontend.StrLit{Val: "hi"}},
		{"bool", "bool", "RebindBool", &frontend.BoolLit{Val: true}},
	}
	for _, c := range cases {
		t.Run(c.scalar, func(t *testing.T) {
			mod := &frontend.Module{Body: []frontend.Stmt{
				&frontend.Assign{
					Targets: []frontend.Expr{&frontend.Name{Id: "K"}},
					Value:   c.lit,
				},
			}}
			src, err := ModuleStaticGlobals(mod, "k.py", nil, nil, nil, map[string]string{"K": c.scalar})
			if err != nil {
				t.Fatal(err)
			}
			got := collapseSpaces(string(src))
			for _, want := range []string{
				"bshadow_K " + c.goType,
				"bshadow_K, bver_K = runtime." + c.rebind + "(u_K)",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("%s global missing %q:\n%s", c.scalar, want, string(src))
				}
			}
		})
	}
}
