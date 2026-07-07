package objects

// This file implements user-class rich comparison, the __eq__/__ne__/__lt__/
// __le__/__gt__/__ge__ dispatch that CPython drives from do_richcompare. It runs
// only when at least one operand is a user instance; two builtins keep the fast
// equals/order path in ops.go, which never consults a dunder.

// richDunder names the forward special method for each comparison operator.
var richDunder = map[CmpOp]string{
	OpEq: "__eq__", OpNe: "__ne__",
	OpLt: "__lt__", OpLe: "__le__", OpGt: "__gt__", OpGe: "__ge__",
}

// richReflect maps an operator to the one its reflected operand answers: the
// reflected form of a < b is b > a, so `<` reflects to `>`. Equality reflects
// to itself.
var richReflect = map[CmpOp]CmpOp{
	OpEq: OpEq, OpNe: OpNe,
	OpLt: OpGt, OpLe: OpGe, OpGt: OpLt, OpGe: OpLe,
}

// isInstance reports whether o is a user-class instance, the only kind of value
// that carries comparison dunders in the boxed tier.
func isInstance(o Object) bool {
	_, ok := o.(*instanceObject)
	return ok
}

// richCompare runs do_richcompare for a comparison where at least one operand is
// a user instance. It tries the two operand slots in CPython's order, honors a
// NotImplemented decline, and falls back to identity for ==/!= or the unorderable
// TypeError for an ordering, exactly as the reference does.
func richCompare(op CmpOp, a, b Object) (Object, error) {
	rop := richReflect[op]
	// A subclass right operand that overrides the reflected slot answers first.
	if reflectFirst(op, a, b) {
		if res, ok, err := richSlot(b, rop, a); err != nil || ok {
			return res, err
		}
		if res, ok, err := richSlot(a, op, b); err != nil || ok {
			return res, err
		}
	} else {
		if res, ok, err := richSlot(a, op, b); err != nil || ok {
			return res, err
		}
		if res, ok, err := richSlot(b, rop, a); err != nil || ok {
			return res, err
		}
	}
	// Both operands declined: == and != fall back to identity, an ordering
	// raises the same unorderable TypeError the builtin path spells.
	switch op {
	case OpEq:
		return NewBool(a == b), nil
	case OpNe:
		return NewBool(a != b), nil
	}
	return nil, Raise(TypeError, "'%s' not supported between instances of '%s' and '%s'",
		cmpSym[op], a.TypeName(), b.TypeName())
}

// richSlot invokes one operand's comparison dunder. ok is false when the operand
// is a builtin, when its type defines no such method, or when the method returns
// NotImplemented, in every case letting the caller try the other operand. A user
// __ne__ that is only inherited from object derives from __eq__, matching
// object.__ne__'s default negation.
func richSlot(x Object, op CmpOp, other Object) (res Object, ok bool, err error) {
	inst, isInst := x.(*instanceObject)
	if !isInst {
		// A builtin does not know how to compare against a user instance; it
		// yields NotImplemented so the instance's slot decides.
		return nil, false, nil
	}
	res, defined, err := instanceSpecial(inst, richDunder[op], other)
	if err != nil {
		return nil, false, err
	}
	if defined {
		if _, ni := res.(*notImplementedObject); ni {
			return nil, false, nil
		}
		return res, true, nil
	}
	// A value subclass with no comparison override compares as its payload, so an
	// int subclass member equals and orders against ints the way its value does.
	// The other operand unwraps too when it is a value subclass, leaving two
	// builtins for the fast comparison path.
	if v, ok := builtinUnwrap(x); ok {
		ov := other
		if uv, ok := builtinUnwrap(other); ok {
			ov = uv
		}
		r, err := Compare(op, v, ov)
		if err != nil {
			return nil, false, err
		}
		return r, true, nil
	}
	if op == OpNe {
		// object.__ne__ negates __eq__ unless __eq__ itself declines.
		eq, hasEq, err := instanceSpecial(inst, "__eq__", other)
		if err != nil {
			return nil, false, err
		}
		if hasEq {
			if _, ni := eq.(*notImplementedObject); ni {
				return nil, false, nil
			}
			// object.__ne__ runs PyObject_IsTrue on the __eq__ result, so a
			// user object with its own __bool__ decides the negation.
			t, terr := TruthOf(eq)
			if terr != nil {
				return nil, false, terr
			}
			return NewBool(!t), true, nil
		}
	}
	return nil, false, nil
}

// reflectFirst reports whether the right operand's reflected slot should run
// before the left operand's forward slot. CPython gives that priority when the
// right operand's type is a proper subclass of the left's and overrides the
// reflected method, so a subclass can refine a base's comparison.
func reflectFirst(op CmpOp, a, b Object) bool {
	ai, aok := a.(*instanceObject)
	bi, bok := b.(*instanceObject)
	if !aok || !bok || ai.cls == bi.cls {
		return false
	}
	if !isProperSubclass(bi.cls, ai.cls) {
		return false
	}
	name := richDunder[richReflect[op]]
	bDef, bHas := bi.cls.lookup(name)
	if !bHas {
		return false
	}
	aDef, aHas := ai.cls.lookup(name)
	return !aHas || bDef != aDef
}
