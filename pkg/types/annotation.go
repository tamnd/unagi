package types

import (
	"strings"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers a type annotation, the expression after a colon or arrow, to
// a lattice element. The rule that governs everything here is D8: the result of
// lowering an annotation is a claim, never a proof (doc 04 section 4.2). It
// becomes a proof only when independent inference agrees with it or a guard
// checks it at a trust boundary. So the caller wraps the lowered type in
// EvAnnotation evidence, and the partitioner treats it as a hint about where to
// spend a guard, not as licence to unbox.
//
// A construct the lattice cannot represent degrades explicitly rather than
// silently: it lowers to Dyn and records a Degradation with the source span, so
// the build report can tell a user their fancy annotation is why a function
// stayed boxed (section 4.2, the paragraph on explicit degradation).

// aliasDepthLimit is the fixed unfolding depth for recursive type aliases from
// doc 04 section 4.2. Past it the alias degrades to Dyn at the cut, which keeps
// lowering total on self-referential aliases.
const aliasDepthLimit = 3

// Scope resolves the names an annotation mentions that are not builtins or
// typing operators: user classes and type aliases. A nil Scope means only
// builtins and typing forms are known, which is the mode the stub loader uses
// for a self-contained stub. Both methods return nil to signal "not a name I
// know", which lowers to a Dyn claim with a recorded degradation.
type Scope interface {
	// ResolveClass returns the interned class element for a name or dotted path,
	// or nil when the name does not denote a class in scope.
	ResolveClass(name string) *Type
	// ResolveAlias returns the type expression a name aliases, or nil when the
	// name is not an alias. Aliases unfold up to aliasDepthLimit.
	ResolveAlias(name string) frontend.Expr
}

// Degradation records where an annotation lost precision and why, so the build
// report can point at the exact expression that soured a claim.
type Degradation struct {
	Span   Span
	Reason string
}

// Lowerer turns annotation expressions into lattice claims. It is stateful only
// in the degradations it accumulates, so one Lowerer serves a whole build and
// the report reads its Degradations at the end. ParseForwardRef, when set,
// reparses a stringified annotation; left nil, a string annotation degrades to
// Dyn.
type Lowerer struct {
	in              *Interner
	scope           Scope
	file            string
	ParseForwardRef func(string) (frontend.Expr, error)
	degradations    []Degradation
}

// NewLowerer builds a lowerer over an interner and an optional scope.
func NewLowerer(in *Interner, scope Scope) *Lowerer {
	return &Lowerer{in: in, scope: scope}
}

// Degradations returns the degradations recorded so far, in the order they were
// hit, which is deterministic under the pass's fixed traversal.
func (l *Lowerer) Degradations() []Degradation { return l.degradations }

// Lower lowers an annotation expression in the named file to a claim: the
// lattice type plus the EvAnnotation evidence that marks it unverified. A nil
// expression is an absent annotation, which is Dyn with no evidence, since the
// absence of a hint is not itself a claim.
func (l *Lowerer) Lower(expr frontend.Expr, file string) (*Type, *Evidence) {
	if expr == nil {
		return l.in.Dyn(), nil
	}
	l.file = file
	t := l.lower(expr, 0)
	return t, Annotated(t, l.spanOf(expr))
}

func (l *Lowerer) spanOf(e frontend.Expr) Span {
	p := e.Span()
	return Span{File: l.file, Line: p.Line, Col: p.Col}
}

func (l *Lowerer) degrade(e frontend.Expr, reason string) *Type {
	l.degradations = append(l.degradations, Degradation{Span: l.spanOf(e), Reason: reason})
	return l.in.Dyn()
}

// lower is the recursive worker. depth counts alias unfoldings so a recursive
// alias cuts to Dyn rather than looping.
func (l *Lowerer) lower(e frontend.Expr, depth int) *Type {
	switch e := e.(type) {
	case *frontend.NoneLit:
		return l.in.None()
	case *frontend.Name:
		return l.lowerName(e, e.Id, depth)
	case *frontend.Attribute:
		return l.lowerName(e, dottedPath(e), depth)
	case *frontend.Subscript:
		return l.lowerSubscript(e, depth)
	case *frontend.BinOp:
		if e.Op == frontend.BinBitOr {
			return l.in.Union(l.lower(e.Left, depth), l.lower(e.Right, depth))
		}
		return l.degrade(e, "operator is not a type expression")
	case *frontend.StrLit:
		return l.lowerForwardRef(e, depth)
	default:
		return l.degrade(e, "unsupported annotation form")
	}
}

// lowerName resolves a bare or dotted name: a builtin scalar or container, a
// typing sentinel, a user class, or a type alias, in that order.
func (l *Lowerer) lowerName(e frontend.Expr, name string, depth int) *Type {
	if t, ok := l.builtinName(name); ok {
		return t
	}
	if l.scope != nil {
		if cls := l.scope.ResolveClass(name); cls != nil {
			return cls
		}
		if alias := l.scope.ResolveAlias(name); alias != nil {
			if depth >= aliasDepthLimit {
				return l.degrade(e, "recursive type alias cut at depth 3")
			}
			return l.lower(alias, depth+1)
		}
	}
	return l.degrade(e, "unknown name in annotation: "+name)
}

// builtinName maps the names that stand for themselves: the scalar types, the
// bare generic containers with unknown element types, and the top sentinels.
func (l *Lowerer) builtinName(name string) (*Type, bool) {
	switch lastSegment(name) {
	case "int":
		return l.in.Int(), true
	case "float":
		return l.in.Float(), true
	case "complex":
		return l.in.Complex(), true
	case "str":
		return l.in.Str(), true
	case "bytes":
		return l.in.Bytes(), true
	case "bool":
		return l.in.Bool(), true
	case "None", "NoneType":
		return l.in.None(), true
	case "Any", "object":
		// object is every value, which for unboxing purposes is no information,
		// so it and Any both lower to the top.
		return l.in.Dyn(), true
	case "list", "List":
		return l.in.List(l.in.Dyn()), true
	case "set", "Set", "frozenset", "FrozenSet":
		return l.in.Set(l.in.Dyn()), true
	case "dict", "Dict":
		return l.in.Dict(l.in.Dyn(), l.in.Dyn()), true
	case "tuple", "Tuple":
		return l.in.TupleVar(l.in.Dyn()), true
	}
	return nil, false
}

// lowerSubscript handles the generic and typing-operator forms X[...].
func (l *Lowerer) lowerSubscript(e *frontend.Subscript, depth int) *Type {
	op := lastSegment(nameOf(e.X))
	args := subscriptArgs(e.Index)
	switch op {
	case "Optional":
		if len(args) != 1 {
			return l.degrade(e, "Optional takes one argument")
		}
		return l.in.Optional(l.lower(args[0], depth))
	case "Union":
		members := make([]*Type, len(args))
		for i, a := range args {
			members[i] = l.lower(a, depth)
		}
		return l.in.Union(members...)
	case "list", "List":
		return l.in.List(l.lowerArg(e, args, 0, depth))
	case "set", "Set", "frozenset", "FrozenSet":
		return l.in.Set(l.lowerArg(e, args, 0, depth))
	case "dict", "Dict":
		return l.in.Dict(l.lowerArg(e, args, 0, depth), l.lowerArg(e, args, 1, depth))
	case "tuple", "Tuple":
		return l.lowerTuple(args, depth)
	case "Callable":
		return l.lowerCallable(e, args, depth)
	case "Literal":
		return l.lowerLiteral(e, args)
	case "Annotated":
		if len(args) == 0 {
			return l.degrade(e, "Annotated needs a base type")
		}
		return l.lower(args[0], depth)
	case "ClassVar", "Final":
		if len(args) != 1 {
			return l.degrade(e, op+" takes one argument")
		}
		return l.lower(args[0], depth)
	case "Type", "type":
		return l.degrade(e, "type[...] metatype is not tracked")
	}
	// An unrecognized subscript over a user class keeps the class and drops the
	// parameters, which is the right claim for a user generic the lattice does
	// not parameterize.
	if l.scope != nil {
		if cls := l.scope.ResolveClass(nameOf(e.X)); cls != nil {
			l.degradations = append(l.degradations,
				Degradation{Span: l.spanOf(e), Reason: "generic parameters dropped on " + nameOf(e.X)})
			return cls
		}
	}
	return l.degrade(e, "unknown generic in annotation: "+op)
}

func (l *Lowerer) lowerArg(e frontend.Expr, args []frontend.Expr, i, depth int) *Type {
	if i >= len(args) {
		l.degradations = append(l.degradations,
			Degradation{Span: l.spanOf(e), Reason: "missing type argument"})
		return l.in.Dyn()
	}
	return l.lower(args[i], depth)
}

// lowerTuple handles tuple[T, ...] (variadic) and tuple[A, B, C] (fixed). An
// empty tuple[()] is the zero-length tuple.
func (l *Lowerer) lowerTuple(args []frontend.Expr, depth int) *Type {
	if len(args) == 2 {
		if _, ok := args[1].(*frontend.EllipsisLit); ok {
			return l.in.TupleVar(l.lower(args[0], depth))
		}
	}
	if len(args) == 1 {
		if _, ok := args[0].(*frontend.TupleLit); ok {
			// tuple[()] spells the empty tuple.
			return l.in.Tuple()
		}
	}
	positions := make([]*Type, len(args))
	for i, a := range args {
		positions[i] = l.lower(a, depth)
	}
	return l.in.Tuple(positions...)
}

// lowerCallable handles Callable[[P...], R] and Callable[..., R]. A ParamSpec or
// Concatenate in the parameter position degrades the parameters to arbitrary,
// matching the section 4.2 rule that higher-order callable tricks lose their
// parameter shape.
func (l *Lowerer) lowerCallable(e *frontend.Subscript, args []frontend.Expr, depth int) *Type {
	if len(args) != 2 {
		return l.degrade(e, "Callable needs a parameter list and a return type")
	}
	sig := &Signature{Return: l.lower(args[1], depth)}
	switch params := args[0].(type) {
	case *frontend.ListLit:
		for _, p := range params.Elts {
			sig.Params = append(sig.Params, Param{Type: l.lower(p, depth), Kind: ParamPosOnly})
		}
	case *frontend.EllipsisLit:
		// Callable[..., R] accepts any arguments, modeled as an open *args of Dyn.
		sig.Star = l.in.Dyn()
	default:
		l.degradations = append(l.degradations,
			Degradation{Span: l.spanOf(e), Reason: "callable parameter shape not representable"})
		sig.Star = l.in.Dyn()
	}
	return l.in.Callable(sig)
}

// lowerLiteral joins the types of the literal alternatives. unagi tracks literal
// values only for match narrowing, so the annotation contributes just the type.
func (l *Lowerer) lowerLiteral(e *frontend.Subscript, args []frontend.Expr) *Type {
	acc := l.in.Never()
	for _, a := range args {
		acc = l.in.Join(acc, l.literalType(a))
	}
	if acc.IsNever() {
		return l.degrade(e, "empty Literal")
	}
	return acc
}

func (l *Lowerer) literalType(a frontend.Expr) *Type {
	switch a.(type) {
	case *frontend.IntLit:
		return l.in.Int()
	case *frontend.StrLit:
		return l.in.Str()
	case *frontend.BytesLit:
		return l.in.Bytes()
	case *frontend.BoolLit:
		return l.in.Bool()
	case *frontend.NoneLit:
		return l.in.None()
	}
	return l.degrade(a, "unsupported Literal alternative")
}

// lowerForwardRef reparses a stringified annotation and lowers the result. With
// no parser wired it degrades, since the string cannot be read as a type here.
func (l *Lowerer) lowerForwardRef(e *frontend.StrLit, depth int) *Type {
	if l.ParseForwardRef == nil {
		return l.degrade(e, "forward reference not resolved")
	}
	inner, err := l.ParseForwardRef(e.Val)
	if err != nil {
		return l.degrade(e, "forward reference did not parse")
	}
	return l.lower(inner, depth)
}

// subscriptArgs flattens a subscript index into its arguments: a TupleLit
// becomes its elements, anything else is a single argument.
func subscriptArgs(index frontend.Expr) []frontend.Expr {
	if t, ok := index.(*frontend.TupleLit); ok {
		return t.Elts
	}
	return []frontend.Expr{index}
}

// nameOf renders the callable side of a subscript as a dotted path, so both
// list[int] and typing.List[int] resolve their operator.
func nameOf(e frontend.Expr) string {
	switch e := e.(type) {
	case *frontend.Name:
		return e.Id
	case *frontend.Attribute:
		return dottedPath(e)
	}
	return ""
}

// dottedPath renders an attribute chain like collections.abc.Iterable back into
// its dotted string.
func dottedPath(e *frontend.Attribute) string {
	var parts []string
	var walk frontend.Expr = e
	for {
		switch n := walk.(type) {
		case *frontend.Attribute:
			parts = append(parts, n.Name)
			walk = n.X
		case *frontend.Name:
			parts = append(parts, n.Id)
			// The chain is built leaf-first, so reverse it into source order.
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			return strings.Join(parts, ".")
		default:
			return strings.Join(parts, ".")
		}
	}
}

// lastSegment returns the final dotted component, so typing.List and List map to
// the same operator.
func lastSegment(path string) string {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[i+1:]
	}
	return path
}
