package objects

import (
	"math"
	"strings"
)

// Truth implements Python truthiness.
func Truth(o Object) bool {
	switch x := o.(type) {
	case *noneObject:
		return false
	case *boolObject:
		return x.v
	case *intObject:
		return x.v != 0
	case *floatObject:
		return x.v != 0
	case *strObject:
		return len(x.v) > 0
	case *listObject:
		return len(x.elts) > 0
	case *tupleObject:
		return len(x.elts) > 0
	case *dictObject:
		return len(x.entries) > 0
	case *rangeObject:
		return x.length() > 0
	case *dictKeysObject:
		return len(x.d.entries) > 0
	case *dictValuesObject:
		return len(x.d.entries) > 0
	case *dictItemsObject:
		return len(x.d.entries) > 0
	case *Exception:
		// Exception instances are always truthy, whatever their args.
		return true
	}
	return true
}

// Not implements the `not` operator.
func Not(o Object) Object { return NewBool(!Truth(o)) }

// Neg implements unary minus.
func Neg(o Object) (Object, error) {
	if i, ok := AsInt(o); ok {
		return NewInt(-i), nil
	}
	if f, ok := o.(*floatObject); ok {
		return NewFloat(-f.v), nil
	}
	return nil, Raise(TypeError, "bad operand type for unary -: '%s'", o.TypeName())
}

// Pos implements unary plus.
func Pos(o Object) (Object, error) {
	if i, ok := AsInt(o); ok {
		return NewInt(i), nil
	}
	if f, ok := o.(*floatObject); ok {
		return NewFloat(f.v), nil
	}
	return nil, Raise(TypeError, "bad operand type for unary +: '%s'", o.TypeName())
}

func unsupported(op string, a, b Object) error {
	return Raise(TypeError, "unsupported operand type(s) for %s: '%s' and '%s'",
		op, a.TypeName(), b.TypeName())
}

func bothInt(a, b Object) (int64, int64, bool) {
	ai, aok := AsInt(a)
	bi, bok := AsInt(b)
	return ai, bi, aok && bok
}

func bothFloat(a, b Object) (float64, float64, bool) {
	af, aok := AsFloat(a)
	bf, bok := AsFloat(b)
	return af, bf, aok && bok
}

// Add implements the + operator.
func Add(a, b Object) (Object, error) {
	switch x := a.(type) {
	case *strObject:
		if y, ok := b.(*strObject); ok {
			return NewStr(x.v + y.v), nil
		}
		return nil, Raise(TypeError, "can only concatenate str (not %q) to str", b.TypeName())
	case *listObject:
		if y, ok := b.(*listObject); ok {
			out := make([]Object, 0, len(x.elts)+len(y.elts))
			out = append(out, x.elts...)
			out = append(out, y.elts...)
			return NewList(out), nil
		}
		return nil, Raise(TypeError, "can only concatenate list (not %q) to list", b.TypeName())
	case *tupleObject:
		if y, ok := b.(*tupleObject); ok {
			out := make([]Object, 0, len(x.elts)+len(y.elts))
			out = append(out, x.elts...)
			out = append(out, y.elts...)
			return NewTuple(out), nil
		}
		return nil, Raise(TypeError, "can only concatenate tuple (not %q) to tuple", b.TypeName())
	}
	if ai, bi, ok := bothInt(a, b); ok {
		return NewInt(ai + bi), nil
	}
	if af, bf, ok := bothFloat(a, b); ok {
		return NewFloat(af + bf), nil
	}
	return nil, unsupported("+", a, b)
}

// Sub implements the - operator.
func Sub(a, b Object) (Object, error) {
	if ai, bi, ok := bothInt(a, b); ok {
		return NewInt(ai - bi), nil
	}
	if af, bf, ok := bothFloat(a, b); ok {
		return NewFloat(af - bf), nil
	}
	return nil, unsupported("-", a, b)
}

func isSequence(o Object) bool {
	switch o.(type) {
	case *strObject, *listObject, *tupleObject:
		return true
	}
	return false
}

func repeatSeq(seq Object, n int64) Object {
	if n < 0 {
		n = 0
	}
	switch x := seq.(type) {
	case *strObject:
		return NewStr(strings.Repeat(x.v, int(n)))
	case *listObject:
		out := make([]Object, 0, int64(len(x.elts))*n)
		for i := int64(0); i < n; i++ {
			out = append(out, x.elts...)
		}
		return NewList(out)
	case *tupleObject:
		out := make([]Object, 0, int64(len(x.elts))*n)
		for i := int64(0); i < n; i++ {
			out = append(out, x.elts...)
		}
		return NewTuple(out)
	}
	return nil
}

// Mul implements the * operator.
func Mul(a, b Object) (Object, error) {
	if isSequence(a) {
		if n, ok := AsInt(b); ok {
			return repeatSeq(a, n), nil
		}
		return nil, Raise(TypeError, "can't multiply sequence by non-int of type '%s'", b.TypeName())
	}
	if isSequence(b) {
		if n, ok := AsInt(a); ok {
			return repeatSeq(b, n), nil
		}
		return nil, Raise(TypeError, "can't multiply sequence by non-int of type '%s'", a.TypeName())
	}
	if ai, bi, ok := bothInt(a, b); ok {
		return NewInt(ai * bi), nil
	}
	if af, bf, ok := bothFloat(a, b); ok {
		return NewFloat(af * bf), nil
	}
	return nil, unsupported("*", a, b)
}

// TrueDiv implements the / operator. The result is always a float.
func TrueDiv(a, b Object) (Object, error) {
	af, bf, ok := bothFloat(a, b)
	if !ok {
		return nil, unsupported("/", a, b)
	}
	if bf == 0 {
		if _, _, ints := bothInt(a, b); ints {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		return nil, Raise(ZeroDivisionError, "division by zero")
	}
	return NewFloat(af / bf), nil
}

func floorDivInt(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func floorModInt(a, b int64) int64 {
	r := a % b
	if r != 0 && (r < 0) != (b < 0) {
		r += b
	}
	return r
}

// FloorDiv implements the // operator with floor semantics.
func FloorDiv(a, b Object) (Object, error) {
	if ai, bi, ok := bothInt(a, b); ok {
		if bi == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		return NewInt(floorDivInt(ai, bi)), nil
	}
	if af, bf, ok := bothFloat(a, b); ok {
		if bf == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		return NewFloat(math.Floor(af / bf)), nil
	}
	return nil, unsupported("//", a, b)
}

// Mod implements the % operator with floor semantics.
func Mod(a, b Object) (Object, error) {
	if ai, bi, ok := bothInt(a, b); ok {
		if bi == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		return NewInt(floorModInt(ai, bi)), nil
	}
	if af, bf, ok := bothFloat(a, b); ok {
		if bf == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		r := math.Mod(af, bf)
		if r != 0 && (r < 0) != (bf < 0) {
			r += bf
		} else if r == 0 {
			r = math.Copysign(0, bf)
		}
		return NewFloat(r), nil
	}
	return nil, unsupported("%", a, b)
}

func ipow(base, exp int64) int64 {
	r := int64(1)
	for exp > 0 {
		if exp&1 == 1 {
			r *= base
		}
		base *= base
		exp >>= 1
	}
	return r
}

// Pow implements the ** operator. A negative exponent gives a float.
func Pow(a, b Object) (Object, error) {
	if ai, bi, ok := bothInt(a, b); ok {
		if bi >= 0 {
			return NewInt(ipow(ai, bi)), nil
		}
		if ai == 0 {
			return nil, Raise(ZeroDivisionError, "0.0 cannot be raised to a negative power")
		}
		return NewFloat(math.Pow(float64(ai), float64(bi))), nil
	}
	if af, bf, ok := bothFloat(a, b); ok {
		if af == 0 && bf < 0 {
			return nil, Raise(ZeroDivisionError, "0.0 cannot be raised to a negative power")
		}
		return NewFloat(math.Pow(af, bf)), nil
	}
	return nil, Raise(TypeError, "unsupported operand type(s) for ** or pow(): '%s' and '%s'",
		a.TypeName(), b.TypeName())
}

// CmpOp identifies a comparison operator.
type CmpOp int

const (
	OpEq CmpOp = iota
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
)

var cmpSym = map[CmpOp]string{
	OpEq: "==", OpNe: "!=", OpLt: "<", OpLe: "<=", OpGt: ">", OpGe: ">=",
}

// equals implements Python == without ever raising.
func equals(a, b Object) bool {
	if af, aok := AsFloat(a); aok {
		bf, bok := AsFloat(b)
		if !bok {
			return false
		}
		if ai, ok := AsInt(a); ok {
			if bi, ok2 := AsInt(b); ok2 {
				return ai == bi
			}
		}
		return af == bf
	}
	switch x := a.(type) {
	case *noneObject:
		_, ok := b.(*noneObject)
		return ok
	case *strObject:
		y, ok := b.(*strObject)
		return ok && x.v == y.v
	case *listObject:
		y, ok := b.(*listObject)
		return ok && seqEquals(x.elts, y.elts)
	case *tupleObject:
		y, ok := b.(*tupleObject)
		return ok && seqEquals(x.elts, y.elts)
	case *dictObject:
		y, ok := b.(*dictObject)
		return ok && dictEquals(x, y)
	case *rangeObject:
		y, ok := b.(*rangeObject)
		return ok && rangeEquals(x, y)
	}
	return a == b
}

func seqEquals(a, b []Object) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equals(a[i], b[i]) {
			return false
		}
	}
	return true
}

func rangeEquals(a, b *rangeObject) bool {
	la, lb := a.length(), b.length()
	if la != lb {
		return false
	}
	if la == 0 {
		return true
	}
	if a.start != b.start {
		return false
	}
	return la == 1 || a.step == b.step
}

func applyOrder(op CmpOp, c int) bool {
	switch op {
	case OpLt:
		return c < 0
	case OpLe:
		return c <= 0
	case OpGt:
		return c > 0
	default:
		return c >= 0
	}
}

func cmpI(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// order evaluates an ordering comparison, raising TypeError for
// incompatible operand types with the real operator symbol.
func order(op CmpOp, a, b Object) (bool, error) {
	if af, aok := AsFloat(a); aok {
		if bf, bok := AsFloat(b); bok {
			if ai, ok := AsInt(a); ok {
				if bi, ok2 := AsInt(b); ok2 {
					return applyOrder(op, cmpI(ai, bi)), nil
				}
			}
			return applyOrder(op, cmpF(af, bf)), nil
		}
	} else if x, ok := a.(*strObject); ok {
		if y, ok2 := b.(*strObject); ok2 {
			return applyOrder(op, strings.Compare(x.v, y.v)), nil
		}
	} else if x, ok := a.(*listObject); ok {
		if y, ok2 := b.(*listObject); ok2 {
			return seqOrder(op, x.elts, y.elts)
		}
	} else if x, ok := a.(*tupleObject); ok {
		if y, ok2 := b.(*tupleObject); ok2 {
			return seqOrder(op, x.elts, y.elts)
		}
	}
	return false, Raise(TypeError, "'%s' not supported between instances of '%s' and '%s'",
		cmpSym[op], a.TypeName(), b.TypeName())
}

func seqOrder(op CmpOp, a, b []Object) (bool, error) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if !equals(a[i], b[i]) {
			return order(op, a[i], b[i])
		}
	}
	return applyOrder(op, cmpI(int64(len(a)), int64(len(b)))), nil
}

// Compare implements the six rich comparison operators.
func Compare(op CmpOp, a, b Object) (Object, error) {
	switch op {
	case OpEq:
		return NewBool(equals(a, b)), nil
	case OpNe:
		return NewBool(!equals(a, b)), nil
	}
	r, err := order(op, a, b)
	if err != nil {
		return nil, err
	}
	return NewBool(r), nil
}

// Contains implements the `in` operator.
func Contains(container, item Object) (Object, error) {
	switch x := container.(type) {
	case *strObject:
		sub, ok := item.(*strObject)
		if !ok {
			return nil, Raise(TypeError, "'in <string>' requires string as left operand, not %s",
				item.TypeName())
		}
		return NewBool(strings.Contains(x.v, sub.v)), nil
	case *listObject:
		return seqContains(x.elts, item), nil
	case *tupleObject:
		return seqContains(x.elts, item), nil
	case *dictObject:
		k, err := dictKey(item)
		if err != nil {
			return nil, err
		}
		_, ok := x.index[k]
		return NewBool(ok), nil
	case *rangeObject:
		f, ok := AsFloat(item)
		if !ok || f != math.Trunc(f) {
			return False, nil
		}
		v := int64(f)
		if x.step > 0 {
			return NewBool(v >= x.start && v < x.stop && (v-x.start)%x.step == 0), nil
		}
		return NewBool(v <= x.start && v > x.stop && (v-x.start)%x.step == 0), nil
	case *dictKeysObject:
		return seqContains(x.d.keySlice(), item), nil
	case *dictValuesObject:
		return seqContains(x.d.valSlice(), item), nil
	case *dictItemsObject:
		return seqContains(x.d.itemSlice(), item), nil
	}
	return nil, Raise(TypeError, "argument of type '%s' is not iterable", container.TypeName())
}

func seqContains(elts []Object, item Object) Object {
	for _, e := range elts {
		if equals(e, item) {
			return True
		}
	}
	return False
}

// Is implements the `is` operator by object identity.
func Is(a, b Object) Object { return NewBool(a == b) }

// seqIndex normalizes a possibly negative index against length n.
func seqIndex(i int64, n int, msg string) (int, error) {
	if i < 0 {
		i += int64(n)
	}
	if i < 0 || i >= int64(n) {
		return 0, Raise(IndexError, "%s", msg)
	}
	return int(i), nil
}

// GetItem implements subscription: o[key].
func GetItem(o, key Object) (Object, error) {
	switch x := o.(type) {
	case *strObject:
		i, ok := AsInt(key)
		if !ok {
			return nil, Raise(TypeError, "string indices must be integers, not '%s'", key.TypeName())
		}
		runes := []rune(x.v)
		j, err := seqIndex(i, len(runes), "string index out of range")
		if err != nil {
			return nil, err
		}
		return NewStr(string(runes[j])), nil
	case *listObject:
		i, ok := AsInt(key)
		if !ok {
			return nil, Raise(TypeError, "list indices must be integers or slices, not '%s'", key.TypeName())
		}
		j, err := seqIndex(i, len(x.elts), "list index out of range")
		if err != nil {
			return nil, err
		}
		return x.elts[j], nil
	case *tupleObject:
		i, ok := AsInt(key)
		if !ok {
			return nil, Raise(TypeError, "tuple indices must be integers or slices, not '%s'", key.TypeName())
		}
		j, err := seqIndex(i, len(x.elts), "tuple index out of range")
		if err != nil {
			return nil, err
		}
		return x.elts[j], nil
	case *dictObject:
		return x.get(key)
	case *rangeObject:
		i, ok := AsInt(key)
		if !ok {
			return nil, Raise(TypeError, "range indices must be integers or slices, not '%s'", key.TypeName())
		}
		n := x.length()
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return nil, Raise(IndexError, "range object index out of range")
		}
		return NewInt(x.start + i*x.step), nil
	}
	return nil, Raise(TypeError, "'%s' object is not subscriptable", o.TypeName())
}

// SetItem implements assignment: o[key] = val.
func SetItem(o, key, val Object) error {
	switch x := o.(type) {
	case *listObject:
		i, ok := AsInt(key)
		if !ok {
			return Raise(TypeError, "list indices must be integers or slices, not '%s'", key.TypeName())
		}
		j, err := seqIndex(i, len(x.elts), "list assignment index out of range")
		if err != nil {
			return err
		}
		x.elts[j] = val
		return nil
	case *dictObject:
		return x.set(key, val)
	}
	return Raise(TypeError, "'%s' object does not support item assignment", o.TypeName())
}

// Len returns the length of a sized object.
func Len(o Object) (int, error) {
	switch x := o.(type) {
	case *strObject:
		return runeCount(x.v), nil
	case *listObject:
		return len(x.elts), nil
	case *tupleObject:
		return len(x.elts), nil
	case *dictObject:
		return len(x.entries), nil
	case *rangeObject:
		return int(x.length()), nil
	case *dictKeysObject:
		return len(x.d.entries), nil
	case *dictValuesObject:
		return len(x.d.entries), nil
	case *dictItemsObject:
		return len(x.d.entries), nil
	}
	return 0, Raise(TypeError, "object of type '%s' has no len()", o.TypeName())
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// Iterator walks an iterable. Next returns ok=false when exhausted.
type Iterator interface {
	Next() (Object, bool, error)
}

type sliceIter struct {
	elts []Object
	i    int
}

func (it *sliceIter) Next() (Object, bool, error) {
	if it.i >= len(it.elts) {
		return nil, false, nil
	}
	v := it.elts[it.i]
	it.i++
	return v, true, nil
}

type rangeIter struct {
	r *rangeObject
	i int64
	n int64
}

func (it *rangeIter) Next() (Object, bool, error) {
	if it.i >= it.n {
		return nil, false, nil
	}
	v := it.r.start + it.i*it.r.step
	it.i++
	return NewInt(v), true, nil
}

// Iter returns an iterator over an iterable object.
func Iter(o Object) (Iterator, error) {
	switch x := o.(type) {
	case *strObject:
		elts := make([]Object, 0, len(x.v))
		for _, r := range x.v {
			elts = append(elts, NewStr(string(r)))
		}
		return &sliceIter{elts: elts}, nil
	case *listObject:
		return &sliceIter{elts: x.elts}, nil
	case *tupleObject:
		return &sliceIter{elts: x.elts}, nil
	case *dictObject:
		return &sliceIter{elts: x.keySlice()}, nil
	case *rangeObject:
		return &rangeIter{r: x, n: x.length()}, nil
	case *dictKeysObject:
		return &sliceIter{elts: x.d.keySlice()}, nil
	case *dictValuesObject:
		return &sliceIter{elts: x.d.valSlice()}, nil
	case *dictItemsObject:
		return &sliceIter{elts: x.d.itemSlice()}, nil
	}
	return nil, Raise(TypeError, "'%s' object is not iterable", o.TypeName())
}

// Unpack destructures an iterable into exactly n values.
func Unpack(o Object, n int) ([]Object, error) {
	it, err := Iter(o)
	if err != nil {
		return nil, Raise(TypeError, "cannot unpack non-iterable %s object", o.TypeName())
	}
	out := make([]Object, 0, n)
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if len(out) == n {
			// CPython names the total when the source has a length, and
			// leaves it off for bare iterators.
			if total, lerr := Len(o); lerr == nil {
				return nil, Raise(ValueError, "too many values to unpack (expected %d, got %d)", n, total)
			}
			return nil, Raise(ValueError, "too many values to unpack (expected %d)", n)
		}
		out = append(out, v)
	}
	if len(out) < n {
		return nil, Raise(ValueError, "not enough values to unpack (expected %d, got %d)", n, len(out))
	}
	return out, nil
}
