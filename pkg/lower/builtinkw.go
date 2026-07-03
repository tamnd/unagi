package lower

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/objects"
)

// This file lowers builtin calls that carry keyword arguments. Each builtin
// that accepts keywords on 3.14 gets its probed binding rules; everything
// else raises the probed "takes no keyword arguments" TypeError at the call
// site so it stays catchable. All wordings come from python3.14 probes.

// builtinKwCall handles a builtin call with at least one keyword argument.
func (f *fnCtx) builtinKwCall(name string, e *frontend.Call) (ast.Expr, error) {
	// Rejections that need no argument evaluation come first.
	switch name {
	case "print":
		for _, a := range e.Args {
			if a.Name != "file" {
				continue
			}
			if _, isNone := a.Value.(*frontend.NoneLit); !isNone {
				return nil, f.e.errf(a.Pos_, "print() file argument other than None is not supported yet")
			}
		}
	}

	pos, kws, temps, err := f.evalArgs(e)
	if err != nil {
		return nil, err
	}

	switch name {
	case "print":
		return f.printKw(pos, kws, temps)
	case "str":
		return f.strKw(pos, kws, temps, e)
	case "int":
		return f.intKw(pos, kws, temps)
	case "sum":
		return f.sumKw(pos, kws, temps)
	case "round":
		return f.clinicKw("round", "Round", []string{"number", "ndigits"}, 1, pos, kws, temps)
	case "pow":
		return f.clinicKw("pow", "", []string{"base", "exp", "mod"}, 2, pos, kws, temps)
	case "enumerate":
		return f.enumerateKw(pos, kws, temps)
	case "sorted":
		return f.sortedKw(pos, kws, temps, e)
	case "min", "max":
		return f.minMaxKw(name, pos, kws, temps, e)
	case "zip":
		return f.zipKw(pos, kws, temps)
	case "dict":
		return f.dictKw(pos, kws, e)
	}
	// Probed across len, abs, range, bool, list, tuple, set, frozenset,
	// divmod, repr, bin, oct, hex, ord, chr, reversed, format and float:
	// every one answers with the same TypeError.
	return f.raiseBindError(temps, fmt.Sprintf("%s() takes no keyword arguments", name)), nil
}

// discard keeps an evaluated-but-unused temporary alive for Go.
func (f *fnCtx) discard(v ast.Expr) {
	f.add(assign(token.ASSIGN, []ast.Expr{ident("_")}, v))
}

func (f *fnCtx) unexpectedKw(fname, kw string, candidates []string) string {
	return objects.UnexpectedKwMsg(fname, kw, candidates)
}

// printKw lowers print(*args, sep=..., end=..., flush=..., file=None).
// flush is accepted and dropped because Stdout never buffers, and file
// already passed the literal-None check.
func (f *fnCtx) printKw(pos []ast.Expr, kws []kwVal, temps []ast.Expr) (ast.Expr, error) {
	var sepV, endV ast.Expr
	for _, kw := range kws {
		switch kw.name {
		case "sep":
			sepV = kw.val
		case "end":
			endV = kw.val
		case "flush", "file":
			f.discard(kw.val)
		default:
			return f.raiseBindError(temps, f.unexpectedKw("print", kw.name, []string{"sep", "end", "file", "flush"})), nil
		}
	}
	if sepV == nil {
		sepV = f.e.obj("None")
	}
	if endV == nil {
		endV = f.e.obj("None")
	}
	f.fallibleVoid(sel("runtime", "PrintKw"), f.objSlice(pos), sepV, endV)
	return f.e.obj("None"), nil
}

// strKw lowers str(object=...). The encoding form needs bytes, which M1
// does not have.
func (f *fnCtx) strKw(pos []ast.Expr, kws []kwVal, temps []ast.Expr, e *frontend.Call) (ast.Expr, error) {
	if len(pos) > 1 {
		return nil, f.e.errf(e.Span(), "str() with an encoding is not supported yet")
	}
	var objV ast.Expr
	for _, kw := range kws {
		switch kw.name {
		case "object":
			if len(pos) >= 1 {
				return f.raiseBindError(temps, "argument for str() given by name ('object') and position (1)"), nil
			}
			objV = kw.val
		case "encoding", "errors":
			return nil, f.e.errf(e.Span(), "str() with an encoding is not supported yet")
		default:
			return f.raiseBindError(temps, f.unexpectedKw("str", kw.name, []string{"object", "encoding", "errors"})), nil
		}
	}
	src := objV
	if src == nil {
		src = pos[0]
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", "StrOf"), src)
	return ident(tmp), nil
}

// intKw lowers int(x, base=...). Probed: base is the only keyword, x is
// positional-only so int(x="12") gets the unexpected-keyword answer, and
// int(base=10) with no string says "int() missing string argument".
func (f *fnCtx) intKw(pos []ast.Expr, kws []kwVal, temps []ast.Expr) (ast.Expr, error) {
	if total := len(pos) + len(kws); total > 2 {
		return f.raiseBindError(temps, fmt.Sprintf("int() takes at most 2 arguments (%d given)", total)), nil
	}
	var baseV ast.Expr
	for _, kw := range kws {
		if kw.name != "base" {
			return f.raiseBindError(temps, f.unexpectedKw("int", kw.name, []string{"base"})), nil
		}
		baseV = kw.val
	}
	if len(pos) == 2 {
		// Two positionals plus base already tripped the count check, so a
		// second positional here means base came by position.
		baseV = pos[1]
	}
	if len(pos) == 0 {
		return f.raiseBindError(temps, "int() missing string argument"), nil
	}
	tmp := f.tmpVar()
	if baseV != nil {
		f.fallible(tmp, sel("runtime", "IntOfBase"), pos[0], baseV)
	} else {
		f.fallible(tmp, sel("runtime", "IntOf"), pos[0])
	}
	return ident(tmp), nil
}

// sumKw lowers sum(iterable, start=...). Probed: the count checks fire
// before the keyword names are looked at.
func (f *fnCtx) sumKw(pos []ast.Expr, kws []kwVal, temps []ast.Expr) (ast.Expr, error) {
	if len(pos) == 0 {
		return f.raiseBindError(temps, "sum() takes at least 1 positional argument (0 given)"), nil
	}
	if total := len(pos) + len(kws); total > 2 {
		return f.raiseBindError(temps, fmt.Sprintf("sum() takes at most 2 arguments (%d given)", total)), nil
	}
	args := pos
	for _, kw := range kws {
		if kw.name != "start" {
			return f.raiseBindError(temps, f.unexpectedKw("sum", kw.name, []string{"start"})), nil
		}
		args = append(args[:len(args):len(args)], kw.val)
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", "Sum"), f.objSlice(args))
	return ident(tmp), nil
}

// clinicKw lowers the argument-clinic builtins round and pow: positional
// slots fillable by name, a probed two-pass check where missing required
// arguments outrank name-and-position conflicts, and "(pos N)" spelling.
// nReq is the count of required leading slots; fn names the runtime helper
// taking a slice, or "" for pow's special dispatch.
func (f *fnCtx) clinicKw(fname, fn string, names []string, nReq int, pos []ast.Expr, kws []kwVal, temps []ast.Expr) (ast.Expr, error) {
	if total := len(pos) + len(kws); total > len(names) {
		return f.raiseBindError(temps, fmt.Sprintf("%s() takes at most %d arguments (%d given)", fname, len(names), total)), nil
	}
	slots := make([]ast.Expr, len(names))
	copy(slots, pos)
	conflict := -1
	for _, kw := range kws {
		idx := -1
		for i, n := range names {
			if n == kw.name {
				idx = i
				break
			}
		}
		if idx < 0 {
			return f.raiseBindError(temps, f.unexpectedKw(fname, kw.name, names)), nil
		}
		if idx < len(pos) {
			if conflict < 0 {
				conflict = idx
			}
			continue
		}
		slots[idx] = kw.val
	}
	for i := 0; i < nReq; i++ {
		if slots[i] == nil {
			return f.raiseBindError(temps, fmt.Sprintf("%s() missing required argument '%s' (pos %d)", fname, names[i], i+1)), nil
		}
	}
	if conflict >= 0 {
		return f.raiseBindError(temps,
			fmt.Sprintf("argument for %s() given by name ('%s') and position (%d)", fname, names[conflict], conflict+1)), nil
	}
	args := slots
	for len(args) > nReq && args[len(args)-1] == nil {
		args = args[:len(args)-1]
	}
	tmp := f.tmpVar()
	if fname == "pow" {
		if len(args) == 2 {
			f.fallible(tmp, f.e.obj("Pow"), args[0], args[1])
		} else {
			f.fallible(tmp, sel("runtime", "Pow3"), args[0], args[1], args[2])
		}
		return ident(tmp), nil
	}
	f.fallible(tmp, sel("runtime", fn), f.objSlice(args))
	return ident(tmp), nil
}

// enumerateKw lowers enumerate(iterable=..., start=...). Probed: the
// failure wording is the invalid-keyword pattern with no suggestions, a
// duplicate iterable answers with 'iterable', and start without a bound
// iterable answers with 'start'.
func (f *fnCtx) enumerateKw(pos []ast.Expr, kws []kwVal, temps []ast.Expr) (ast.Expr, error) {
	invalid := func(n string) (ast.Expr, error) {
		return f.raiseBindError(temps, fmt.Sprintf("'%s' is an invalid keyword argument for enumerate()", n)), nil
	}
	if total := len(pos) + len(kws); total > 2 {
		return f.raiseBindError(temps, fmt.Sprintf("enumerate() takes at most 2 arguments (%d given)", total)), nil
	}
	slots := make([]ast.Expr, 2)
	copy(slots, pos)
	for _, kw := range kws {
		switch kw.name {
		case "iterable":
			if slots[0] != nil {
				return invalid("iterable")
			}
			slots[0] = kw.val
		case "start":
			slots[1] = kw.val
		default:
			return invalid(kw.name)
		}
	}
	if slots[0] == nil {
		return invalid("start")
	}
	args := slots[:1]
	if slots[1] != nil {
		args = slots
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", "Enumerate"), f.objSlice(args))
	return ident(tmp), nil
}

// sortedKw lowers sorted(iterable, key=..., reverse=...). Probed: the
// unexpected-keyword message says sort(), not sorted().
func (f *fnCtx) sortedKw(pos []ast.Expr, kws []kwVal, temps []ast.Expr, e *frontend.Call) (ast.Expr, error) {
	if len(pos) != 1 {
		return nil, f.e.errf(e.Span(), "sorted expected 1 argument, got %d", len(pos))
	}
	var keyV, revV ast.Expr
	for _, kw := range kws {
		switch kw.name {
		case "key":
			keyV = kw.val
		case "reverse":
			revV = kw.val
		default:
			return f.raiseBindError(temps, f.unexpectedKw("sort", kw.name, []string{"key", "reverse"})), nil
		}
	}
	if keyV == nil {
		keyV = f.e.obj("None")
	}
	if revV == nil {
		revV = f.e.obj("False")
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", "SortedKw"), pos[0], keyV, revV)
	return ident(tmp), nil
}

// minMaxKw lowers min/max with key= and default=. The default sentinel is
// a Go nil so an explicit default=None still counts as present.
func (f *fnCtx) minMaxKw(name string, pos []ast.Expr, kws []kwVal, temps []ast.Expr, e *frontend.Call) (ast.Expr, error) {
	if len(pos) == 0 {
		return nil, f.e.errf(e.Span(), "%s expected at least 1 argument, got 0", name)
	}
	var keyV, dfltV ast.Expr
	for _, kw := range kws {
		switch kw.name {
		case "key":
			keyV = kw.val
		case "default":
			dfltV = kw.val
		default:
			return f.raiseBindError(temps, f.unexpectedKw(name, kw.name, []string{"key", "default"})), nil
		}
	}
	if dfltV != nil && len(pos) > 1 {
		return f.raiseBindError(temps,
			fmt.Sprintf("Cannot specify a default for %s() with multiple positional arguments", name)), nil
	}
	if keyV == nil {
		keyV = f.e.obj("None")
	}
	if dfltV == nil {
		dfltV = ident("nil")
	}
	fn := "MinKw"
	if name == "max" {
		fn = "MaxKw"
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", fn), f.objSlice(pos), keyV, dfltV)
	return ident(tmp), nil
}

// zipKw lowers zip(*iterables, strict=...).
func (f *fnCtx) zipKw(pos []ast.Expr, kws []kwVal, temps []ast.Expr) (ast.Expr, error) {
	var strictV ast.Expr
	for _, kw := range kws {
		if kw.name != "strict" {
			return f.raiseBindError(temps, f.unexpectedKw("zip", kw.name, []string{"strict"})), nil
		}
		strictV = kw.val
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", "ZipStrict"), f.objSlice(pos), strictV)
	return ident(tmp), nil
}

// dictKw lowers dict(**kw): keyword entries land after the positional
// entries and a duplicate key updates in place, both probed.
func (f *fnCtx) dictKw(pos []ast.Expr, kws []kwVal, e *frontend.Call) (ast.Expr, error) {
	if len(pos) > 1 {
		return nil, f.e.errf(e.Span(), "dict expected at most 1 argument, got %d", len(pos))
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", "DictOf"), f.objSlice(pos))
	for _, kw := range kws {
		f.fallibleVoid(f.e.obj("SetItem"), ident(tmp), callExpr(f.e.obj("NewStr"), strLit(kw.name)), kw.val)
	}
	return ident(tmp), nil
}
