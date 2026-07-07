package objects

// notImplementedObject backs the NotImplemented singleton. A binary-operator
// dunder returns it to say "I don't handle this pair", which hands the operation
// to the reflected method and, failing that, to the unsupported-operand error.
type notImplementedObject struct{}

func (*notImplementedObject) TypeName() string { return "NotImplementedType" }

// NotImplemented is the singleton user code returns from __add__ and friends.
// Identity against this pointer is how the dispatch recognizes a declined op.
var NotImplemented Object = &notImplementedObject{}

// opDunders maps each binary operator symbol to its forward and reflected
// dunder names, so one fallback path serves every arithmetic and bitwise op.
var opDunders = map[string][2]string{
	"+":  {"__add__", "__radd__"},
	"-":  {"__sub__", "__rsub__"},
	"*":  {"__mul__", "__rmul__"},
	"@":  {"__matmul__", "__rmatmul__"},
	"/":  {"__truediv__", "__rtruediv__"},
	"//": {"__floordiv__", "__rfloordiv__"},
	"%":  {"__mod__", "__rmod__"},
	"**": {"__pow__", "__rpow__"},
	"<<": {"__lshift__", "__rlshift__"},
	">>": {"__rshift__", "__rrshift__"},
	"&":  {"__and__", "__rand__"},
	"|":  {"__or__", "__ror__"},
	"^":  {"__xor__", "__rxor__"},
}

// binFallback runs the binary-operator dunder protocol for a pair the builtin
// paths could not handle, then raises the unsupported-operand TypeError if
// neither operand participates. Every arithmetic and bitwise op routes its
// no-match return here so a user class defining __op__ or __rop__ works.
func binFallback(op string, a, b Object) (Object, error) {
	names := opDunders[op]
	if res, ok, err := binaryDunder(names[0], names[1], a, b); ok || err != nil {
		return res, err
	}
	if res, ok, err := valueSubclassBinary(op, a, b); ok || err != nil {
		return res, err
	}
	return nil, unsupported(op, a, b)
}

// binOp returns the concrete binary function for an operator symbol, so a value
// subclass operand can recompute the operation on its unwrapped payload. It is a
// switch rather than a package map to avoid an initialization cycle through the
// operator functions, which route back here.
func binOp(op string) func(a, b Object) (Object, error) {
	switch op {
	case "+":
		return Add
	case "-":
		return Sub
	case "*":
		return Mul
	case "@":
		return MatMul
	case "/":
		return TrueDiv
	case "//":
		return FloorDiv
	case "%":
		return Mod
	case "**":
		return Pow
	case "<<":
		return LShift
	case ">>":
		return RShift
	case "&":
		return BitAnd
	case "|":
		return BitOr
	case "^":
		return BitXor
	}
	return nil
}

// valueSubclassBinary handles a binary operator whose builtin fast paths and the
// dunder protocol both declined, when at least one operand is a value subclass
// instance with no operator override. It unwraps such operands to their builtin
// payload and recomputes, so a plain int subclass adds and shifts as its int and
// returns the plain builtin result CPython does. ok is false when neither
// operand unwraps, letting the caller raise the unsupported-operand error. A
// recomputed unsupported-operand error is re-raised with the original operand
// type names, the subclass-named message CPython reports.
func valueSubclassBinary(op string, a, b Object) (Object, bool, error) {
	ua, oka := builtinUnwrap(a)
	ub, okb := builtinUnwrap(b)
	if !oka && !okb {
		return nil, false, nil
	}
	if !oka {
		ua = a
	}
	if !okb {
		ub = b
	}
	fn := binOp(op)
	if fn == nil {
		return nil, false, nil
	}
	r, err := fn(ua, ub)
	if err != nil {
		if e, isExc := err.(*Exception); isExc && e.Kind == TypeError &&
			e.Error() == unsupported(op, ua, ub).Error() {
			return nil, true, unsupported(op, a, b)
		}
		return nil, true, err
	}
	return r, true, nil
}

// binaryDunder implements CPython's binary_op1: try the left operand's forward
// method and the right operand's reflected method, reflected first when the
// right type is a proper subclass of the left type and overrides the reflected
// slot. It reports ok=false (letting the caller raise) when neither method
// exists or both return NotImplemented, and threads through any raised error.
func binaryDunder(forward, reflected string, a, b Object) (Object, bool, error) {
	lfn := instDunderFn(a, forward)
	rfn := instDunderFn(b, reflected)
	// Skip the reflected call when it resolves to the same function the left
	// operand would use, which covers same-type operands and shared
	// inheritance the way CPython nulls a duplicated slot.
	if rfn != nil && rfn == instDunderFn(a, reflected) {
		rfn = nil
	}
	if lfn == nil && rfn == nil {
		return nil, false, nil
	}
	reflectedFirst := false
	if lfn != nil && rfn != nil {
		if isProperSubclass(b.(*instanceObject).cls, a.(*instanceObject).cls) {
			reflectedFirst = true
		}
	}
	calls := [2]struct {
		fn      *functionObject
		recv, x Object
	}{
		{lfn, a, b},
		{rfn, b, a},
	}
	if reflectedFirst {
		calls[0], calls[1] = calls[1], calls[0]
	}
	for _, c := range calls {
		if c.fn == nil {
			continue
		}
		res, err := c.fn.bind([]Object{c.recv, c.x}, nil, nil)
		if err != nil {
			return nil, true, err
		}
		if res != NotImplemented {
			return res, true, nil
		}
	}
	return nil, false, nil
}

// instDunderFn returns the plain function a user instance would call for the
// named dunder, or nil when o is not a user instance, the name is absent, or it
// resolves to something other than a plain function. Non-function dunders need
// the descriptor call path and stay a later refinement.
func instDunderFn(o Object, name string) *functionObject {
	inst, ok := o.(*instanceObject)
	if !ok {
		return nil
	}
	v, ok := inst.cls.lookup(name)
	if !ok {
		return nil
	}
	fn, _ := v.(*functionObject)
	return fn
}

// isProperSubclass reports whether sub is a strict subclass of super, walking
// sub's MRO. It backs the reflected-first rule so a subclass's reflected method
// gets the first attempt over its base's forward method.
func isProperSubclass(sub, super *classObject) bool {
	if sub == super {
		return false
	}
	for _, c := range sub.mro {
		if c == super {
			return true
		}
	}
	return false
}

// MatMul implements the @ operator. No builtin type defines matrix
// multiplication, so it goes straight to the dunder protocol and otherwise
// raises the unsupported-operand TypeError.
func MatMul(a, b Object) (Object, error) {
	return binFallback("@", a, b)
}
