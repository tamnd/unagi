package objects

import "strings"

// placeholderObject is functools.Placeholder: a sentinel a partial leaves in a
// frozen positional slot for a later call to fill. It is a singleton, so an
// identity test against Placeholder is how the partial machinery spots a hole.
type placeholderObject struct{}

// Placeholder is the single functools.Placeholder instance. _functools exposes
// it and the vendored functools.py rebinds Placeholder to it, so user code and
// the native partial share one sentinel.
var Placeholder Object = &placeholderObject{}

func (*placeholderObject) TypeName() string { return "functools._PlaceholderType" }

// NewPlaceholder returns the Placeholder singleton, the body of the
// _PlaceholderType constructor _functools exposes: the type is a singleton, so
// every call hands back the same instance.
func NewPlaceholder() Object { return Placeholder }

// partialObject is functools.partial: a callable that freezes some leading
// positional arguments and some keywords of another callable, then forwards
// whatever a later call adds. func, args, and keywords are readable attributes,
// matching CPython's _functools.partial. A frozen positional may be Placeholder,
// a hole a later call fills; phcount counts them and order is the itemgetter
// permutation that merges the call's leading positionals into the holes.
type partialObject struct {
	fn      Object
	args    []Object
	kwNames []string
	kwVals  []Object
	phcount int
	order   []int
}

func (*partialObject) TypeName() string { return "functools.partial" }

// partialPrepareMerger inspects the frozen positionals and returns the count of
// Placeholder holes and the itemgetter order that fills them, mirroring
// functools._partial_prepare_merger. Non-hole slots map to themselves; each hole
// maps past the frozen tail so a later call's leading positionals slot in by
// position. With no holes the order is nil and the call takes the fast path.
func partialPrepareMerger(args []Object) (int, []int) {
	if len(args) == 0 {
		return 0, nil
	}
	nargs := len(args)
	order := make([]int, nargs)
	j := nargs
	for i, a := range args {
		if a == Placeholder {
			order[i] = j
			j++
		} else {
			order[i] = i
		}
	}
	phcount := j - nargs
	if phcount == 0 {
		return 0, nil
	}
	return phcount, order
}

// applyMerger permutes seq by order, the itemgetter(*order) the partial machinery
// uses to weave a call's positionals into the frozen holes. order indexes into
// seq, so seq must be at least as long as the largest index.
func applyMerger(order []int, seq []Object) []Object {
	out := make([]Object, len(order))
	for i, k := range order {
		out[i] = seq[k]
	}
	return out
}

// NewPartial builds a functools.partial over fn with the given frozen arguments,
// the body of _functools.partial's constructor. It rejects a non-callable func, a
// trailing Placeholder, and a Placeholder passed as a keyword the way CPython
// does, and folds a partial-of-a-partial into a single partial over the innermost
// callable, merging the frozen arguments (holes included) and letting the outer
// keywords override the inner ones.
func NewPartial(fn Object, args []Object, kwNames []string, kwVals []Object) (Object, error) {
	if !Callable(fn) {
		return nil, Raise(TypeError, "the first argument must be callable")
	}
	if len(args) > 0 && args[len(args)-1] == Placeholder {
		return nil, Raise(TypeError, "trailing Placeholders are not allowed")
	}
	for _, v := range kwVals {
		if v == Placeholder {
			return nil, Raise(TypeError, "Placeholder cannot be passed as a keyword argument")
		}
	}

	var totArgs []Object
	var phcount int
	var order []int
	if inner, ok := fn.(*partialObject); ok {
		totArgs = append([]Object(nil), inner.args...)
		if len(args) > 0 {
			totArgs = append(totArgs, args...)
			if inner.phcount > 0 {
				nargs := len(args)
				for k := nargs; k < inner.phcount; k++ {
					totArgs = append(totArgs, Placeholder)
				}
				totArgs = applyMerger(inner.order, totArgs)
				if nargs > inner.phcount {
					totArgs = append(totArgs, args[inner.phcount:]...)
				}
			}
			phcount, order = partialPrepareMerger(totArgs)
		} else {
			phcount, order = inner.phcount, inner.order
		}
		kwNames, kwVals = mergeKeywords(inner.kwNames, inner.kwVals, kwNames, kwVals)
		fn = inner.fn
	} else {
		totArgs = append([]Object(nil), args...)
		phcount, order = partialPrepareMerger(totArgs)
	}

	return &partialObject{
		fn:      fn,
		args:    totArgs,
		kwNames: append([]string(nil), kwNames...),
		kwVals:  append([]Object(nil), kwVals...),
		phcount: phcount,
		order:   order,
	}, nil
}

// partialCall forwards a call to the wrapped callable: the frozen positionals
// come first, then the call's own, and the frozen keywords are overridden by any
// the call repeats. When the partial carries Placeholder holes the call's leading
// positionals fill them first, and a call that supplies fewer than the hole count
// is the CPython "missing positional arguments" TypeError.
func partialCall(p *partialObject, args []Object, kwNames []string, kwVals []Object) (Object, error) {
	ptoArgs := p.args
	rest := args
	if p.phcount > 0 {
		if len(args) < p.phcount {
			return nil, Raise(TypeError,
				"missing positional arguments in 'partial' call; expected at least %d, got %d",
				p.phcount, len(args))
		}
		combined := append(append([]Object(nil), p.args...), args...)
		ptoArgs = applyMerger(p.order, combined)
		rest = args[p.phcount:]
	}
	finalArgs := append(append([]Object(nil), ptoArgs...), rest...)
	mn, mv := mergeKeywords(p.kwNames, p.kwVals, kwNames, kwVals)
	return CallKw(p.fn, finalArgs, mn, mv)
}

// mergeKeywords overlays the call keywords onto the frozen ones, preserving the
// frozen order and appending any keyword the call introduces, so a repeated
// keyword takes the call's value in place.
func mergeKeywords(baseNames []string, baseVals []Object, addNames []string, addVals []Object) ([]string, []Object) {
	names := append([]string(nil), baseNames...)
	vals := append([]Object(nil), baseVals...)
	for i, n := range addNames {
		if j := indexOf(names, n); j >= 0 {
			vals[j] = addVals[i]
			continue
		}
		names = append(names, n)
		vals = append(vals, addVals[i])
	}
	return names, vals
}

func indexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

// UnwrapPartial peels a functools.partial chain down to the innermost wrapped
// callable, following .func while the value is a partial, the way functools'
// _unwrap_partial helper does. A value that is not a partial comes back
// unchanged.
func UnwrapPartial(o Object) Object {
	for {
		p, ok := o.(*partialObject)
		if !ok {
			return o
		}
		o = p.fn
	}
}

// partialAttr reads the three attributes a partial exposes: the wrapped
// callable, the frozen positionals as a tuple, and the frozen keywords as a
// dict.
func partialAttr(p *partialObject, name string) (Object, error) {
	switch name {
	case "func":
		return p.fn, nil
	case "args":
		return NewTuple(append([]Object(nil), p.args...)), nil
	case "keywords":
		keys := make([]Object, len(p.kwNames))
		for i, n := range p.kwNames {
			keys[i] = NewStr(n)
		}
		return NewDict(keys, append([]Object(nil), p.kwVals...))
	}
	return nil, Raise(AttributeError, "'functools.partial' object has no attribute '%s'", name)
}

// partialRepr spells functools.partial(func, args..., key=value...), the frozen
// callable followed by the frozen positionals and keywords, matching CPython. A
// Placeholder hole reprs as Placeholder through reprCore.
func partialRepr(p *partialObject, strict bool) (string, error) {
	var b strings.Builder
	b.WriteString("functools.partial(")
	fn, err := reprCore(p.fn, strict)
	if err != nil {
		return "", err
	}
	b.WriteString(fn)
	for _, a := range p.args {
		v, err := reprCore(a, strict)
		if err != nil {
			return "", err
		}
		b.WriteString(", ")
		b.WriteString(v)
	}
	for i, n := range p.kwNames {
		v, err := reprCore(p.kwVals[i], strict)
		if err != nil {
			return "", err
		}
		b.WriteString(", ")
		b.WriteString(n)
		b.WriteByte('=')
		b.WriteString(v)
	}
	b.WriteByte(')')
	return b.String(), nil
}
