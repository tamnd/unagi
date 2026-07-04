package frontend

// This file enforces the global-declaration conflicts CPython's symtable
// raises, all probed on 3.14. A scope walks its statements in textual
// order accumulating three flags per name: parameter, used, and assigned.
// Each global statement then checks its names with parameter outranking
// used outranking assigned (probed: an assign followed by a read reports
// "used prior", del and augmented assignment report "assigned to"). Flags
// keep accumulating after a global statement, so declaring the same name
// again after an assignment still errors even though the assignment
// itself targeted module scope.
//
// Lambda bodies and comprehension interiors are their own scopes and
// leave the flags alone; lambda defaults and the outermost comprehension
// iterable evaluate in this scope and count. A walrus anywhere in a
// comprehension binds this scope and counts as assigned.

// checkGlobals validates one scope: the module body, or one def body with
// its parameters. Def bodies recurse from the FuncDef case.
func (p *parser) checkGlobals(body []Stmt, params []Param) {
	c := &globalScope{p: p, param: map[string]bool{}, used: map[string]bool{}, assigned: map[string]bool{}}
	for _, pr := range params {
		c.param[pr.Name] = true
	}
	c.stmts(body)
}

type globalScope struct {
	p        *parser
	param    map[string]bool
	used     map[string]bool
	assigned map[string]bool
}

func (c *globalScope) stmts(list []Stmt) {
	for _, s := range list {
		c.stmt(s)
	}
}

func (c *globalScope) stmt(s Stmt) {
	switch s := s.(type) {
	case *ExprStmt:
		c.use(s.X)
	case *Assign:
		c.use(s.Value)
		for _, t := range s.Targets {
			c.target(t)
		}
	case *AugAssign:
		// Probed: the augmented target counts as assigned, not used.
		c.target(s.Target)
		c.use(s.Value)
	case *Del:
		// Probed: a del target reports "assigned to", like CPython's
		// symtable, which files del under binding operations.
		for _, t := range s.Targets {
			c.target(t)
		}
	case *Return:
		c.use(s.Value)
	case *Raise:
		c.use(s.Exc)
		c.use(s.Cause)
	case *Assert:
		c.use(s.Test)
		c.use(s.Msg)
	case *If:
		c.use(s.Cond)
		c.stmts(s.Body)
		c.stmts(s.Else)
	case *While:
		c.use(s.Cond)
		c.stmts(s.Body)
		c.stmts(s.Else)
	case *For:
		c.use(s.Iter)
		c.target(s.Target)
		c.stmts(s.Body)
		c.stmts(s.Else)
	case *Try:
		c.stmts(s.Body)
		for _, h := range s.Handlers {
			c.use(h.Type)
			if h.Name != "" {
				c.assigned[h.Name] = true
			}
			c.stmts(h.Body)
		}
		c.stmts(s.OrElse)
		c.stmts(s.Final)
	case *FuncDef:
		// Defaults evaluate in this scope; the body is its own scope and
		// gets its own checker seeded with the parameters.
		for _, pr := range s.Params {
			c.use(pr.Default)
		}
		c.assigned[s.Name] = true
		c.p.checkGlobals(s.Body, s.Params)
	case *Global:
		for _, n := range s.Names {
			switch {
			case c.param[n]:
				c.p.errf(s.Pos_, "name '%s' is parameter and global", n)
			case c.used[n]:
				c.p.errf(s.Pos_, "name '%s' is used prior to global declaration", n)
			case c.assigned[n]:
				c.p.errf(s.Pos_, "name '%s' is assigned to before global declaration", n)
			}
		}
	}
}

// target flags a binding position. Names bind; subscript and slice targets
// only use their parts.
func (c *globalScope) target(t Expr) {
	switch t := t.(type) {
	case *Name:
		c.assigned[t.Id] = true
	case *Starred:
		c.target(t.X)
	case *TupleLit:
		for _, el := range t.Elts {
			c.target(el)
		}
	default:
		c.use(t)
	}
}

// use flags every name read in this scope. A nil child (optional slice
// part, bare return) matches no case and is skipped.
func (c *globalScope) use(e Expr) {
	switch e := e.(type) {
	case *Name:
		c.used[e.Id] = true
	case *NamedExpr:
		c.assigned[e.Target] = true
		c.use(e.Value)
	case *Lambda:
		// The body is the lambda's scope; only defaults evaluate here.
		for _, pr := range e.Params {
			c.use(pr.Default)
		}
	case *Comp:
		// Probed: only the outermost iterable counts as evaluated here;
		// the element, conditions, and inner iterables belong to the
		// comprehension for this rule even though PEP 709 inlines them.
		// A walrus anywhere inside still binds this scope.
		c.use(e.Clauses[0].Iter)
		c.bindOnly(e.Elt)
		c.bindOnly(e.Val)
		for i, cl := range e.Clauses {
			if i > 0 {
				c.bindOnly(cl.Iter)
			}
			for _, cond := range cl.Ifs {
				c.bindOnly(cond)
			}
		}
	case *ListLit:
		c.useAll(e.Elts)
	case *TupleLit:
		c.useAll(e.Elts)
	case *SetLit:
		c.useAll(e.Elts)
	case *DictLit:
		c.useAll(e.Keys)
		c.useAll(e.Vals)
	case *BinOp:
		c.use(e.Left)
		c.use(e.Right)
	case *UnaryOp:
		c.use(e.X)
	case *BoolOp:
		c.useAll(e.Values)
	case *Compare:
		c.use(e.Left)
		c.useAll(e.Rights)
	case *Call:
		c.use(e.Fn)
		for _, a := range e.Args {
			c.use(a.Value)
		}
	case *Attribute:
		c.use(e.X)
	case *Subscript:
		c.use(e.X)
		c.use(e.Index)
	case *SliceExpr:
		c.use(e.Lo)
		c.use(e.Hi)
		c.use(e.Step)
	case *IfExp:
		c.use(e.Cond)
		c.use(e.Then)
		c.use(e.Else)
	case *Starred:
		c.use(e.X)
	case *FStr:
		for _, part := range e.Parts {
			if in, ok := part.(*FInterp); ok {
				c.use(in.X)
			}
		}
	}
}

func (c *globalScope) useAll(list []Expr) {
	for _, x := range list {
		c.use(x)
	}
}

// bindOnly walks comprehension interiors: walrus targets bind this scope,
// nothing reads in it. Lambda bodies stay excluded here too.
func (c *globalScope) bindOnly(e Expr) {
	switch e := e.(type) {
	case *NamedExpr:
		c.assigned[e.Target] = true
		c.bindOnly(e.Value)
	case *Lambda:
		for _, pr := range e.Params {
			c.bindOnly(pr.Default)
		}
	case *Comp:
		c.bindOnly(e.Elt)
		c.bindOnly(e.Val)
		for _, cl := range e.Clauses {
			c.bindOnly(cl.Iter)
			for _, cond := range cl.Ifs {
				c.bindOnly(cond)
			}
		}
	case *ListLit:
		c.bindAll(e.Elts)
	case *TupleLit:
		c.bindAll(e.Elts)
	case *SetLit:
		c.bindAll(e.Elts)
	case *DictLit:
		c.bindAll(e.Keys)
		c.bindAll(e.Vals)
	case *BinOp:
		c.bindOnly(e.Left)
		c.bindOnly(e.Right)
	case *UnaryOp:
		c.bindOnly(e.X)
	case *BoolOp:
		c.bindAll(e.Values)
	case *Compare:
		c.bindOnly(e.Left)
		c.bindAll(e.Rights)
	case *Call:
		c.bindOnly(e.Fn)
		for _, a := range e.Args {
			c.bindOnly(a.Value)
		}
	case *Attribute:
		c.bindOnly(e.X)
	case *Subscript:
		c.bindOnly(e.X)
		c.bindOnly(e.Index)
	case *SliceExpr:
		c.bindOnly(e.Lo)
		c.bindOnly(e.Hi)
		c.bindOnly(e.Step)
	case *IfExp:
		c.bindOnly(e.Cond)
		c.bindOnly(e.Then)
		c.bindOnly(e.Else)
	case *Starred:
		c.bindOnly(e.X)
	case *FStr:
		for _, part := range e.Parts {
			if in, ok := part.(*FInterp); ok {
				c.bindOnly(in.X)
			}
		}
	}
}

func (c *globalScope) bindAll(list []Expr) {
	for _, x := range list {
		c.bindOnly(x)
	}
}
