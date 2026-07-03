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
	src, err := Module(mod, "hello.py")
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
	src, err := Module(&frontend.Module{}, "empty.py")
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
	src, err := Module(mod, "add.py")
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"func u_add(u_a objects.Object, u_b objects.Object) (objects.Object, error) {",
		"objects.Add(u_a, u_b)",
		"u_add(objects.NewInt(1), objects.NewInt(2))",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
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
			"attribute read",
			&frontend.Module{Body: []frontend.Stmt{&frontend.ExprStmt{X: &frontend.Attribute{X: str("s"), Name: "upper"}}}},
			"attribute access outside a method call",
		},
		{
			"huge int literal",
			&frontend.Module{Body: []frontend.Stmt{&frontend.ExprStmt{X: call("print", &frontend.IntLit{Text: "99999999999999999999"})}}},
			"int64 range",
		},
		{
			"nested def",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.FuncDef{Name: "outer", Body: []frontend.Stmt{
					&frontend.FuncDef{Name: "inner", Body: []frontend.Stmt{&frontend.Pass{}}},
				}},
			}},
			"nested def",
		},
		{
			"wrong arity",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.FuncDef{Name: "one", Params: params("a"), Body: []frontend.Stmt{&frontend.Pass{}}},
				&frontend.ExprStmt{X: call("one")},
			}},
			"one() takes 1 positional arguments but 0 were given",
		},
		{
			"return inside finally",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.FuncDef{Name: "f", Body: []frontend.Stmt{
					&frontend.Try{
						Body:  []frontend.Stmt{&frontend.Pass{}},
						Final: []frontend.Stmt{&frontend.Return{}},
					},
				}},
			}},
			"'return' inside 'finally' is not supported yet",
		},
		{
			"break inside finally",
			&frontend.Module{Body: []frontend.Stmt{
				&frontend.While{Cond: &frontend.BoolLit{Val: true}, Body: []frontend.Stmt{
					&frontend.Try{
						Body:  []frontend.Stmt{&frontend.Pass{}},
						Final: []frontend.Stmt{&frontend.Break{}},
					},
				}},
			}},
			"'break' inside 'finally' is not supported yet",
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
			_, err := Module(tc.mod, "err.py")
			if err == nil {
				t.Fatal("expected a compile error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}
