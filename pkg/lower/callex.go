package lower

import (
	"go/ast"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/objects"
)

// Call sites that unpack * or ** arguments merge their parts at runtime.
// CPython evaluates the positional group first and the keyword group
// second, each in source order, with the merge checks firing in argument
// position; the lowering here emits statements in exactly that order.

func hasUnpack(args []frontend.Arg) bool {
	for _, a := range args {
		if a.Star != 0 {
			return true
		}
	}
	return false
}

func hasKwParts(args []frontend.Arg) bool {
	for _, a := range args {
		if a.Name != "" || a.Star == 2 {
			return true
		}
	}
	return false
}

// callEx lowers a call that unpacks. Methods, builtins, and exception
// classes keep dedicated routes; everything else calls through a function
// value and the runtime binder.
func (f *fnCtx) callEx(e *frontend.Call) (ast.Expr, error) {
	if attr, ok := e.Fn.(*frontend.Attribute); ok {
		return f.methodCallEx(attr, e)
	}
	if name, ok := e.Fn.(*frontend.Name); ok && !f.nameIsBound(name.Id) {
		if _, isDef := f.e.defs[name.Id]; !isDef {
			if builtinNames[name.Id] {
				if hasKwParts(e.Args) {
					return nil, f.e.errf(e.Span(), "keyword arguments combined with argument unpacking are not supported yet on builtin calls")
				}
				ct := f.tmpVar()
				f.add(define(ident(ct), callExpr(sel("runtime", "BuiltinFn"), strLit(name.Id))))
				return f.callExValue(ident(ct), e)
			}
			if objects.IsExceptionClass(name.Id) {
				return f.excClassStarNew(name.Id, e)
			}
		}
	}
	// The Name lowering owns function objects, rebound-name checks, and the
	// not-defined rejection; whatever it produces is the callee value.
	fv, err := f.expr(e.Fn)
	if err != nil {
		return nil, err
	}
	ct := f.tmpVar()
	f.add(define(ident(ct), fv))
	return f.callExValue(ident(ct), e)
}

func (f *fnCtx) nameIsBound(id string) bool {
	if f.locals[id] {
		return true
	}
	for o := f.outer; o != nil; o = o.outer {
		if o.locals[id] {
			return true
		}
	}
	return false
}

// callExValue finishes an unpacking call once the callee sits in a temp:
// positional group, keyword group, then the CallEx entry that matches the
// shape.
func (f *fnCtx) callExValue(ct ast.Expr, e *frontend.Call) (ast.Expr, error) {
	pos, star, err := f.unpackPos(e.Args)
	if err != nil {
		return nil, err
	}
	kw, hasKw, err := f.unpackKw(ct, e.Args)
	if err != nil {
		return nil, err
	}
	tmp := f.tmpVar()
	switch {
	case star != nil:
		f.fallible(tmp, f.e.obj("CallStarExT"), threadArg(), ct, star, kw)
	case hasKw:
		f.fallible(tmp, f.e.obj("CallExT"), threadArg(), ct, pos, kw)
	default:
		f.fallible(tmp, f.e.obj("CallT"), threadArg(), ct, pos)
	}
	return ident(tmp), nil
}

// unpackPos evaluates the positional parts in source order. A lone
// *iterable stays unconverted, spilled to a temp for CallStarEx to convert
// at call time with the callee-naming wording; any other mix builds the
// slice in place, and a star merges through ExtendStar the moment its
// value exists, before anything to its right evaluates.
func (f *fnCtx) unpackPos(args []frontend.Arg) (pos, star ast.Expr, err error) {
	var parts []frontend.Arg
	for _, a := range args {
		if a.Name == "" && a.Star != 2 {
			parts = append(parts, a)
		}
	}
	if len(parts) == 1 && parts[0].Star == 1 {
		v, err := f.expr(parts[0].Value)
		if err != nil {
			return nil, nil, err
		}
		t := f.tmpVar()
		f.add(define(ident(t), v))
		return nil, ident(t), nil
	}
	var lead []ast.Expr
	i := 0
	for ; i < len(parts) && parts[i].Star == 0; i++ {
		v, err := f.expr(parts[i].Value)
		if err != nil {
			return nil, nil, err
		}
		lead = append(lead, v)
	}
	cur := f.tmpVar()
	f.add(define(ident(cur), f.objSlice(lead)))
	for ; i < len(parts); i++ {
		v, err := f.expr(parts[i].Value)
		if err != nil {
			return nil, nil, err
		}
		next := f.tmpVar()
		if parts[i].Star == 1 {
			f.fallible(next, f.e.obj("ExtendStar"), ident(cur), v)
		} else {
			f.add(define(ident(next), callExpr(ident("append"), ident(cur), v)))
		}
		cur = next
	}
	return ident(cur), nil, nil
}

// unpackKw evaluates the keyword parts in source order into an accumulated
// keyword dict. KwSet and KwMerge check duplicates and mapping-ness in
// argument position; key stringness waits for the call itself.
func (f *fnCtx) unpackKw(ct ast.Expr, args []frontend.Arg) (kw ast.Expr, hasKw bool, err error) {
	cur := ast.Expr(ident("nil"))
	for _, a := range args {
		if a.Name == "" && a.Star != 2 {
			continue
		}
		hasKw = true
		v, err := f.expr(a.Value)
		if err != nil {
			return nil, false, err
		}
		next := f.tmpVar()
		if a.Star == 2 {
			f.fallible(next, f.e.obj("KwMerge"), ct, cur, v)
		} else {
			f.fallible(next, f.e.obj("KwSet"), ct, cur, strLit(a.Name), v)
		}
		cur = ident(next)
	}
	return cur, hasKw, nil
}

// methodCallEx lowers obj.method(*parts, name=value, **mapping). The receiver
// evaluates first and, when keywords are present, spills to a temp the merge
// helpers reference to spell their errors as receiver.method(); the positional
// and keyword groups then merge in source order the way callExValue does.
func (f *fnCtx) methodCallEx(attr *frontend.Attribute, e *frontend.Call) (ast.Expr, error) {
	recv, err := f.expr(attr.X)
	if err != nil {
		return nil, err
	}
	if !hasKwParts(e.Args) {
		pos, star, err := f.unpackPos(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		if star != nil {
			f.fallible(tmp, f.e.obj("CallMethodStarT"), threadArg(), recv, strLit(attr.Name), star)
		} else {
			f.fallible(tmp, f.e.obj("CallMethodT"), threadArg(), recv, strLit(attr.Name), pos)
		}
		return ident(tmp), nil
	}
	rt := f.tmpVar()
	f.add(define(ident(rt), recv))
	pos, star, err := f.unpackPos(e.Args)
	if err != nil {
		return nil, err
	}
	kw, err := f.unpackKwM(ident(rt), attr.Name, e.Args)
	if err != nil {
		return nil, err
	}
	tmp := f.tmpVar()
	if star != nil {
		f.fallible(tmp, f.e.obj("CallMethodStarExT"), threadArg(), ident(rt), strLit(attr.Name), star, kw)
	} else {
		f.fallible(tmp, f.e.obj("CallMethodExT"), threadArg(), ident(rt), strLit(attr.Name), pos, kw)
	}
	return ident(tmp), nil
}

// unpackKwM is unpackKw for a method call: it merges the keyword parts through
// KwSetM and KwMergeM, which spell their duplicate and mapping errors against
// the receiver-qualified method name instead of a callee value.
func (f *fnCtx) unpackKwM(recv ast.Expr, name string, args []frontend.Arg) (ast.Expr, error) {
	cur := ast.Expr(ident("nil"))
	for _, a := range args {
		if a.Name == "" && a.Star != 2 {
			continue
		}
		v, err := f.expr(a.Value)
		if err != nil {
			return nil, err
		}
		next := f.tmpVar()
		if a.Star == 2 {
			f.fallible(next, f.e.obj("KwMergeM"), recv, strLit(name), cur, v)
		} else {
			f.fallible(next, f.e.obj("KwSetM"), recv, strLit(name), cur, strLit(a.Name), v)
		}
		cur = ident(next)
	}
	return cur, nil
}

// excClassStarNew lowers ClassName(*parts) and ClassName(..., kw=v, **m). The
// callee spelling is static, so the positional conversion and the keyword merge
// both carry it as a literal. A builtin exception type takes no keywords, so any
// surviving keyword raises the catchable takes-no-keyword TypeError, but only
// after the argument assembly the way CPython orders it: keyword merge checks,
// then the lone-star conversion, then the key-stringness check and the keyword
// rejection.
func (f *fnCtx) excClassStarNew(c string, e *frontend.Call) (ast.Expr, error) {
	pos, star, err := f.unpackPos(e.Args)
	if err != nil {
		return nil, err
	}
	var kw ast.Expr
	if hasKwParts(e.Args) {
		kw, err = f.unpackKwFor(c+"()", e.Args)
		if err != nil {
			return nil, err
		}
	}
	argsExpr := pos
	if star != nil {
		t := f.tmpVar()
		f.fallible(t, f.e.obj("StarArgsFor"), strLit(c+"()"), star)
		argsExpr = ident(t)
	}
	if kw != nil {
		v := f.tmpVar()
		f.fallible(v, f.e.obj("ExcNoKeywords"), strLit(c), argsExpr, kw)
		argsExpr = ident(v)
	}
	if objects.IsExcGroupClass(c) {
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "NewExcGroup"), strLit(c), argsExpr)
		return ident(tmp), nil
	}
	return callExpr(sel("runtime", "NewExc"), strLit(c), argsExpr), nil
}

// unpackKwFor is unpackKw for a callee whose spelling is known at compile time,
// like an exception class. It folds the keyword parts through KwSetFor and
// KwMergeFor, which carry the pre-rendered funcstr instead of a callee value.
func (f *fnCtx) unpackKwFor(funcstr string, args []frontend.Arg) (ast.Expr, error) {
	cur := ast.Expr(ident("nil"))
	for _, a := range args {
		if a.Name == "" && a.Star != 2 {
			continue
		}
		v, err := f.expr(a.Value)
		if err != nil {
			return nil, err
		}
		next := f.tmpVar()
		if a.Star == 2 {
			f.fallible(next, f.e.obj("KwMergeFor"), strLit(funcstr), cur, v)
		} else {
			f.fallible(next, f.e.obj("KwSetFor"), strLit(funcstr), cur, strLit(a.Name), v)
		}
		cur = ident(next)
	}
	return cur, nil
}
