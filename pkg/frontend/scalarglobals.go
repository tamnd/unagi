package frontend

import "sort"

// This file finds the module-level scalar globals the static tier can read
// through a typed shadow guarded by a world-age version (doc 11 tier 3). The
// static tier works on native scalars, so a global it reads must have a native
// type fixed ahead of time and a way to notice a rebinding that no longer fits
// that type. A qualifying global gets a package-level shadow of its scalar type
// and a version counter the boxed tier keeps in step on every assignment; a
// static reader guards the counter at entry and reads the shadow only while the
// live binding still matches, deopting to the boxed twin otherwise.
//
// A name qualifies when a module-scope assignment binds it to a scalar literal,
// which fixes the shadow type, every such literal agrees on that type, and the
// name is never bound by a form the shadow-update path does not instrument: a
// def, a class, an import, a walrus, or a nonlocal capture. Plain assignment,
// augmented assignment, tuple and list unpack, for and with targets, and del all
// route through the boxed store the instrumentation hooks, so they are allowed
// and a same-type rebind keeps the fast path while an incompatible one bumps the
// counter off the specialized version and deopts.
//
// The scope handling is deliberately conservative: a def, class, walrus, or
// nonlocal that binds a matching name anywhere in the module, even as an
// unrelated function local, disqualifies the global. That costs a little
// coverage on name reuse but never tracks a global whose shadow the boxed tier
// could leave stale, which the byte-identity invariant does not allow.

// ScalarGlobal names a module-level global the static tier tracks through a typed
// shadow, together with the scalar type its module-scope binding fixes.
type ScalarGlobal struct {
	Name string
	Type string // "int", "float", "bool", or "str"
}

// ScalarGlobals returns the module globals a static function may read through a
// world-age-guarded typed shadow, sorted by name so the build is deterministic. A
// module with no qualifying global returns nil, which leaves the boxed tier
// exactly as it was.
func ScalarGlobals(m *Module) []ScalarGlobal {
	typ := map[string]string{}
	disq := map[string]bool{}
	markDisq := func(name string) {
		disq[name] = true
		delete(typ, name)
	}
	establish := func(name, t string) {
		if disq[name] {
			return
		}
		if prev, ok := typ[name]; ok && prev != t {
			markDisq(name)
			return
		}
		typ[name] = t
	}

	// Module-scope assignments fix the shadow type. Only a direct-body assignment
	// of a scalar literal to a plain name establishes a type; a conflicting literal
	// type on the same name disqualifies it, since the shadow can hold only one Go
	// type. A non-literal or non-name assignment neither establishes nor
	// disqualifies: it is an instrumented rebind whose type the runtime bump tracks.
	for _, s := range m.Body {
		switch s := s.(type) {
		case *Assign:
			t, ok := scalarLitType(s.Value)
			if ok && allPlainNames(s.Targets) {
				for _, tgt := range s.Targets {
					establish(tgt.(*Name).Id, t)
				}
			}
		case *AnnAssign:
			if name, ok := s.Target.(*Name); ok && s.Value != nil {
				if t, ok := scalarLitType(s.Value); ok {
					establish(name.Id, t)
				}
			}
		}
	}

	// A def, class, import, walrus, or nonlocal capture binds a name the shadow
	// update does not instrument, so any global sharing that name is disqualified
	// wherever the binding appears, module scope or a nested function.
	uninstrumented(m.Body, markDisq)

	var out []ScalarGlobal
	for name, t := range typ {
		if disq[name] {
			continue
		}
		out = append(out, ScalarGlobal{Name: name, Type: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// scalarLitType reports the scalar type of a bare scalar-literal expression, so a
// module-scope assignment of a literal fixes a global's shadow type. A negated or
// signed numeric literal keeps its numeric type, and a bitwise-inverted int stays
// an int, so `-3`, `+2`, and `~1` still establish. Anything else has no
// compile-time scalar type here.
func scalarLitType(e Expr) (string, bool) {
	switch e := e.(type) {
	case *IntLit:
		return "int", true
	case *FloatLit:
		return "float", true
	case *BoolLit:
		return "bool", true
	case *StrLit:
		return "str", true
	case *UnaryOp:
		switch e.Op {
		case UnaryNeg, UnaryPos:
			if t, ok := scalarLitType(e.X); ok && (t == "int" || t == "float") {
				return t, true
			}
		case UnaryInvert:
			if t, ok := scalarLitType(e.X); ok && t == "int" {
				return "int", true
			}
		}
	}
	return "", false
}

// allPlainNames reports whether every assignment target is a bare name, the only
// shape whose literal RHS establishes a shadow type. A tuple, subscript, or
// attribute target binds something other than the literal itself, so it does not.
func allPlainNames(targets []Expr) bool {
	for _, t := range targets {
		if _, ok := t.(*Name); !ok {
			return false
		}
	}
	return len(targets) > 0
}

// uninstrumented walks the whole module, nested functions included, and reports
// every name bound by a def, class, import, walrus, or nonlocal, the forms whose
// binding the shadow-update path does not hook. A global sharing one of these
// names cannot keep a faithful shadow, so it is disqualified.
func uninstrumented(body []Stmt, mark func(string)) {
	var walkExpr func(e Expr)
	walkExpr = func(e Expr) {
		switch e := e.(type) {
		case nil:
			return
		case *NamedExpr:
			mark(e.Target)
			walkExpr(e.Value)
		case *ListLit:
			for _, x := range e.Elts {
				walkExpr(x)
			}
		case *TupleLit:
			for _, x := range e.Elts {
				walkExpr(x)
			}
		case *SetLit:
			for _, x := range e.Elts {
				walkExpr(x)
			}
		case *DictLit:
			for _, x := range e.Keys {
				walkExpr(x)
			}
			for _, x := range e.Vals {
				walkExpr(x)
			}
		case *BinOp:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *UnaryOp:
			walkExpr(e.X)
		case *BoolOp:
			for _, x := range e.Values {
				walkExpr(x)
			}
		case *Compare:
			walkExpr(e.Left)
			for _, x := range e.Rights {
				walkExpr(x)
			}
		case *Call:
			walkExpr(e.Fn)
			for _, a := range e.Args {
				walkExpr(a.Value)
			}
		case *Attribute:
			walkExpr(e.X)
		case *Subscript:
			walkExpr(e.X)
			walkExpr(e.Index)
		case *SliceExpr:
			walkExpr(e.Lo)
			walkExpr(e.Hi)
			walkExpr(e.Step)
		case *IfExp:
			walkExpr(e.Cond)
			walkExpr(e.Then)
			walkExpr(e.Else)
		case *Starred:
			walkExpr(e.X)
		case *Await:
			walkExpr(e.X)
		case *Yield:
			walkExpr(e.Value)
		case *Comp:
			walkExpr(e.Elt)
			walkExpr(e.Val)
			for _, cl := range e.Clauses {
				walkExpr(cl.Iter)
				for _, f := range cl.Ifs {
					walkExpr(f)
				}
			}
		case *Lambda:
			for _, p := range e.Params {
				walkExpr(p.Default)
			}
			walkExpr(e.Body)
		case *FStr:
			for _, in := range FInterps(e.Parts) {
				walkExpr(in.X)
			}
		case *TStr:
			for _, in := range FInterps(e.Parts) {
				walkExpr(in.X)
			}
		}
	}

	var walk func(list []Stmt)
	walk = func(list []Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *FuncDef:
				mark(s.Name)
				for _, p := range s.Params {
					walkExpr(p.Default)
				}
				walk(s.Body)
			case *ClassDef:
				mark(s.Name)
				walk(s.Body)
			case *Import:
				for _, a := range s.Names {
					mark(a.Bound())
				}
			case *ImportFrom:
				for _, a := range s.Names {
					mark(a.Bound())
				}
			case *Nonlocal:
				for _, n := range s.Names {
					mark(n)
				}
			case *ExprStmt:
				walkExpr(s.X)
			case *Assign:
				walkExpr(s.Value)
				for _, t := range s.Targets {
					walkExpr(t)
				}
			case *AugAssign:
				walkExpr(s.Value)
			case *AnnAssign:
				walkExpr(s.Value)
			case *Return:
				walkExpr(s.Value)
			case *Raise:
				walkExpr(s.Exc)
				walkExpr(s.Cause)
			case *Assert:
				walkExpr(s.Test)
				walkExpr(s.Msg)
			case *If:
				walkExpr(s.Cond)
				walk(s.Body)
				walk(s.Else)
			case *While:
				walkExpr(s.Cond)
				walk(s.Body)
				walk(s.Else)
			case *For:
				walkExpr(s.Iter)
				walk(s.Body)
				walk(s.Else)
			case *With:
				for _, it := range s.Items {
					walkExpr(it.Context)
				}
				walk(s.Body)
			case *Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walkExpr(h.Type)
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *Match:
				walkExpr(s.Subject)
				for _, c := range s.Cases {
					walkExpr(c.Guard)
					walk(c.Body)
				}
			}
		}
	}
	walk(body)
}
