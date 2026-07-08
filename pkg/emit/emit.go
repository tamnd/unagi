// Package emit holds the typed lowering paths, the half of the compiler that
// turns a proven static unit into ordinary Go working on unboxed values. Where
// pkg/lower emits boxed Go that manipulates *objects.Object through pkg/objects,
// emit produces the static tier of doc 06 section 2.1: a Python int becomes a Go
// int64 with an overflow guard, a Python float becomes a float64 with no wrapper,
// and arithmetic on them is native Go, not a slot-table dispatch.
//
// This slice lands the scalar core: the representation mapping of doc 04, the
// int and float arithmetic lowering, and the function shape that carries unboxed
// parameters and returns (T, error) per D14. It has no dependency on the IR pass,
// which does not exist yet; it lowers a small typed expression and statement
// model that a later IR pass will build, exactly as the partitioner slices lower
// abstract census and cost inputs. Everything composes by nesting go/ast nodes;
// only Print produces text.
package emit

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strconv"
	"strings"
)

// runtimePkg is the import alias the emitted static tier reaches its runtime
// helpers through, the rt of the doc 06 worked examples. Deopt handlers and
// semantic-error constructors are named against it.
const runtimePkg = "rt"

// ident is a bare identifier node.
func ident(name string) *ast.Ident { return ast.NewIdent(name) }

// sel is a qualified name, pkg.Name.
func sel(pkg, name string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: ident(pkg), Sel: ident(name)}
}

// callExpr is fn(args...).
func callExpr(fn ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fn, Args: args}
}

// intLit is a decimal integer literal.
func intLit(v int64) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(v, 10)}
}

// floatLit renders a float with the shortest round-tripping form, forcing a
// decimal point when the shortest form has none so the literal keeps its float
// type in Go: a bare 0 would type the target int and miscompile the arithmetic.
func floatLit(v float64) *ast.BasicLit {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eEnN") {
		s += ".0"
	}
	return &ast.BasicLit{Kind: token.FLOAT, Value: s}
}

// binary is x op y.
func binary(op token.Token, x, y ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{X: x, Op: op, Y: y}
}

// define is `name := rhs`.
func define(name string, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{ident(name)}, Tok: token.DEFINE, Rhs: []ast.Expr{rhs}}
}

// setStmt is `lhs = rhs`.
func setStmt(lhs, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{lhs}, Tok: token.ASSIGN, Rhs: []ast.Expr{rhs}}
}

// addAssign is `name += rhs`, the total form float accumulation lowers to.
func addAssign(name string, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{ident(name)}, Tok: token.ADD_ASSIGN, Rhs: []ast.Expr{rhs}}
}

// ifStmt is `if cond { body }` with no else.
func ifStmt(cond ast.Expr, body ...ast.Stmt) *ast.IfStmt {
	return &ast.IfStmt{Cond: cond, Body: block(body...)}
}

// ret is `return results...`.
func ret(results ...ast.Expr) *ast.ReturnStmt { return &ast.ReturnStmt{Results: results} }

// block wraps statements in a braces block.
func block(list ...ast.Stmt) *ast.BlockStmt { return &ast.BlockStmt{List: list} }

// field is one parameter or result; names is empty for an unnamed result.
func field(typ ast.Expr, names ...string) *ast.Field {
	f := &ast.Field{Type: typ}
	for _, n := range names {
		f.Names = append(f.Names, ident(n))
	}
	return f
}

// fieldList groups fields for a parameter or result list.
func fieldList(fs ...*ast.Field) *ast.FieldList { return &ast.FieldList{List: fs} }

// Print renders one built declaration to gofmt-clean source. A print failure
// means the emitter built a node the printer rejects, an emitter bug, so it is an
// error rather than silent bad output.
func Print(d ast.Decl) (string, error) {
	var out bytes.Buffer
	if err := format.Node(&out, token.NewFileSet(), d); err != nil {
		return "", fmt.Errorf("emit: generated declaration did not print: %v", err)
	}
	return out.String(), nil
}
