package vet

import "github.com/tamnd/unagi/pkg/frontend"

// eachStmt visits every statement in a body, descending into every block
// including nested functions and classes, so a scan sees the whole module.
func eachStmt(stmts []frontend.Stmt, fn func(frontend.Stmt)) {
	for _, s := range stmts {
		fn(s)
		switch n := s.(type) {
		case *frontend.FuncDef:
			eachStmt(n.Body, fn)
		case *frontend.ClassDef:
			eachStmt(n.Body, fn)
		case *frontend.If:
			eachStmt(n.Body, fn)
			eachStmt(n.Else, fn)
		case *frontend.For:
			eachStmt(n.Body, fn)
			eachStmt(n.Else, fn)
		case *frontend.While:
			eachStmt(n.Body, fn)
			eachStmt(n.Else, fn)
		case *frontend.With:
			eachStmt(n.Body, fn)
		case *frontend.Try:
			eachStmt(n.Body, fn)
			for _, h := range n.Handlers {
				eachStmt(h.Body, fn)
			}
			eachStmt(n.OrElse, fn)
			eachStmt(n.Final, fn)
		case *frontend.Match:
			for _, c := range n.Cases {
				eachStmt(c.Body, fn)
			}
		}
	}
}

// stmtExprs returns the expressions a single statement holds directly, not
// recursing into nested statement bodies (eachStmt handles that). Callers wrap
// each result in walkExpr to reach the full subtree.
func stmtExprs(s frontend.Stmt) []frontend.Expr {
	switch n := s.(type) {
	case *frontend.ExprStmt:
		return []frontend.Expr{n.X}
	case *frontend.Assign:
		return append(append([]frontend.Expr{}, n.Targets...), n.Value)
	case *frontend.AugAssign:
		return []frontend.Expr{n.Target, n.Value}
	case *frontend.AnnAssign:
		return []frontend.Expr{n.Target, n.Annotation, n.Value}
	case *frontend.Return:
		return []frontend.Expr{n.Value}
	case *frontend.If:
		return []frontend.Expr{n.Cond}
	case *frontend.While:
		return []frontend.Expr{n.Cond}
	case *frontend.For:
		return []frontend.Expr{n.Target, n.Iter}
	case *frontend.With:
		var out []frontend.Expr
		for _, it := range n.Items {
			out = append(out, it.Context, it.Target)
		}
		return out
	case *frontend.Raise:
		return []frontend.Expr{n.Exc, n.Cause}
	case *frontend.Assert:
		return []frontend.Expr{n.Test, n.Msg}
	case *frontend.Del:
		return n.Targets
	case *frontend.Match:
		return []frontend.Expr{n.Subject}
	}
	return nil
}

// walkExpr visits an expression and every subexpression, calling fn on each. It
// is used to find calls and name reads anywhere in a tree.
func walkExpr(e frontend.Expr, fn func(frontend.Expr)) {
	if e == nil {
		return
	}
	fn(e)
	switch n := e.(type) {
	case *frontend.BinOp:
		walkExpr(n.Left, fn)
		walkExpr(n.Right, fn)
	case *frontend.UnaryOp:
		walkExpr(n.X, fn)
	case *frontend.BoolOp:
		for _, v := range n.Values {
			walkExpr(v, fn)
		}
	case *frontend.Compare:
		walkExpr(n.Left, fn)
		for _, r := range n.Rights {
			walkExpr(r, fn)
		}
	case *frontend.Call:
		walkExpr(n.Fn, fn)
		for _, a := range n.Args {
			walkExpr(a.Value, fn)
		}
	case *frontend.Attribute:
		walkExpr(n.X, fn)
	case *frontend.Subscript:
		walkExpr(n.X, fn)
		walkExpr(n.Index, fn)
	case *frontend.SliceExpr:
		walkExpr(n.Lo, fn)
		walkExpr(n.Hi, fn)
		walkExpr(n.Step, fn)
	case *frontend.IfExp:
		walkExpr(n.Cond, fn)
		walkExpr(n.Then, fn)
		walkExpr(n.Else, fn)
	case *frontend.NamedExpr:
		walkExpr(n.Value, fn)
	case *frontend.Starred:
		walkExpr(n.X, fn)
	case *frontend.Await:
		walkExpr(n.X, fn)
	case *frontend.Lambda:
		walkExpr(n.Body, fn)
	case *frontend.Yield:
		walkExpr(n.Value, fn)
	case *frontend.ListLit:
		for _, x := range n.Elts {
			walkExpr(x, fn)
		}
	case *frontend.TupleLit:
		for _, x := range n.Elts {
			walkExpr(x, fn)
		}
	case *frontend.SetLit:
		for _, x := range n.Elts {
			walkExpr(x, fn)
		}
	case *frontend.DictLit:
		for _, x := range n.Keys {
			walkExpr(x, fn)
		}
		for _, x := range n.Vals {
			walkExpr(x, fn)
		}
	case *frontend.Comp:
		walkExpr(n.Elt, fn)
		walkExpr(n.Val, fn)
		for _, c := range n.Clauses {
			walkExpr(c.Target, fn)
			walkExpr(c.Iter, fn)
			for _, cif := range c.Ifs {
				walkExpr(cif, fn)
			}
		}
	case *frontend.FStr:
		for _, in := range frontend.FInterps(n.Parts) {
			walkExpr(in.X, fn)
		}
	}
}
