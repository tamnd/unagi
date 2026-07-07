package objects

// TotalOrdering is functools.total_ordering: given a class that defines __eq__
// and at least one ordering operation, it fills in the rest from the one it
// prefers (< over <= over > over >=). Each synthesized method calls the root
// operation and, unless that returns NotImplemented, combines the result with an
// equality check exactly as CPython's functools does. A class with no ordering
// operation is the ValueError CPython raises.
func TotalOrdering(cls Object) (Object, error) {
	c, ok := cls.(*classObject)
	if !ok {
		return nil, Raise(TypeError, "total_ordering() argument must be a class")
	}
	// The four ops in preference order; the first the class defines is the root.
	pref := []string{"__lt__", "__le__", "__gt__", "__ge__"}
	roots := map[string]bool{}
	for _, op := range pref {
		if _, found := c.lookup(op); found {
			roots[op] = true
		}
	}
	if len(roots) == 0 {
		return nil, Raise(ValueError, "must define at least one ordering operation: < > <= >=")
	}
	var root string
	for _, op := range pref {
		if roots[op] {
			root = op
			break
		}
	}
	for _, d := range orderingDerivations[root] {
		if roots[d.target] {
			continue
		}
		c.setAttr(d.target, newDerivedComparison(d.target, root, d.formula))
	}
	return cls, nil
}

// orderingFormula names the way a derived comparison combines the root result
// with equality. The letters follow the five distinct shapes in CPython's
// _convert table.
type orderingFormula int

const (
	// formNotOp is `not op`.
	formNotOp orderingFormula = iota
	// formNotOpAndNe is `not op and self != other`.
	formNotOpAndNe
	// formOpOrEq is `op or self == other`.
	formOpOrEq
	// formOpAndNe is `op and self != other`.
	formOpAndNe
	// formNotOpOrEq is `not op or self == other`.
	formNotOpOrEq
)

type orderingDerivation struct {
	target  string
	formula orderingFormula
}

// orderingDerivations mirrors functools._convert: for each root operation, the
// three others it can produce and the formula each uses.
var orderingDerivations = map[string][]orderingDerivation{
	"__lt__": {{"__gt__", formNotOpAndNe}, {"__le__", formOpOrEq}, {"__ge__", formNotOp}},
	"__le__": {{"__ge__", formNotOpOrEq}, {"__lt__", formOpAndNe}, {"__gt__", formNotOp}},
	"__gt__": {{"__lt__", formNotOpAndNe}, {"__ge__", formOpOrEq}, {"__le__", formNotOp}},
	"__ge__": {{"__le__", formNotOpOrEq}, {"__gt__", formOpAndNe}, {"__lt__", formNotOp}},
}

// newDerivedComparison builds the method total_ordering installs for target,
// computed from the root operation. It takes self and other, calls the root, and
// short-circuits NotImplemented before applying the formula.
func newDerivedComparison(target, root string, formula orderingFormula) Object {
	impl := func(a []Object) (Object, error) {
		self, other := a[0], a[1]
		op, err := CallMethod(self, root, []Object{other})
		if err != nil {
			return nil, err
		}
		if op == NotImplemented {
			return op, nil
		}
		t, err := TruthOf(op)
		if err != nil {
			return nil, err
		}
		switch formula {
		case formNotOp:
			return NewBool(!t), nil
		case formNotOpAndNe:
			if t {
				return False, nil
			}
			return Compare(OpNe, self, other)
		case formOpOrEq:
			if t {
				return op, nil
			}
			return Compare(OpEq, self, other)
		case formOpAndNe:
			if !t {
				return op, nil
			}
			return Compare(OpNe, self, other)
		default: // formNotOpOrEq
			if !t {
				return True, nil
			}
			return Compare(OpEq, self, other)
		}
	}
	return NewFunction(target, []Param{
		{Name: "self", Kind: ParamPlain},
		{Name: "other", Kind: ParamPlain},
	}, nil, impl)
}
