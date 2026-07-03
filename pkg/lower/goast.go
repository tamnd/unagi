package lower

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strconv"
)

// This file is the emitter's go/ast toolbox: small constructors for the node
// shapes the lowering builds over and over, plus the printer that turns one
// finished declaration into text. Everything here composes by nesting nodes;
// only writeDecl produces source, and Module owns the seam that calls it.

// ident is a bare identifier node, the leaf of most expressions.
func ident(name string) *ast.Ident { return ast.NewIdent(name) }

// sel is a qualified name, pkg.Name, for the objects and runtime helpers.
func sel(pkg, name string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: ident(pkg), Sel: ident(name)}
}

// callExpr is fn(args...).
func callExpr(fn ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fn, Args: args}
}

// strLit is a quoted Go string literal; strconv.Quote handles the escaping so
// no caller ever splices raw text into source.
func strLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)}
}

// intLit is a decimal integer literal from already-normalized digits.
func intLit(text string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: text}
}

// floatLit renders a float with the shortest round-tripping form, the same
// formatting the string emitter used.
func floatLit(v float64) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.FLOAT, Value: strconv.FormatFloat(v, 'g', -1, 64)}
}

// notExpr is the logical negation !x.
func notExpr(x ast.Expr) *ast.UnaryExpr {
	return &ast.UnaryExpr{Op: token.NOT, X: x}
}

// errNotNil is the `err != nil` condition every check guards on.
func errNotNil() *ast.BinaryExpr {
	return &ast.BinaryExpr{X: ident("err"), Op: token.NEQ, Y: ident("nil")}
}

// assign is the general lhs... tok rhs... statement; define and set cover the
// two common single-target shapes.
func assign(tok token.Token, lhs []ast.Expr, rhs ...ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Tok: tok, Rhs: rhs}
}

// define is `name := rhs`.
func define(name *ast.Ident, rhs ast.Expr) *ast.AssignStmt {
	return assign(token.DEFINE, []ast.Expr{name}, rhs)
}

// set is `lhs = rhs`.
func set(lhs, rhs ast.Expr) *ast.AssignStmt {
	return assign(token.ASSIGN, []ast.Expr{lhs}, rhs)
}

// exprStmt evaluates an expression for effect.
func exprStmt(x ast.Expr) *ast.ExprStmt { return &ast.ExprStmt{X: x} }

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
func fieldList(fs ...*ast.Field) *ast.FieldList {
	return &ast.FieldList{List: fs}
}

// mainDecl is the fixed entry point: run pymain and report an uncaught
// Python exception through the runtime before exiting nonzero.
func mainDecl() *ast.FuncDecl {
	return &ast.FuncDecl{
		Name: ident("main"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: block(&ast.IfStmt{
			Init: define(ident("err"), callExpr(ident("pymain"))),
			Cond: errNotNil(),
			Body: block(
				exprStmt(callExpr(sel("runtime", "PrintUncaught"), ident("err"))),
				exprStmt(callExpr(sel("os", "Exit"), intLit("1"))),
			),
		}),
	}
}

// writeDecl prints one built declaration followed by a blank line. This is
// the boundary between the node world and the text world: a print failure
// means the emitter built a node the printer rejects, an emitter bug, so it
// surfaces as an error rather than a panic.
func writeDecl(out *bytes.Buffer, d ast.Decl) error {
	if err := format.Node(out, token.NewFileSet(), d); err != nil {
		return fmt.Errorf("emit: generated declaration did not print: %v", err)
	}
	out.WriteString("\n\n")
	return nil
}
