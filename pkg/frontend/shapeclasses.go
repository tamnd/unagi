package frontend

import "sort"

// This file finds the module-level classes whose instances have a fixed scalar
// shape the static tier can lower to a flat Go struct (doc 11 tier 2). A class
// qualifies when __slots__ fixes its instance layout, every slot has a scalar
// type the class body annotates, and nothing in the module can leave that layout
// unfaithful. An attribute read on such an instance lowers to a plain Go struct
// field load behind a shape guard the boxed-to-static entry checks; a receiver
// whose runtime class does not match deopts to the boxed twin, so the fixed
// layout only ever describes the exact class named here.
//
// The bar is deliberately high and conservative, the same stance the scalar
// global tracker takes. A class earns a shape only when all of these hold:
//
//   - It is a direct module-body class, not nested in a function or another
//     class, so its definition and every use sit in one scope the analysis sees.
//   - It has no decorator, no keyword (so no metaclass), and no base other than
//     object, so nothing rewrites the class or contributes an unmodeled slot.
//   - Its body assigns __slots__ exactly once, to a tuple or list of plain string
//     literals naming simple identifiers, so the slot set is fixed and known.
//   - Every slot has a bare class-body annotation naming a scalar type (int,
//     float, bool, or str), which fixes the field's representation without
//     running __init__; a slot missing a scalar annotation disqualifies the class.
//   - Its name is bound exactly once in the whole module, by this class
//     definition, so no later assignment, def, import, or second class can swap a
//     differently shaped object in under the same name.
//
// A class that misses any of these keeps its boxed instance layout, exactly as
// before. The exact-class shape guard at the entry means a subclass instance is
// never a false match: it fails the guard and deopts, so subclassing does not
// need to disqualify the base here.

// ShapeClass names a module-level class whose instances the static tier lowers to
// a fixed Go struct, together with its fields in __slots__ order.
type ShapeClass struct {
	Name   string
	Fields []ShapeSlot
}

// ShapeSlot is one slot of a shape: its name and the scalar type its class-body
// annotation fixes.
type ShapeSlot struct {
	Name string
	Type string // "int", "float", "bool", or "str"
}

// ShapeClasses returns the module classes a static attribute read can treat as a
// fixed Go struct, sorted by name so the build is deterministic. A module with no
// qualifying class returns nil, which leaves every instance boxed exactly as it
// was.
func ShapeClasses(m *Module) []ShapeClass {
	// A class name bound more than once in the module cannot be trusted to keep a
	// fixed shape, so count every binding of every name across all scopes first.
	counts := bindingCounts(m.Body)

	var out []ShapeClass
	for _, s := range m.Body {
		cls, ok := s.(*ClassDef)
		if !ok {
			continue
		}
		if counts[cls.Name] != 1 {
			// The name is rebound somewhere, so a differently shaped object could
			// stand in under it. Leave the class boxed.
			continue
		}
		fields, ok := shapeOf(cls)
		if !ok {
			continue
		}
		out = append(out, ShapeClass{Name: cls.Name, Fields: fields})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// shapeOf reports the fixed scalar field layout of a class, or false when the
// class does not qualify. It enforces the no-decorator, no-keyword, object-only
// base rules, reads the one __slots__ assignment for the slot order, and requires
// every slot to carry a scalar class-body annotation.
func shapeOf(cls *ClassDef) ([]ShapeSlot, bool) {
	if len(cls.Decorators) != 0 || len(cls.Keywords) != 0 {
		return nil, false
	}
	if !basesAreObjectOnly(cls.Bases) {
		return nil, false
	}

	slots, ok := slotNames(cls.Body)
	if !ok || len(slots) == 0 {
		return nil, false
	}

	// The bare class-body annotations fix each slot's scalar type.
	anns := scalarAnnotations(cls.Body)
	fields := make([]ShapeSlot, 0, len(slots))
	for _, name := range slots {
		t, ok := anns[name]
		if !ok {
			// A slot without a scalar annotation has no fixed representation, so
			// the whole class stays boxed rather than lowering an unknown field.
			return nil, false
		}
		fields = append(fields, ShapeSlot{Name: name, Type: t})
	}
	return fields, true
}

// basesAreObjectOnly reports whether a class has no base or only the object base.
// Any other base could contribute a slot this analysis does not model, so it
// disqualifies the shape.
func basesAreObjectOnly(bases []Expr) bool {
	for _, b := range bases {
		n, ok := b.(*Name)
		if !ok || n.Id != "object" {
			return false
		}
	}
	return true
}

// slotNames reads the single __slots__ assignment in a class body and returns the
// slot names in order. It reports false when the body has no __slots__, more than
// one, or a __slots__ whose value is not a tuple or list of plain string literals
// naming simple identifiers, so an unusual or computed slot set stays boxed. A
// __dict__ or __weakref__ entry reopens a dynamic layout, so it disqualifies too.
func slotNames(body []Stmt) ([]string, bool) {
	var names []string
	seen := false
	for _, s := range body {
		a, ok := s.(*Assign)
		if !ok {
			continue
		}
		if len(a.Targets) != 1 {
			continue
		}
		n, ok := a.Targets[0].(*Name)
		if !ok || n.Id != "__slots__" {
			continue
		}
		if seen {
			// Two __slots__ assignments leave the layout ambiguous.
			return nil, false
		}
		seen = true
		var elts []Expr
		switch v := a.Value.(type) {
		case *TupleLit:
			elts = v.Elts
		case *ListLit:
			elts = v.Elts
		default:
			return nil, false
		}
		for _, e := range elts {
			lit, ok := e.(*StrLit)
			if !ok || !isIdentifier(lit.Val) {
				return nil, false
			}
			if lit.Val == "__dict__" || lit.Val == "__weakref__" {
				return nil, false
			}
			names = append(names, lit.Val)
		}
	}
	if !seen {
		return nil, false
	}
	return names, true
}

// scalarAnnotations collects the bare class-body annotations that name a scalar
// type, name to type. A bare annotation `x: int` records the field type without
// binding a class attribute (which __slots__ would forbid), so it is how a typed
// slot declares its representation. An annotation with a value, a non-name
// target, or a non-scalar type contributes nothing.
func scalarAnnotations(body []Stmt) map[string]string {
	out := map[string]string{}
	for _, s := range body {
		ann, ok := s.(*AnnAssign)
		if !ok || ann.Value != nil {
			continue
		}
		name, ok := ann.Target.(*Name)
		if !ok {
			continue
		}
		if t, ok := scalarTypeName(ann.Annotation); ok {
			out[name.Id] = t
		}
	}
	return out
}

// scalarTypeName reports the scalar type a bare type-name annotation denotes. It
// recognizes only the four scalar builtins the representation table lowers, so a
// qualified, subscripted, or user type is not a scalar slot here.
func scalarTypeName(e Expr) (string, bool) {
	n, ok := e.(*Name)
	if !ok {
		return "", false
	}
	switch n.Id {
	case "int", "float", "bool", "str":
		return n.Id, true
	}
	return "", false
}

// isIdentifier reports whether a slot string is a plain Python identifier, so a
// name with a space, a dot, or an empty string never becomes a Go field.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if i > 0 {
			ok = ok || (r >= '0' && r <= '9')
		}
		if !ok {
			return false
		}
	}
	return true
}

// bindingCounts walks the whole module, nested scopes included, and counts how
// many times each name is bound by a form that introduces a name: a def, a class,
// an import, or a plain-name assignment target (simple, tuple, and list unpack).
// A class whose name is bound more than once cannot be trusted to keep a fixed
// shape, so the shape analysis requires a count of exactly one.
func bindingCounts(body []Stmt) map[string]int {
	counts := map[string]int{}
	bump := func(name string) { counts[name]++ }

	var bindTarget func(t Expr)
	bindTarget = func(t Expr) {
		switch t := t.(type) {
		case *Name:
			bump(t.Id)
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

	var walk func(list []Stmt)
	walk = func(list []Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *FuncDef:
				bump(s.Name)
				walk(s.Body)
			case *ClassDef:
				bump(s.Name)
				walk(s.Body)
			case *Import:
				for _, a := range s.Names {
					bump(a.Bound())
				}
			case *ImportFrom:
				for _, a := range s.Names {
					bump(a.Bound())
				}
			case *Assign:
				for _, t := range s.Targets {
					bindTarget(t)
				}
			case *AnnAssign:
				if s.Value != nil {
					bindTarget(s.Target)
				}
			case *AugAssign:
				bindTarget(s.Target)
			case *For:
				bindTarget(s.Target)
				walk(s.Body)
				walk(s.Else)
			case *With:
				for _, it := range s.Items {
					if it.Target != nil {
						bindTarget(it.Target)
					}
				}
				walk(s.Body)
			case *If:
				walk(s.Body)
				walk(s.Else)
			case *While:
				walk(s.Body)
				walk(s.Else)
			case *Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					if h.Name != "" {
						bump(h.Name)
					}
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *Match:
				for _, c := range s.Cases {
					for _, name := range PatternNames(c.Pattern) {
						bump(name)
					}
					walk(c.Body)
				}
			}
		}
	}
	walk(body)
	return counts
}
