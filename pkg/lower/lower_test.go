package lower

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

func call(name string, args ...frontend.Expr) *frontend.Call {
	wrapped := make([]frontend.Arg, len(args))
	for i, a := range args {
		wrapped[i] = frontend.Arg{Value: a}
	}
	return &frontend.Call{Fn: &frontend.Name{Id: name}, Args: wrapped}
}

func params(names ...string) []frontend.Param {
	ps := make([]frontend.Param, len(names))
	for i, n := range names {
		ps[i] = frontend.Param{Name: n, Kind: frontend.ParamPlain}
	}
	return ps
}

func str(s string) *frontend.StrLit { return &frontend.StrLit{Val: s} }

func TestHelloLowering(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.ExprStmt{X: call("print", str("hi"))},
	}}
	src, err := Module(mod, "hello.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"package main",
		"runtime.Print(objects.NewStr(\"hi\"))",
		"func pymain() error {",
		"runtime.PrintUncaught(err)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

func TestEmptyModuleSkipsObjectsImport(t *testing.T) {
	src, err := Module(&frontend.Module{}, "empty.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "pkg/objects") {
		t.Errorf("empty module should not import pkg/objects:\n%s", src)
	}
}

func TestFunctionLowering(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.FuncDef{Name: "add", Params: params("a", "b"), Body: []frontend.Stmt{
			&frontend.Return{Value: &frontend.BinOp{
				Left: &frontend.Name{Id: "a"}, Op: frontend.BinAdd, Right: &frontend.Name{Id: "b"},
			}},
		}},
		&frontend.ExprStmt{X: call("print", call("add", &frontend.IntLit{Text: "1"}, &frontend.IntLit{Text: "2"}))},
	}}
	src, err := Module(mod, "add.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"func def0_add(u_a objects.Object, u_b objects.Object) (objects.Object, error) {",
		"objects.Add(u_a, u_b)",
		"def0_add(objects.NewInt(1), objects.NewInt(2))",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

// Binding failures that CPython raises at call time lower to inline raises,
// so the TypeError stays catchable rather than failing the compile.
func TestWrongArityLowersToRaise(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.FuncDef{Name: "one", Params: params("a"), Body: []frontend.Stmt{&frontend.Pass{}}},
		&frontend.ExprStmt{X: call("one")},
	}}
	src, err := Module(mod, "arity.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `objects.Raise(objects.TypeError, "one() missing 1 required positional argument: 'a'")`
	if !strings.Contains(string(src), want) {
		t.Errorf("emitted source missing %q:\n%s", want, src)
	}
}

// A decorated def builds its function object then applies the decorator
// through objects.Call, and binds the name to the result through the module
// variable rather than the static function fast path.
func TestDecoratedDefLowering(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.FuncDef{Name: "deco", Params: params("f"), Body: []frontend.Stmt{
			&frontend.Return{Value: &frontend.Name{Id: "f"}},
		}},
		&frontend.FuncDef{Name: "target", Body: []frontend.Stmt{&frontend.Pass{}},
			Decorators: []frontend.Expr{&frontend.Name{Id: "deco"}}},
	}}
	src, err := Module(mod, "dec.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"objects.Call(",
		"u_target =",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

// A method parameter default evaluates once at class-definition time and
// rides on the function object, so the NewFunction call carries the aligned
// defaults slice with a nil for self and the temp for the defaulted param.
func TestMethodDefaultLowering(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.ClassDef{Name: "C", Body: []frontend.Stmt{
			&frontend.FuncDef{Name: "m", Params: []frontend.Param{
				{Name: "self", Kind: frontend.ParamPlain},
				{Name: "x", Kind: frontend.ParamPlain, Default: &frontend.IntLit{Text: "1"}},
			}, Body: []frontend.Stmt{
				&frontend.Return{Value: &frontend.Name{Id: "x"}},
			}},
		}},
	}}
	src, err := Module(mod, "md.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"t1 := objects.NewInt(1)",
		`objects.NewFunction("C.m"`,
		"[]objects.Object{nil, t1}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

// A keyword argument on a method call routes through CallMethodKw with the
// name and value carried as parallel slices, so the runtime binder resolves
// it against the method's signature.
func TestMethodKeywordCallLowering(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.Assign{Targets: []frontend.Expr{&frontend.Name{Id: "obj"}}, Value: &frontend.IntLit{Text: "0"}},
		&frontend.ExprStmt{X: &frontend.Call{
			Fn: &frontend.Attribute{X: &frontend.Name{Id: "obj"}, Name: "m"},
			Args: []frontend.Arg{
				{Name: "x", Value: &frontend.IntLit{Text: "1"}},
			},
		}},
	}}
	src, err := Module(mod, "mk.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		`objects.CallMethodKw(`,
		`"m"`,
		`[]string{"x"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

// A literal past int64 lowers to a NewIntText call so the emitted program
// parses it into a big int at startup.
func TestHugeIntLiteralLowersToText(t *testing.T) {
	mod := &frontend.Module{Body: []frontend.Stmt{
		&frontend.ExprStmt{X: call("print", &frontend.IntLit{Text: "99999999999999999999"})},
	}}
	src, err := Module(mod, "big.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `objects.NewIntText("99999999999999999999")`
	if !strings.Contains(string(src), want) {
		t.Errorf("emitted source missing %q:\n%s", want, src)
	}
}

// An unshadowed builtin read as a value resolves to its function object
// through runtime.BuiltinFn, so it can be passed around and called later.
func TestBuiltinReadAsValueLowering(t *testing.T) {
	got, err := lowerSrc(t, "f = len\nprint(f([1]))\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	want := `runtime.BuiltinFn("len")`
	if !strings.Contains(got, want) {
		t.Errorf("emitted source missing %q:\n%s", want, got)
	}
}

// A builtin shadowed by a local binding reads the local slot, never the
// builtin fallback.
func TestShadowedBuiltinReadsLocal(t *testing.T) {
	got, err := lowerSrc(t, "len = 5\nprint(len)\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if strings.Contains(got, `runtime.BuiltinFn("len")`) {
		t.Errorf("shadowed builtin should not fall back to BuiltinFn:\n%s", got)
	}
}

// A return inside a finally block lowers without error, parks its value, and
// emits the PEP 765 SyntaxWarning replay at the top of pymain.
func TestFinallyReturnLowersAndWarns(t *testing.T) {
	got, err := lowerSrc(t, "def f():\n    try:\n        return 1\n    finally:\n        return 2\nprint(f())\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	for _, want := range []string{
		`runtime.SyntaxWarn("gen.py:5: SyntaxWarning: 'return' in a 'finally' block\n`,
		"pend = 1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

// A break inside a finally targeting the enclosing loop parks and warns; a
// finally with no exiting jump keeps the plain single-branch shape.
func TestFinallyBreakWarns(t *testing.T) {
	got, err := lowerSrc(t, "for i in range(2):\n    try:\n        pass\n    finally:\n        break\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	want := `runtime.SyntaxWarn("gen.py:5: SyntaxWarning: 'break' in a 'finally' block\n`
	if !strings.Contains(got, want) {
		t.Errorf("emitted source missing %q:\n%s", want, got)
	}
}

func TestCompileErrors(t *testing.T) {
	cases := []struct {
		name string
		mod  *frontend.Module
		want string
	}{
		{
			"return outside function",
			&frontend.Module{Body: []frontend.Stmt{&frontend.Return{}}},
			"'return' outside function",
		},
		{
			"break outside loop",
			&frontend.Module{Body: []frontend.Stmt{&frontend.Break{}}},
			"'break' outside loop",
		},
		{
			"undefined name",
			&frontend.Module{Body: []frontend.Stmt{&frontend.ExprStmt{X: call("print", &frontend.Name{Id: "ghost"})}}},
			`name "ghost" is not defined`,
		},
		{
			"conditional module-level def",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.If{Cond: &frontend.BoolLit{Val: true}, Body: []frontend.Stmt{
					&frontend.FuncDef{Name: "inner", Body: []frontend.Stmt{&frontend.Pass{}}},
				}},
			}},
			"conditional module-level def",
		},
		{
			"except matcher is a variable",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.Assign{Targets: []frontend.Expr{&frontend.Name{Id: "x"}}, Value: &frontend.IntLit{Text: "1"}},
				&frontend.Try{
					Body: []frontend.Stmt{&frontend.Pass{}},
					Handlers: []*frontend.ExceptHandler{{
						Type: &frontend.Name{Id: "x"},
						Body: []frontend.Stmt{&frontend.Pass{}},
					}},
				},
			}},
			"except matcher must be a builtin exception class name",
		},
		{
			"except matcher unknown name",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.Try{
					Body: []frontend.Stmt{&frontend.Pass{}},
					Handlers: []*frontend.ExceptHandler{{
						Type: &frontend.Name{Id: "NoSuchError"},
						Body: []frontend.Stmt{&frontend.Pass{}},
					}},
				},
			}},
			`name "NoSuchError" is not defined`,
		},
		{
			"except matcher not a name",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.Try{
					Body: []frontend.Stmt{&frontend.Pass{}},
					Handlers: []*frontend.ExceptHandler{{
						Type: &frontend.IntLit{Text: "1"},
						Body: []frontend.Stmt{&frontend.Pass{}},
					}},
				},
			}},
			"except matcher must be an exception class name or a tuple of them",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Module(tc.mod, "err.py", nil)
			if err == nil {
				t.Fatal("expected a compile error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}
