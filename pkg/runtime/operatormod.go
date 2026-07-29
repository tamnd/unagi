package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _operator is the C accelerator behind the public operator module. operator.py
// defines every function in pure Python first, then does `from _operator import
// *` to override them with the faster C versions; the import is guarded by
// `except ImportError: pass`, so the module works with or without us. This slice
// registers the function surface (the arithmetic, comparison, sequence, and
// logical operators plus length_hint and call) so operator routes through the
// same primitives the interpreter uses for the corresponding syntax.
//
// The three generalized-lookup helpers attrgetter, itemgetter, and methodcaller
// stay on their pure-Python definitions in operator.py: they are stateful
// callables whose observable behavior (including pickling through __reduce__) is
// already correct there, and _operator omitting them simply leaves the pure
// classes in place after the star-import, with identical results.

func init() {
	moduleTable["_operator"] = &moduleEntry{builtin: true, exec: initOperator}
}

// operatorBinops are the two-argument operators that map straight onto an
// objects primitive returning (Object, error).
var operatorBinops = map[string]func(a, b objects.Object) (objects.Object, error){
	"add":      objects.Add,
	"sub":      objects.Sub,
	"mul":      objects.Mul,
	"truediv":  objects.TrueDiv,
	"floordiv": objects.FloorDiv,
	"mod":      objects.Mod,
	"pow":      objects.Pow,
	"lshift":   objects.LShift,
	"rshift":   objects.RShift,
	"and_":     objects.BitAnd,
	"or_":      objects.BitOr,
	"xor":      objects.BitXor,
	"matmul":   objects.MatMul,
	"getitem":  objects.GetItem,
	// contains(a, b) is `b in a`, note the reversed operands.
	"contains": func(a, b objects.Object) (objects.Object, error) { return objects.Contains(a, b) },
}

// operatorCmps are the six rich comparisons.
var operatorCmps = map[string]objects.CmpOp{
	"lt": objects.OpLt,
	"le": objects.OpLe,
	"eq": objects.OpEq,
	"ne": objects.OpNe,
	"gt": objects.OpGt,
	"ge": objects.OpGe,
}

// operatorUnops are the one-argument operators that map onto an objects
// primitive returning (Object, error).
var operatorUnops = map[string]func(a objects.Object) (objects.Object, error){
	"neg":    objects.Neg,
	"pos":    objects.Pos,
	"inv":    objects.Invert,
	"invert": objects.Invert,
	"abs":    Abs,
	"index":  operatorIndex,
	"not_":   func(a objects.Object) (objects.Object, error) { return objects.NewBool(!objects.Truth(a)), nil },
	"truth":  func(a objects.Object) (objects.Object, error) { return objects.NewBool(objects.Truth(a)), nil },
}

func initOperator(m *objects.Module) error {
	for name, fn := range operatorBinops {
		f := objects.NewFunc(name, 2, func(args []objects.Object) (objects.Object, error) {
			return fn(args[0], args[1])
		})
		if err := objects.StoreAttr(m, name, f); err != nil {
			return err
		}
	}
	for name, op := range operatorCmps {
		f := objects.NewFunc(name, 2, func(args []objects.Object) (objects.Object, error) {
			return objects.Compare(op, args[0], args[1])
		})
		if err := objects.StoreAttr(m, name, f); err != nil {
			return err
		}
	}
	for name, fn := range operatorUnops {
		f := objects.NewFunc(name, 1, func(args []objects.Object) (objects.Object, error) {
			return fn(args[0])
		})
		if err := objects.StoreAttr(m, name, f); err != nil {
			return err
		}
	}

	// Identity and None checks. Go interface equality compares the boxed pointer,
	// matching Python `is` for the same object.
	rest := []struct {
		name  string
		arity int
		fn    func(args []objects.Object) (objects.Object, error)
	}{
		{"is_", 2, func(a []objects.Object) (objects.Object, error) { return objects.NewBool(a[0] == a[1]), nil }},
		{"is_not", 2, func(a []objects.Object) (objects.Object, error) { return objects.NewBool(a[0] != a[1]), nil }},
		{"is_none", 1, func(a []objects.Object) (objects.Object, error) { return objects.NewBool(a[0] == objects.None), nil }},
		{"is_not_none", 1, func(a []objects.Object) (objects.Object, error) { return objects.NewBool(a[0] != objects.None), nil }},
		{"concat", 2, operatorConcat},
		{"setitem", 3, operatorSetitem},
		{"delitem", 2, operatorDelitem},
		{"countOf", 2, operatorCountOf},
		{"indexOf", 2, operatorIndexOf},
	}
	for _, e := range rest {
		f := objects.NewFunc(e.name, e.arity, e.fn)
		if err := objects.StoreAttr(m, e.name, f); err != nil {
			return err
		}
	}

	// length_hint(obj, default=0) and call(obj, /, *args, **kwargs) take
	// keywords / varargs, so they go through the keyword calling convention.
	if err := objects.StoreAttr(m, "length_hint", objects.NewFuncKw("length_hint", operatorLengthHint)); err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "call", objects.NewFuncKw("call", operatorCall)); err != nil {
		return err
	}
	return nil
}

// operatorIndex implements operator.index(a) == a.__index__(): the integer an
// object yields for indexing, preserving arbitrary precision.
func operatorIndex(a objects.Object) (objects.Object, error) {
	if objects.IsBigInt(a) {
		if b, ok := objects.AsBigInt(a); ok {
			return objects.NewIntFromBig(b), nil
		}
	}
	if i, ok := objects.AsInt(a); ok {
		return objects.NewInt(i), nil
	}
	// An int subclass indexes as its payload.
	if v, ok := objects.BuiltinValue(a); ok {
		return operatorIndex(v)
	}
	if r, ok, err := objects.IndexOf(a); err != nil {
		return nil, err
	} else if ok {
		return r, nil
	}
	return nil, objects.Raise(objects.TypeError,
		"'%s' object cannot be interpreted as an integer", a.TypeName())
}

// operatorConcat implements operator.concat(a, b): a + b restricted to
// sequences, so a non-subscriptable operand reports the concatenation error
// rather than falling through to numeric addition.
func operatorConcat(args []objects.Object) (objects.Object, error) {
	if _, err := objects.LoadAttr(args[0], "__getitem__"); err != nil {
		return nil, objects.Raise(objects.TypeError,
			"'%s' object can't be concatenated", args[0].TypeName())
	}
	return objects.Add(args[0], args[1])
}

// operatorSetitem implements operator.setitem(a, b, c): a[b] = c.
func operatorSetitem(args []objects.Object) (objects.Object, error) {
	if err := objects.SetItem(args[0], args[1], args[2]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// operatorDelitem implements operator.delitem(a, b): del a[b].
func operatorDelitem(args []objects.Object) (objects.Object, error) {
	if err := objects.DelItem(args[0], args[1]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// operatorCountOf implements operator.countOf(a, b): the number of items in a
// that are, or equal, b.
func operatorCountOf(args []objects.Object) (objects.Object, error) {
	it, err := objects.Iter(args[0])
	if err != nil {
		return nil, err
	}
	count := int64(0)
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		eq, err := operatorSameOrEqual(v, args[1])
		if err != nil {
			return nil, err
		}
		if eq {
			count++
		}
	}
	return objects.NewInt(count), nil
}

// operatorIndexOf implements operator.indexOf(a, b): the first index of b in a.
func operatorIndexOf(args []objects.Object) (objects.Object, error) {
	it, err := objects.Iter(args[0])
	if err != nil {
		return nil, err
	}
	idx := int64(0)
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		eq, err := operatorSameOrEqual(v, args[1])
		if err != nil {
			return nil, err
		}
		if eq {
			return objects.NewInt(idx), nil
		}
		idx++
	}
	return nil, objects.Raise(objects.ValueError, "sequence.index(x): x not in sequence")
}

// operatorSameOrEqual reports `i is b or i == b`, the membership test countOf and
// indexOf run over each element.
func operatorSameOrEqual(i, b objects.Object) (bool, error) {
	if i == b {
		return true, nil
	}
	r, err := objects.Compare(objects.OpEq, i, b)
	if err != nil {
		return false, err
	}
	return objects.Truth(r), nil
}

// operatorLengthHint implements operator.length_hint(obj, default=0): len(obj)
// when sized, else __length_hint__, else default.
func operatorLengthHint(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 || len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError,
			"length_hint() takes at most 2 arguments (%d given)", len(pos))
	}
	def := objects.NewInt(0)
	if len(pos) == 2 {
		def = pos[1]
	}
	for i, kn := range kwNames {
		if kn == "default" {
			def = kwVals[i]
		} else {
			return nil, objects.Raise(objects.TypeError,
				"'%s' is an invalid keyword argument for length_hint()", kn)
		}
	}
	if _, ok := objects.AsInt(def); !ok {
		return nil, objects.Raise(objects.TypeError,
			"'%s' object cannot be interpreted as an integer", def.TypeName())
	}

	obj := pos[0]
	if n, err := objects.Len(obj); err == nil {
		return objects.NewInt(int64(n)), nil
	}

	hint, err := objects.LoadAttr(obj, "__length_hint__")
	if err != nil {
		return def, nil
	}
	val, err := objects.Call(hint, nil)
	if err != nil {
		if ex, ok := err.(*objects.Exception); ok && ex.Kind == "TypeError" {
			return def, nil
		}
		return nil, err
	}
	if val == objects.NotImplemented {
		return def, nil
	}
	iv, ok := objects.AsInt(val)
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"__length_hint__ must be integer, not %s", val.TypeName())
	}
	if iv < 0 {
		return nil, objects.Raise(objects.ValueError, "__length_hint__() should return >= 0")
	}
	return objects.NewInt(iv), nil
}

// operatorCall implements operator.call(obj, /, *args, **kwargs): obj(*args,
// **kwargs).
func operatorCall(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError,
			"call expected at least 1 argument, got 0")
	}
	return objects.CallKw(pos[0], pos[1:], kwNames, kwVals)
}
