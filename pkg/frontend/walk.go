package frontend

// WalkCalls invokes fn for every Call expression reachable in body. It descends
// through every statement and expression, including nested function, class,
// comprehension and lambda bodies, f-string and t-string interpolations, and
// the annotations, decorators, defaults and bases a definition carries. The
// walk is read-only and structural: it lets a pass spot a call pattern the
// import graph cannot see, such as a str.encode that resolves a codec by name
// at runtime. fn runs for a call before the walk descends into that call's own
// function expression and arguments, so a call nested inside another call still
// reports. Statement forms that carry no expression (pass, break, global) and
// the interiors of match patterns, which cannot hold a call, are skipped.
func WalkCalls(body []Stmt, fn func(*Call)) {
	w := callWalker{fn: fn}
	w.stmts(body)
}

type callWalker struct {
	fn func(*Call)
}

func (w *callWalker) stmts(list []Stmt) {
	for _, s := range list {
		w.stmt(s)
	}
}

func (w *callWalker) stmt(s Stmt) {
	switch s := s.(type) {
	case *ExprStmt:
		w.expr(s.X)
	case *Assign:
		w.expr(s.Value)
		w.exprs(s.Targets)
	case *AugAssign:
		w.expr(s.Target)
		w.expr(s.Value)
	case *AnnAssign:
		w.expr(s.Target)
		w.expr(s.Annotation)
		w.expr(s.Value)
	case *Del:
		w.exprs(s.Targets)
	case *Return:
		w.expr(s.Value)
	case *Raise:
		w.expr(s.Exc)
		w.expr(s.Cause)
	case *Assert:
		w.expr(s.Test)
		w.expr(s.Msg)
	case *If:
		w.expr(s.Cond)
		w.stmts(s.Body)
		w.stmts(s.Else)
	case *While:
		w.expr(s.Cond)
		w.stmts(s.Body)
		w.stmts(s.Else)
	case *For:
		w.expr(s.Target)
		w.expr(s.Iter)
		w.stmts(s.Body)
		w.stmts(s.Else)
	case *With:
		for _, it := range s.Items {
			w.expr(it.Context)
			w.expr(it.Target)
		}
		w.stmts(s.Body)
	case *Try:
		w.stmts(s.Body)
		for _, h := range s.Handlers {
			w.expr(h.Type)
			w.stmts(h.Body)
		}
		w.stmts(s.OrElse)
		w.stmts(s.Final)
	case *Match:
		w.expr(s.Subject)
		for _, cs := range s.Cases {
			w.expr(cs.Guard)
			w.stmts(cs.Body)
		}
	case *FuncDef:
		for _, d := range s.Decorators {
			w.expr(d)
		}
		for _, pr := range s.Params {
			w.expr(pr.Annotation)
			w.expr(pr.Default)
		}
		w.expr(s.Returns)
		w.stmts(s.Body)
	case *ClassDef:
		for _, d := range s.Decorators {
			w.expr(d)
		}
		w.exprs(s.Bases)
		for _, k := range s.Keywords {
			w.expr(k.Value)
		}
		w.stmts(s.Body)
	case *TypeAlias:
		w.expr(s.Value)
	}
}

func (w *callWalker) exprs(list []Expr) {
	for _, e := range list {
		w.expr(e)
	}
}

func (w *callWalker) expr(e Expr) {
	switch e := e.(type) {
	case *Call:
		w.fn(e)
		w.expr(e.Fn)
		for _, a := range e.Args {
			w.expr(a.Value)
		}
	case *Attribute:
		w.expr(e.X)
	case *Subscript:
		w.expr(e.X)
		w.expr(e.Index)
	case *SliceExpr:
		w.expr(e.Lo)
		w.expr(e.Hi)
		w.expr(e.Step)
	case *BinOp:
		w.expr(e.Left)
		w.expr(e.Right)
	case *UnaryOp:
		w.expr(e.X)
	case *BoolOp:
		w.exprs(e.Values)
	case *Compare:
		w.expr(e.Left)
		w.exprs(e.Rights)
	case *IfExp:
		w.expr(e.Cond)
		w.expr(e.Then)
		w.expr(e.Else)
	case *ListLit:
		w.exprs(e.Elts)
	case *TupleLit:
		w.exprs(e.Elts)
	case *SetLit:
		w.exprs(e.Elts)
	case *DictLit:
		w.exprs(e.Keys)
		w.exprs(e.Vals)
	case *Comp:
		w.expr(e.Elt)
		w.expr(e.Val)
		for _, cl := range e.Clauses {
			w.expr(cl.Target)
			w.expr(cl.Iter)
			w.exprs(cl.Ifs)
		}
	case *Starred:
		w.expr(e.X)
	case *NamedExpr:
		w.expr(e.Value)
	case *Await:
		w.expr(e.X)
	case *Yield:
		w.expr(e.Value)
	case *Lambda:
		for _, pr := range e.Params {
			w.expr(pr.Default)
		}
		w.expr(e.Body)
	case *FStr:
		for _, in := range FInterps(e.Parts) {
			w.expr(in.X)
		}
	case *TStr:
		for _, in := range FInterps(e.Parts) {
			w.expr(in.X)
		}
	}
}
