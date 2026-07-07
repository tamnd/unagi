package frontend

import "strings"

// MangleClassPrivates applies Python's private-name mangling to every class
// body in the module, in place. A name that begins with two underscores and
// does not end with two underscores, written lexically inside a class, is
// rewritten to _ClassName__name with the class name's own leading underscores
// stripped. This is the compile-time transform CPython's _Py_Mangle performs,
// so self.__x, a bare __local, a __helper method, a __count class variable,
// and a global/nonlocal declaration of a private name all resolve to the same
// mangled identifier. The mangling reaches into methods, lambdas, and
// comprehensions nested in the body, because the private context propagates
// into nested function scopes; only a nested class starts a fresh context.
// Keyword-argument names at a call site are never mangled, matching CPython:
// obj.m(__a=1) keeps the keyword __a even when the parameter it targets was
// itself mangled.
func MangleClassPrivates(mod *Module) {
	mangleStmts(mod.Body, "")
}

// mangleEligible reports whether an identifier written in a class body is
// subject to mangling: it must start with two underscores and must not end
// with two underscores, so __x and __x_ mangle but the __dunder__ form, a
// single-underscore _x, and the all-underscore __ do not.
func mangleEligible(name string) bool {
	return strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__")
}

// mangleName rewrites one identifier under the private context priv, the class
// name with its leading underscores already stripped. An empty priv means
// there is no active class context, or the enclosing class was named with
// only underscores, so nothing mangles.
func mangleName(priv, name string) string {
	if priv == "" || !mangleEligible(name) {
		return name
	}
	return "_" + priv + name
}

// classPriv turns a class name into its mangling prefix: the name with leading
// underscores stripped, or "" when the name is all underscores, which disables
// mangling inside that class exactly as CPython does.
func classPriv(name string) string {
	return strings.TrimLeft(name, "_")
}

func mangleStmts(stmts []Stmt, priv string) {
	for _, s := range stmts {
		mangleStmt(s, priv)
	}
}

func mangleStmt(s Stmt, priv string) {
	switch s := s.(type) {
	case *ExprStmt:
		mangleExpr(s.X, priv)
	case *Assign:
		mangleExprs(s.Targets, priv)
		mangleExpr(s.Value, priv)
	case *AugAssign:
		mangleExpr(s.Target, priv)
		mangleExpr(s.Value, priv)
	case *AnnAssign:
		mangleExpr(s.Target, priv)
		mangleExpr(s.Value, priv)
	case *If:
		mangleExpr(s.Cond, priv)
		mangleStmts(s.Body, priv)
		mangleStmts(s.Else, priv)
	case *While:
		mangleExpr(s.Cond, priv)
		mangleStmts(s.Body, priv)
		mangleStmts(s.Else, priv)
	case *For:
		mangleExpr(s.Target, priv)
		mangleExpr(s.Iter, priv)
		mangleStmts(s.Body, priv)
		mangleStmts(s.Else, priv)
	case *With:
		for i := range s.Items {
			mangleExpr(s.Items[i].Context, priv)
			if s.Items[i].Target != nil {
				mangleExpr(s.Items[i].Target, priv)
			}
		}
		mangleStmts(s.Body, priv)
	case *Match:
		mangleExpr(s.Subject, priv)
		for i := range s.Cases {
			manglePattern(s.Cases[i].Pattern, priv)
			if s.Cases[i].Guard != nil {
				mangleExpr(s.Cases[i].Guard, priv)
			}
			mangleStmts(s.Cases[i].Body, priv)
		}
	case *FuncDef:
		// The def name and parameters are mangled in the enclosing class
		// context, and the body keeps that same context: private mangling
		// propagates through nested function scopes.
		s.Name = mangleName(priv, s.Name)
		mangleParams(s.Params, priv)
		mangleExprs(s.Decorators, priv)
		mangleStmts(s.Body, priv)
	case *ClassDef:
		// The class name, bases, and decorators live in the enclosing scope,
		// so they take the current context; the body starts a fresh one keyed
		// on this class's own name.
		s.Name = mangleName(priv, s.Name)
		mangleExprs(s.Bases, priv)
		mangleExprs(s.Decorators, priv)
		mangleStmts(s.Body, classPriv(s.Name))
	case *Try:
		mangleStmts(s.Body, priv)
		for _, h := range s.Handlers {
			if h.Type != nil {
				mangleExpr(h.Type, priv)
			}
			h.Name = mangleName(priv, h.Name)
			mangleStmts(h.Body, priv)
		}
		mangleStmts(s.OrElse, priv)
		mangleStmts(s.Final, priv)
	case *Raise:
		if s.Exc != nil {
			mangleExpr(s.Exc, priv)
		}
		if s.Cause != nil {
			mangleExpr(s.Cause, priv)
		}
	case *Assert:
		mangleExpr(s.Test, priv)
		if s.Msg != nil {
			mangleExpr(s.Msg, priv)
		}
	case *Return:
		if s.Value != nil {
			mangleExpr(s.Value, priv)
		}
	case *Del:
		mangleExprs(s.Targets, priv)
	case *Global:
		mangleNames(s.Names, priv)
	case *Nonlocal:
		mangleNames(s.Names, priv)
	case *Pass, *Break, *Continue:
		// no identifiers
	}
}

func mangleExprs(exprs []Expr, priv string) {
	for _, e := range exprs {
		mangleExpr(e, priv)
	}
}

func mangleExpr(e Expr, priv string) {
	switch e := e.(type) {
	case *Name:
		e.Id = mangleName(priv, e.Id)
	case *ListLit:
		mangleExprs(e.Elts, priv)
	case *TupleLit:
		mangleExprs(e.Elts, priv)
	case *SetLit:
		mangleExprs(e.Elts, priv)
	case *DictLit:
		mangleExprs(e.Keys, priv)
		mangleExprs(e.Vals, priv)
	case *Comp:
		mangleExpr(e.Elt, priv)
		if e.Val != nil {
			mangleExpr(e.Val, priv)
		}
		for i := range e.Clauses {
			mangleExpr(e.Clauses[i].Target, priv)
			mangleExpr(e.Clauses[i].Iter, priv)
			mangleExprs(e.Clauses[i].Ifs, priv)
		}
	case *BinOp:
		mangleExpr(e.Left, priv)
		mangleExpr(e.Right, priv)
	case *UnaryOp:
		mangleExpr(e.X, priv)
	case *BoolOp:
		mangleExprs(e.Values, priv)
	case *Compare:
		mangleExpr(e.Left, priv)
		mangleExprs(e.Rights, priv)
	case *Call:
		mangleExpr(e.Fn, priv)
		for i := range e.Args {
			// The keyword name is never mangled; only the argument value is.
			mangleExpr(e.Args[i].Value, priv)
		}
	case *Attribute:
		mangleExpr(e.X, priv)
		e.Name = mangleName(priv, e.Name)
	case *Subscript:
		mangleExpr(e.X, priv)
		mangleExpr(e.Index, priv)
	case *SliceExpr:
		if e.Lo != nil {
			mangleExpr(e.Lo, priv)
		}
		if e.Hi != nil {
			mangleExpr(e.Hi, priv)
		}
		if e.Step != nil {
			mangleExpr(e.Step, priv)
		}
	case *IfExp:
		mangleExpr(e.Cond, priv)
		mangleExpr(e.Then, priv)
		mangleExpr(e.Else, priv)
	case *NamedExpr:
		e.Target = mangleName(priv, e.Target)
		mangleExpr(e.Value, priv)
	case *Starred:
		mangleExpr(e.X, priv)
	case *Await:
		mangleExpr(e.X, priv)
	case *Yield:
		if e.Value != nil {
			mangleExpr(e.Value, priv)
		}
	case *Lambda:
		mangleParams(e.Params, priv)
		mangleExpr(e.Body, priv)
	case *FStr:
		for _, fi := range FInterps(e.Parts) {
			mangleExpr(fi.X, priv)
		}
	}
}

func manglePattern(p Pattern, priv string) {
	switch p := p.(type) {
	case *PatLiteral:
		mangleExpr(p.Value, priv)
	case *PatValue:
		mangleExpr(p.Value, priv)
	case *PatCapture:
		p.Name = mangleName(priv, p.Name)
	case *PatStar:
		p.Name = mangleName(priv, p.Name)
	case *PatSequence:
		for _, e := range p.Elts {
			manglePattern(e, priv)
		}
	case *PatMapping:
		mangleExprs(p.Keys, priv)
		for _, v := range p.Vals {
			manglePattern(v, priv)
		}
		p.Rest = mangleName(priv, p.Rest)
	case *PatClass:
		mangleExpr(p.Cls, priv)
		for _, sp := range p.Pos {
			manglePattern(sp, priv)
		}
		mangleNames(p.KwNames, priv)
		for _, sp := range p.KwValues {
			manglePattern(sp, priv)
		}
	case *PatOr:
		for _, alt := range p.Alts {
			manglePattern(alt, priv)
		}
	case *PatAs:
		if p.Pattern != nil {
			manglePattern(p.Pattern, priv)
		}
		p.Name = mangleName(priv, p.Name)
	}
}

func mangleParams(params []Param, priv string) {
	for i := range params {
		params[i].Name = mangleName(priv, params[i].Name)
		if params[i].Default != nil {
			mangleExpr(params[i].Default, priv)
		}
	}
}

func mangleNames(names []string, priv string) {
	for i, n := range names {
		names[i] = mangleName(priv, n)
	}
}
