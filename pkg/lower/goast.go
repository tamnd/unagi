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

// varDecl declares one variable of the given type with no initializer, the
// `var name type` statement.
func varDecl(name string, typ ast.Expr) *ast.DeclStmt {
	return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
		&ast.ValueSpec{Names: []*ast.Ident{ident(name)}, Type: typ},
	}}}
}

// exprStmt evaluates an expression for effect.
func exprStmt(x ast.Expr) *ast.ExprStmt { return &ast.ExprStmt{X: x} }

// strSliceLit builds a []string{...} composite literal.
func strSliceLit(elts []string) ast.Expr {
	lits := make([]ast.Expr, len(elts))
	for i, s := range elts {
		lits[i] = strLit(s)
	}
	return &ast.CompositeLit{Type: &ast.ArrayType{Elt: ident("string")}, Elts: lits}
}

// kv is one key: value entry in a keyed composite literal.
func kv(key string, val ast.Expr) ast.Expr {
	return &ast.KeyValueExpr{Key: ident(key), Value: val}
}

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

// threadParam is the hidden first parameter every emitted Python function
// carries: the runtime.Thread the call spine threads through so per-goroutine
// identity is honest inside any function, the way CPython threads tstate.
func threadParam() *ast.Field {
	return field(&ast.StarExpr{X: sel("runtime", "Thread")}, "t")
}

// threadArg is the ambient thread every emitted body holds, passed on as the
// first argument of a call into another emitted function.
func threadArg() ast.Expr { return ident("t") }

// mainThreadArg is runtime.NewMainThread(), the thread the static tier's boxed
// re-entry points pass on: a static subtree is proven identity-free and carries
// no thread of its own, so its boxed twin re-runs under the main thread.
func mainThreadArg() ast.Expr { return callExpr(sel("runtime", "NewMainThread")) }

// mainDecl is the fixed entry point: run pymain, wait for the non-daemon
// threads it started to finish so their output lands before the process exits,
// then let the runtime turn an uncaught error into a process exit status, a
// traceback for an ordinary exception and a bare code for SystemExit. The wait
// mirrors threading._shutdown and is a no-op for a program that starts no
// thread.
func mainDecl() *ast.FuncDecl {
	return &ast.FuncDecl{
		Name: ident("main"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: block(
			define(ident("err"), callExpr(ident("pymain"), mainThreadArg())),
			exprStmt(callExpr(sel("runtime", "WaitForNonDaemonThreads"))),
			&ast.IfStmt{
				Cond: errNotNil(),
				Body: block(
					exprStmt(callExpr(sel("os", "Exit"),
						callExpr(sel("runtime", "ReportExit"), ident("err")))),
				),
			},
		),
	}
}

// execDecl is a module package's entry point: remember the module object for
// dynamic name loads, bind every module-scope variable as a live slot on it
// so importer and body share storage, and run the body. vars are the Python
// names, already sorted, each backed by its mangled package variable.
func execDecl(vars []string) *ast.FuncDecl {
	body := []ast.Stmt{set(ident("thisModule"), ident("m"))}
	for _, n := range vars {
		body = append(body, exprStmt(callExpr(sel("m", "Bind"),
			strLit(n), &ast.UnaryExpr{Op: token.AND, X: ident(mangle(n))})))
	}
	body = append(body, &ast.ReturnStmt{Results: []ast.Expr{callExpr(ident("pymain"), mainThreadArg())}})
	return &ast.FuncDecl{
		Name: ident("Exec"),
		Type: &ast.FuncType{
			Params:  fieldList(field(&ast.StarExpr{X: sel("objects", "Module")}, "m")),
			Results: fieldList(field(ident("error"))),
		},
		Body: block(body...),
	}
}

// mainGlobalsDecl is the entry point for a script that calls globals(): build
// a __main__ module, remember it in thisModule, and bind every module-scope
// name onto it as a live slot the way a module package's Exec does, so
// globals() reads the same storage the body writes. moduleVars are the checked
// module variables (already sorted); plainDefs are the top-level defs that kept
// the static fast path and so are not module variables, bound through their
// function-object slots. The body then runs exactly as the plain main does.
func (e *emitter) mainGlobalsDecl(moduleVars, plainDefs []string) *ast.FuncDecl {
	addr := func(name string) ast.Expr { return &ast.UnaryExpr{Op: token.AND, X: ident(name)} }
	body := []ast.Stmt{
		define(ident("m"), callExpr(sel("objects", "NewModule"), strLit("__main__"), ident("pyFile"))),
		set(ident("thisModule"), ident("m")),
	}
	for _, n := range moduleVars {
		body = append(body, exprStmt(callExpr(sel("m", "Bind"), strLit(n), addr(mangle(n)))))
	}
	for _, n := range plainDefs {
		body = append(body, exprStmt(callExpr(sel("m", "Bind"), strLit(n), addr(e.fnObjName(n)))))
	}
	body = append(body,
		define(ident("err"), callExpr(ident("pymain"), mainThreadArg())),
		exprStmt(callExpr(sel("runtime", "WaitForNonDaemonThreads"))),
		&ast.IfStmt{
			Cond: errNotNil(),
			Body: block(
				exprStmt(callExpr(sel("os", "Exit"),
					callExpr(sel("runtime", "ReportExit"), ident("err")))),
			),
		})
	return &ast.FuncDecl{
		Name: ident("main"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: block(body...),
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
