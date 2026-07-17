package frontend

// This file enforces the global- and nonlocal-declaration conflicts
// CPython's symtable raises, all probed on 3.14. A scope walks its
// statements in textual order accumulating three flags per name:
// parameter, used, and assigned. Each global or nonlocal statement then
// checks its names with parameter outranking used outranking assigned
// (probed: an assign followed by a read reports "used prior", del and
// augmented assignment report "assigned to"). Flags keep accumulating
// after a declaration, so declaring the same name again after an
// assignment still errors even though the assignment itself targeted an
// outer scope.
//
// nonlocal adds two checks a global does not: it is illegal at module
// level, and it must resolve to a binding in an enclosing function scope.
// So each scope also carries the bound-name sets of its enclosing function
// scopes, nearest first, and its own bound set to hand down. A class body
// is a scope but not a function scope: it does not join the enclosing
// chain, so a method resolves nonlocal against the class's own enclosing
// functions, not the class.
//
// Lambda bodies and comprehension interiors are their own scopes and
// leave the flags alone; lambda defaults and the outermost comprehension
// iterable evaluate in this scope and count. A walrus anywhere in a
// comprehension binds this scope and counts as assigned.

// checkScopes validates one scope: the module body, one def body with its
// parameters, or a class body. enclosing holds the bound-name sets of the
// enclosing function scopes nearest first, atModule marks the module body,
// and isFunc marks a function scope, the only kind that joins the chain a
// nested nonlocal resolves against. Nested scopes recurse from the FuncDef
// and ClassDef cases.
func (p *parser) checkScopes(body []Stmt, params []Param, enclosing []map[string]bool, atModule, isFunc bool) {
	c := &globalScope{
		p:         p,
		param:     map[string]bool{},
		used:      map[string]bool{},
		assigned:  map[string]bool{},
		enclosing: enclosing,
		atModule:  atModule,
		isFunc:    isFunc,
		selfBound: boundNames(body, params),
	}
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
	// enclosing is the bound-name set of each enclosing function scope,
	// nearest first; selfBound is this scope's own bound set, prepended when
	// descending into a function or class scope. atModule and isFunc classify
	// this scope for the nonlocal rules.
	enclosing []map[string]bool
	selfBound map[string]bool
	atModule  bool
	isFunc    bool
}

// childEnclosing is the enclosing chain a nested scope sees: a function
// scope prepends its own bound set, a class or module scope passes its
// chain through unchanged because it is not a function scope.
func (c *globalScope) childEnclosing() []map[string]bool {
	if !c.isFunc {
		return c.enclosing
	}
	return append([]map[string]bool{c.selfBound}, c.enclosing...)
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
	case *AnnAssign:
		// Only a valued annotation binds its target. A bare `y: int` binds
		// nothing (at module scope the name stays unbound), so it counts as an
		// assignment only when a value is present; a bare attribute or
		// subscript target still reads its base. The annotation itself is
		// deferred (PEP 649) and never contributes a use.
		c.use(s.Value)
		if s.Value != nil {
			c.target(s.Target)
		} else if _, ok := s.Target.(*Name); !ok {
			c.use(s.Target)
		}
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
	case *Match:
		c.use(s.Subject)
		for _, cs := range s.Cases {
			c.usePattern(cs.Pattern)
			for _, name := range PatternNames(cs.Pattern) {
				c.assigned[name] = true
			}
			c.use(cs.Guard)
			c.stmts(cs.Body)
		}
	case *FuncDef:
		// Decorators and defaults evaluate in this scope; the body is its own
		// function scope and gets its own checker seeded with the parameters.
		for _, d := range s.Decorators {
			c.use(d)
		}
		for _, pr := range s.Params {
			c.use(pr.Default)
		}
		c.assigned[s.Name] = true
		c.p.checkScopes(s.Body, s.Params, c.childEnclosing(), false, true)
	case *ClassDef:
		// Decorators and bases evaluate in this scope; the class name binds
		// here. The body is a scope but not a function scope, so it does not
		// join the chain a nested nonlocal resolves against.
		for _, d := range s.Decorators {
			c.use(d)
		}
		for _, b := range s.Bases {
			c.use(b)
		}
		c.assigned[s.Name] = true
		c.p.checkScopes(s.Body, nil, c.childEnclosing(), false, false)
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
	case *Nonlocal:
		if c.atModule {
			c.p.errf(s.Pos_, "nonlocal declaration not allowed at module level")
		}
		for _, n := range s.Names {
			switch {
			case c.param[n]:
				c.p.errf(s.Pos_, "name '%s' is parameter and nonlocal", n)
			case c.used[n]:
				c.p.errf(s.Pos_, "name '%s' is used prior to nonlocal declaration", n)
			case c.assigned[n]:
				c.p.errf(s.Pos_, "name '%s' is assigned to before nonlocal declaration", n)
			default:
				bound := false
				for _, b := range c.enclosing {
					if b[n] {
						bound = true
						break
					}
				}
				if !bound {
					c.p.errf(s.Pos_, "no binding for nonlocal '%s' found", n)
				}
			}
		}
	}
}

// LocalBindings returns the names a function binds as its own locals: its
// parameters and every name it assigns, iterates, captures with a walrus, or
// defines, minus the names it declares global or nonlocal. A name in this set is
// a local of the function, so a read of it resolves to that local, never to a
// module global of the same name; a name absent from it that the function reads
// is a free name, which may be a module global. The static tier uses this to
// decide whether a free name in a function body can be a tracked-global read: a
// name the function binds locally is off limits, since reading it before its
// assignment is an UnboundLocalError rather than a global load.
func LocalBindings(fn *FuncDef) map[string]bool {
	return boundNames(fn.Body, fn.Params)
}

// boundNames gathers every name a scope binds directly: parameters, every
// assignment, augmented-assignment, for, with, and except-as target, del
// targets, walrus targets, and nested def and class names. It descends
// compound statements but not into a nested def, lambda, or class body,
// which are deeper scopes. Names the scope declares global or nonlocal are
// removed at the end, since the scope does not own them, so a nested
// nonlocal cannot resolve to them here. This is the set an inner nonlocal
// checks the enclosing chain against.
func boundNames(body []Stmt, params []Param) map[string]bool {
	out := map[string]bool{}
	for _, pr := range params {
		out[pr.Name] = true
	}
	notLocal := map[string]bool{}

	var bindTarget func(t Expr)
	bindTarget = func(t Expr) {
		switch t := t.(type) {
		case *Name:
			out[t.Id] = true
		case *Starred:
			bindTarget(t.X)
		case *TupleLit:
			for _, el := range t.Elts {
				bindTarget(el)
			}
		case *ListLit:
			for _, el := range t.Elts {
				bindTarget(el)
			}
		}
	}

	// walrus targets bind the scope no matter how deep the expression, so
	// every read position is scanned for them, including comprehension
	// interiors where a walrus leaks to the enclosing scope.
	var walrus func(e Expr)
	walrusAll := func(list []Expr) {
		for _, x := range list {
			walrus(x)
		}
	}
	walrus = func(e Expr) {
		switch e := e.(type) {
		case *NamedExpr:
			out[e.Target] = true
			walrus(e.Value)
		case *ListLit:
			walrusAll(e.Elts)
		case *TupleLit:
			walrusAll(e.Elts)
		case *SetLit:
			walrusAll(e.Elts)
		case *DictLit:
			walrusAll(e.Keys)
			walrusAll(e.Vals)
		case *BinOp:
			walrus(e.Left)
			walrus(e.Right)
		case *UnaryOp:
			walrus(e.X)
		case *BoolOp:
			walrusAll(e.Values)
		case *Compare:
			walrus(e.Left)
			walrusAll(e.Rights)
		case *Call:
			walrus(e.Fn)
			for _, a := range e.Args {
				walrus(a.Value)
			}
		case *Attribute:
			walrus(e.X)
		case *Subscript:
			walrus(e.X)
			walrus(e.Index)
		case *SliceExpr:
			walrus(e.Lo)
			walrus(e.Hi)
			walrus(e.Step)
		case *IfExp:
			walrus(e.Cond)
			walrus(e.Then)
			walrus(e.Else)
		case *Starred:
			walrus(e.X)
		case *Await:
			walrus(e.X)
		case *Yield:
			walrus(e.Value)
		case *Comp:
			walrus(e.Elt)
			walrus(e.Val)
			for _, cl := range e.Clauses {
				walrus(cl.Iter)
				walrusAll(cl.Ifs)
			}
		case *Lambda:
			// A lambda default evaluates here, so a walrus in it binds here.
			for _, pr := range e.Params {
				walrus(pr.Default)
			}
		case *FStr:
			for _, in := range FInterps(e.Parts) {
				walrus(in.X)
			}
		}
	}

	var walk func(list []Stmt)
	walk = func(list []Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *ExprStmt:
				walrus(s.X)
			case *Assign:
				walrus(s.Value)
				for _, t := range s.Targets {
					bindTarget(t)
					walrus(t)
				}
			case *AugAssign:
				bindTarget(s.Target)
				walrus(s.Value)
			case *AnnAssign:
				walrus(s.Value)
				walrus(s.Target)
				if s.Value != nil {
					bindTarget(s.Target)
				}
			case *Del:
				for _, t := range s.Targets {
					bindTarget(t)
				}
			case *Return:
				walrus(s.Value)
			case *Raise:
				walrus(s.Exc)
				walrus(s.Cause)
			case *Assert:
				walrus(s.Test)
				walrus(s.Msg)
			case *If:
				walrus(s.Cond)
				walk(s.Body)
				walk(s.Else)
			case *While:
				walrus(s.Cond)
				walk(s.Body)
				walk(s.Else)
			case *For:
				bindTarget(s.Target)
				walrus(s.Iter)
				walk(s.Body)
				walk(s.Else)
			case *With:
				for _, it := range s.Items {
					walrus(it.Context)
					if it.Target != nil {
						bindTarget(it.Target)
					}
				}
				walk(s.Body)
			case *Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walrus(h.Type)
					if h.Name != "" {
						out[h.Name] = true
					}
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *FuncDef:
				out[s.Name] = true
				walrusAll(s.Decorators)
				for _, pr := range s.Params {
					walrus(pr.Default)
				}
			case *Match:
				walrus(s.Subject)
				for _, cs := range s.Cases {
					for _, name := range PatternNames(cs.Pattern) {
						out[name] = true
					}
					walrus(cs.Guard)
					walk(cs.Body)
				}
			case *ClassDef:
				out[s.Name] = true
				walrusAll(s.Decorators)
				walrusAll(s.Bases)
			case *Global:
				for _, n := range s.Names {
					notLocal[n] = true
				}
			case *Nonlocal:
				for _, n := range s.Names {
					notLocal[n] = true
				}
			}
		}
	}
	walk(body)

	for n := range notLocal {
		delete(out, n)
	}
	return out
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
	case *ListLit:
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
	case *Await:
		c.use(e.X)
	case *Yield:
		c.use(e.Value)
	case *FStr:
		for _, in := range FInterps(e.Parts) {
			c.use(in.X)
		}
	}
}

func (c *globalScope) useAll(list []Expr) {
	for _, x := range list {
		c.use(x)
	}
}

// usePattern flags the names a pattern reads: a value lookup's dotted name, a
// class pattern's class, and mapping keys. Capture names bind rather than read
// and are handled by the caller through PatternNames.
func (c *globalScope) usePattern(pat Pattern) {
	switch pat := pat.(type) {
	case *PatValue:
		c.use(pat.Value)
	case *PatLiteral:
		c.use(pat.Value)
	case *PatSequence:
		for _, e := range pat.Elts {
			c.usePattern(e)
		}
	case *PatMapping:
		c.useAll(pat.Keys)
		for _, v := range pat.Vals {
			c.usePattern(v)
		}
	case *PatClass:
		c.use(pat.Cls)
		for _, sp := range pat.Pos {
			c.usePattern(sp)
		}
		for _, sp := range pat.KwValues {
			c.usePattern(sp)
		}
	case *PatOr:
		for _, a := range pat.Alts {
			c.usePattern(a)
		}
	case *PatAs:
		if pat.Pattern != nil {
			c.usePattern(pat.Pattern)
		}
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
	case *Await:
		c.bindOnly(e.X)
	case *Yield:
		c.bindOnly(e.Value)
	case *FStr:
		for _, in := range FInterps(e.Parts) {
			c.bindOnly(in.X)
		}
	}
}

func (c *globalScope) bindAll(list []Expr) {
	for _, x := range list {
		c.bindOnly(x)
	}
}
