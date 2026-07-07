package objects

import "strings"

// partialObject is functools.partial: a callable that freezes some leading
// positional arguments and some keywords of another callable, then forwards
// whatever a later call adds. func, args, and keywords are readable attributes,
// matching CPython's _functools.partial.
type partialObject struct {
	fn      Object
	args    []Object
	kwNames []string
	kwVals  []Object
}

func (*partialObject) TypeName() string { return "functools.partial" }

// NewPartial builds a partial over fn with the given frozen arguments. When fn
// is itself a partial the two are flattened, the way CPython folds
// partial(partial(f, 1), 2) into a single partial over f, with the outer
// keywords overriding the inner ones.
func NewPartial(fn Object, args []Object, kwNames []string, kwVals []Object) Object {
	if inner, ok := fn.(*partialObject); ok {
		mergedArgs := append(append([]Object(nil), inner.args...), args...)
		mn, mv := mergeKeywords(inner.kwNames, inner.kwVals, kwNames, kwVals)
		return &partialObject{fn: inner.fn, args: mergedArgs, kwNames: mn, kwVals: mv}
	}
	return &partialObject{
		fn:      fn,
		args:    append([]Object(nil), args...),
		kwNames: append([]string(nil), kwNames...),
		kwVals:  append([]Object(nil), kwVals...),
	}
}

// partialCall forwards a call to the wrapped callable: the frozen positionals
// come first, then the call's own, and the frozen keywords are overridden by
// any the call repeats.
func partialCall(p *partialObject, args []Object, kwNames []string, kwVals []Object) (Object, error) {
	finalArgs := append(append([]Object(nil), p.args...), args...)
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
// callable followed by the frozen positionals and keywords, matching CPython.
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
