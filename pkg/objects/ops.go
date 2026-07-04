package objects

import (
	"math"
	"math/big"
	"math/bits"
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
		// A spilled big int is never zero by the normalization invariant.
		return x.big != nil || x.v != 0
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
	case *setObject:
		return len(x.elts) > 0
	case *frozensetObject:
		return len(x.elts) > 0
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
		if i == math.MinInt64 {
			return NewIntFromBig(new(big.Int).Neg(big.NewInt(i))), nil
		}
		return NewInt(-i), nil
	}
	if b, ok := AsBigInt(o); ok {
		return NewIntFromBig(new(big.Int).Neg(b)), nil
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
	if x, ok := o.(*intObject); ok {
		return x, nil
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

// bothFloat resolves a mixed float operation's operands. It reports
// ok=false for a non-numeric pair and a non-nil error when a spilled
// int is too large for float64, which CPython raises as OverflowError.
func bothFloat(a, b Object) (float64, float64, bool, error) {
	af, aok, err := asFloatChecked(a)
	if !aok {
		return 0, 0, false, nil
	}
	bf, bok, berr := asFloatChecked(b)
	if !bok {
		return 0, 0, false, nil
	}
	if err == nil {
		err = berr
	}
	return af, bf, true, err
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
		if r, fits := addChk(ai, bi); fits {
			return NewInt(r), nil
		}
	}
	if x, y, ok := bothBig(a, b); ok {
		return NewIntFromBig(new(big.Int).Add(x, y)), nil
	}
	if af, bf, ok, err := bothFloat(a, b); ok {
		if err != nil {
			return nil, err
		}
		return NewFloat(af + bf), nil
	}
	return binFallback("+", a, b)
}

// Sub implements the - operator. On set operands it is set difference,
// with the result type following the left operand.
func Sub(a, b Object) (Object, error) {
	if ac, ok := asSetCore(a); ok {
		bc, ok2 := asSetCore(b)
		if !ok2 {
			return binFallback("-", a, b)
		}
		out, oc := newLike(a)
		diffInto(oc, ac, bc)
		return out, nil
	}
	if ai, bi, ok := bothInt(a, b); ok {
		if r, fits := subChk(ai, bi); fits {
			return NewInt(r), nil
		}
	}
	if x, y, ok := bothBig(a, b); ok {
		return NewIntFromBig(new(big.Int).Sub(x, y)), nil
	}
	if af, bf, ok, err := bothFloat(a, b); ok {
		if err != nil {
			return nil, err
		}
		return NewFloat(af - bf), nil
	}
	return binFallback("-", a, b)
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
		if IsBigInt(b) {
			return nil, Raise(OverflowError, "cannot fit 'int' into an index-sized integer")
		}
		return nil, Raise(TypeError, "can't multiply sequence by non-int of type '%s'", b.TypeName())
	}
	if isSequence(b) {
		if n, ok := AsInt(a); ok {
			return repeatSeq(b, n), nil
		}
		if IsBigInt(a) {
			return nil, Raise(OverflowError, "cannot fit 'int' into an index-sized integer")
		}
		return nil, Raise(TypeError, "can't multiply sequence by non-int of type '%s'", a.TypeName())
	}
	if ai, bi, ok := bothInt(a, b); ok {
		if r, fits := mulChk(ai, bi); fits {
			return NewInt(r), nil
		}
	}
	if x, y, ok := bothBig(a, b); ok {
		return NewIntFromBig(new(big.Int).Mul(x, y)), nil
	}
	if af, bf, ok, err := bothFloat(a, b); ok {
		if err != nil {
			return nil, err
		}
		return NewFloat(af * bf), nil
	}
	return binFallback("*", a, b)
}

// TrueDiv implements the / operator. The result is always a float.
// Two big ints divide exactly through big.Rat, matching CPython's
// correctly rounded int/int quotient past the float64 range.
func TrueDiv(a, b Object) (Object, error) {
	if ai, bi, ok := bothInt(a, b); ok && ai <= 1<<53 && ai >= -(1<<53) && bi <= 1<<53 && bi >= -(1<<53) {
		if bi == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		// Both operands convert exactly, so IEEE division is already
		// the correctly rounded quotient.
		return NewFloat(float64(ai) / float64(bi)), nil
	}
	if x, y, ok := bothBig(a, b); ok {
		if y.Sign() == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		f, _ := new(big.Rat).SetFrac(x, y).Float64()
		if math.IsInf(f, 0) {
			return nil, Raise(OverflowError, "integer division result too large for a float")
		}
		return NewFloat(f), nil
	}
	af, bf, ok, err := bothFloat(a, b)
	if !ok {
		return binFallback("/", a, b)
	}
	if err != nil {
		return nil, err
	}
	if bf == 0 {
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
		// The only overflowing quotient is MinInt64 // -1.
		if ai != math.MinInt64 || bi != -1 {
			return NewInt(floorDivInt(ai, bi)), nil
		}
	}
	if x, y, ok := bothBig(a, b); ok {
		if y.Sign() == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		q, _ := bigFloorDivMod(x, y)
		return NewIntFromBig(q), nil
	}
	if af, bf, ok, err := bothFloat(a, b); ok {
		if err != nil {
			return nil, err
		}
		if bf == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		return NewFloat(math.Floor(af / bf)), nil
	}
	return binFallback("//", a, b)
}

// Mod implements the % operator with floor semantics. A str left
// operand means percent formatting instead.
func Mod(a, b Object) (Object, error) {
	if s, ok := a.(*strObject); ok {
		return percentFormat(s.v, b)
	}
	if ai, bi, ok := bothInt(a, b); ok {
		if bi == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		return NewInt(floorModInt(ai, bi)), nil
	}
	if x, y, ok := bothBig(a, b); ok {
		if y.Sign() == 0 {
			return nil, Raise(ZeroDivisionError, "division by zero")
		}
		_, r := bigFloorDivMod(x, y)
		return NewIntFromBig(r), nil
	}
	if af, bf, ok, err := bothFloat(a, b); ok {
		if err != nil {
			return nil, err
		}
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
	return binFallback("%", a, b)
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

// Pow implements the ** operator. A negative exponent gives a float,
// and an int result past int64 spills to big. Probed 3.14 wordings:
// 0 ** -1 and 0.0 ** -1 both say "zero to a negative power", and a
// float result past the double range is errno-flavored OverflowError.
func Pow(a, b Object) (Object, error) {
	if isIntish(a) && isIntish(b) {
		x, _ := AsBigInt(a)
		y, _ := AsBigInt(b)
		if y.Sign() >= 0 {
			if ai, bi, ok := bothInt(a, b); ok && smallPowFits(ai, bi) {
				return NewInt(ipow(ai, bi)), nil
			}
			return NewIntFromBig(new(big.Int).Exp(x, y, nil)), nil
		}
		if x.Sign() == 0 {
			return nil, Raise(ZeroDivisionError, "zero to a negative power")
		}
	}
	if af, bf, ok, err := bothFloat(a, b); ok {
		if err != nil {
			return nil, err
		}
		if af == 0 && bf < 0 {
			return nil, Raise(ZeroDivisionError, "zero to a negative power")
		}
		r := math.Pow(af, bf)
		if math.IsInf(r, 0) && !math.IsInf(af, 0) && !math.IsInf(bf, 0) {
			return nil, Raise(OverflowError, "(34, 'Result too large')")
		}
		return NewFloat(r), nil
	}
	if res, ok, err := binaryDunder("__pow__", "__rpow__", a, b); ok || err != nil {
		return res, err
	}
	return nil, Raise(TypeError, "unsupported operand type(s) for ** or pow(): '%s' and '%s'",
		a.TypeName(), b.TypeName())
}

// BitOr implements the | operator: int bitwise or, set union. Probed on
// 3.14: True | False is bool True, but mixing bool with int gives int.
func BitOr(a, b Object) (Object, error) {
	if ac, ok := asSetCore(a); ok {
		bc, ok2 := asSetCore(b)
		if !ok2 {
			return binFallback("|", a, b)
		}
		out, oc := newLike(a)
		unionInto(oc, ac, bc)
		return out, nil
	}
	if x, ok := a.(*boolObject); ok {
		if y, ok2 := b.(*boolObject); ok2 {
			return NewBool(x.v || y.v), nil
		}
	}
	if ai, bi, ok := bothInt(a, b); ok {
		return NewInt(ai | bi), nil
	}
	if x, y, ok := bothBig(a, b); ok {
		return NewIntFromBig(new(big.Int).Or(x, y)), nil
	}
	return binFallback("|", a, b)
}

// BitXor implements the ^ operator: int bitwise xor, set symmetric
// difference. bool ^ bool stays bool like |.
func BitXor(a, b Object) (Object, error) {
	if ac, ok := asSetCore(a); ok {
		bc, ok2 := asSetCore(b)
		if !ok2 {
			return binFallback("^", a, b)
		}
		out, oc := newLike(a)
		symDiffInto(oc, ac, bc)
		return out, nil
	}
	if x, ok := a.(*boolObject); ok {
		if y, ok2 := b.(*boolObject); ok2 {
			return NewBool(x.v != y.v), nil
		}
	}
	if ai, bi, ok := bothInt(a, b); ok {
		return NewInt(ai ^ bi), nil
	}
	if x, y, ok := bothBig(a, b); ok {
		return NewIntFromBig(new(big.Int).Xor(x, y)), nil
	}
	return binFallback("^", a, b)
}

// BitAnd implements the & operator: int bitwise and, set intersection.
// bool & bool stays bool like |.
func BitAnd(a, b Object) (Object, error) {
	if ac, ok := asSetCore(a); ok {
		bc, ok2 := asSetCore(b)
		if !ok2 {
			return binFallback("&", a, b)
		}
		out, oc := newLike(a)
		intersectInto(oc, ac, bc)
		return out, nil
	}
	if x, ok := a.(*boolObject); ok {
		if y, ok2 := b.(*boolObject); ok2 {
			return NewBool(x.v && y.v), nil
		}
	}
	if ai, bi, ok := bothInt(a, b); ok {
		return NewInt(ai & bi), nil
	}
	if x, y, ok := bothBig(a, b); ok {
		return NewIntFromBig(new(big.Int).And(x, y)), nil
	}
	return binFallback("&", a, b)
}

// LShift implements the << operator. Probed: True << True is int 2, so
// shifts never keep bool. Overflowing bits promote to big; a shift
// count past int64 raises unless the value is zero, in CPython's
// "too many digits in integer" wording.
func LShift(a, b Object) (Object, error) {
	if !isIntish(a) || !isIntish(b) {
		return binFallback("<<", a, b)
	}
	x, _ := AsBigInt(a)
	if IsBigInt(b) {
		y, _ := AsBigInt(b)
		if y.Sign() < 0 {
			return nil, Raise(ValueError, "negative shift count")
		}
		if x.Sign() == 0 {
			return NewInt(0), nil
		}
		return nil, Raise(OverflowError, "too many digits in integer")
	}
	bi, _ := AsInt(b)
	if bi < 0 {
		return nil, Raise(ValueError, "negative shift count")
	}
	if ai, ok := AsInt(a); ok && bi < 63 && int64(bits.Len64(magnitude(ai)))+bi <= 62 {
		return NewInt(ai << uint(bi)), nil
	}
	return NewIntFromBig(new(big.Int).Lsh(x, uint(bi))), nil
}

func magnitude(v int64) uint64 {
	if v < 0 {
		return uint64(-(v + 1)) + 1
	}
	return uint64(v)
}

// RShift implements the >> operator with arithmetic (sign-filling) shift,
// which matches Python's floor semantics for negative ints. A shift
// count past int64 leaves only the sign: 0 or -1.
func RShift(a, b Object) (Object, error) {
	if !isIntish(a) || !isIntish(b) {
		return binFallback(">>", a, b)
	}
	if IsBigInt(b) {
		y, _ := AsBigInt(b)
		if y.Sign() < 0 {
			return nil, Raise(ValueError, "negative shift count")
		}
		x, _ := AsBigInt(a)
		if x.Sign() < 0 {
			return NewInt(-1), nil
		}
		return NewInt(0), nil
	}
	bi, _ := AsInt(b)
	if bi < 0 {
		return nil, Raise(ValueError, "negative shift count")
	}
	if ai, ok := AsInt(a); ok {
		if bi >= 64 {
			bi = 63
		}
		return NewInt(ai >> uint(bi)), nil
	}
	x, _ := AsBigInt(a)
	if bi > int64(x.BitLen())+1 {
		bi = int64(x.BitLen()) + 1
	}
	return NewIntFromBig(new(big.Int).Rsh(x, uint(bi))), nil
}

// Invert implements unary ~. Probed: ~True is int -2, ~ on bool never
// stays bool.
func Invert(o Object) (Object, error) {
	if i, ok := AsInt(o); ok {
		return NewInt(-i - 1), nil
	}
	if b, ok := AsBigInt(o); ok {
		return NewIntFromBig(new(big.Int).Not(b)), nil
	}
	return nil, Raise(TypeError, "bad operand type for unary ~: '%s'", o.TypeName())
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
	if c, unordered, ok := numCmp(a, b); ok {
		return !unordered && c == 0
	}
	// One side numeric, the other not: unequal, never an error.
	if isNumeric(a) != isNumeric(b) {
		return false
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
	case *setObject:
		// A set equals a frozenset with the same elements; probed.
		return coreEquals(&x.setCore, b)
	case *frozensetObject:
		return coreEquals(&x.setCore, b)
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
	if c, unordered, ok := numCmp(a, b); ok {
		// A NaN operand loses every ordering, like CPython.
		return !unordered && applyOrder(op, c), nil
	}
	if x, ok := a.(*strObject); ok {
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
	} else if x, ok := asSetCore(a); ok {
		// Sets order by subset relation, mixing set and frozenset freely.
		if y, ok2 := asSetCore(b); ok2 {
			return setOrder(op, x, y), nil
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
	case *setObject:
		return setContains(&x.setCore, item)
	case *frozensetObject:
		return setContains(&x.setCore, item)
	case *rangeObject:
		if IsBigInt(item) {
			// A spilled int can never land in an int64-backed range.
			return False, nil
		}
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

// errIndexFit is the probed 3.14 wording for an index past ssize_t:
// subscripts raise it as IndexError, repeat counts as OverflowError.
func errIndexFit() error {
	return Raise(IndexError, "cannot fit 'int' into an index-sized integer")
}

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
			if IsBigInt(key) {
				return nil, errIndexFit()
			}
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
			if IsBigInt(key) {
				return nil, errIndexFit()
			}
			// Probed on 3.14: [1][None] -> TypeError: list indices must be
			// integers or slices, not NoneType. List, tuple and range spell
			// the type bare; only the string message quotes it.
			return nil, Raise(TypeError, "list indices must be integers or slices, not %s", key.TypeName())
		}
		j, err := seqIndex(i, len(x.elts), "list index out of range")
		if err != nil {
			return nil, err
		}
		return x.elts[j], nil
	case *tupleObject:
		i, ok := AsInt(key)
		if !ok {
			if IsBigInt(key) {
				return nil, errIndexFit()
			}
			return nil, Raise(TypeError, "tuple indices must be integers or slices, not %s", key.TypeName())
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
			if IsBigInt(key) {
				return nil, errIndexFit()
			}
			return nil, Raise(TypeError, "range indices must be integers or slices, not %s", key.TypeName())
		}
		n := x.length()
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return nil, Raise(IndexError, "range object index out of range")
		}
		return NewInt(x.start + i*x.step), nil
	case *classObject:
		return classSubscript(x, key)
	}
	return nil, Raise(TypeError, "'%s' object is not subscriptable", o.TypeName())
}

// SetItem implements assignment: o[key] = val.
func SetItem(o, key, val Object) error {
	switch x := o.(type) {
	case *listObject:
		i, ok := AsInt(key)
		if !ok {
			if IsBigInt(key) {
				return errIndexFit()
			}
			// Probed on 3.14: xs[None] = 1 spells the type bare too.
			return Raise(TypeError, "list indices must be integers or slices, not %s", key.TypeName())
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
	case *setObject:
		return len(x.elts), nil
	case *frozensetObject:
		return len(x.elts), nil
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
	case *setObject:
		return &sliceIter{elts: x.elts}, nil
	case *frozensetObject:
		return &sliceIter{elts: x.elts}, nil
	case *rangeObject:
		return &rangeIter{r: x, n: x.length()}, nil
	case *dictKeysObject:
		return &sliceIter{elts: x.d.keySlice()}, nil
	case *dictValuesObject:
		return &sliceIter{elts: x.d.valSlice()}, nil
	case *dictItemsObject:
		return &sliceIter{elts: x.d.itemSlice()}, nil
	case Iterable:
		return x.Iterate()
	}
	return nil, Raise(TypeError, "'%s' object is not iterable", o.TypeName())
}

// Iterable lets object types defined outside this package, like the runtime's
// enumerate and zip objects, plug into Iter.
type Iterable interface {
	Object
	Iterate() (Iterator, error)
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

// UnpackEx destructures an iterable around a starred target: before
// fixed leading values, one list soaking up the middle, then after
// fixed trailing values. The result has before+1+after entries with
// the list at index before.
func UnpackEx(o Object, before, after int) ([]Object, error) {
	it, err := Iter(o)
	if err != nil {
		// Probed on 3.14: a, *b = 1 gives the same text as the plain form.
		return nil, Raise(TypeError, "cannot unpack non-iterable %s object", o.TypeName())
	}
	var vals []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		vals = append(vals, v)
	}
	need := before + after
	if len(vals) < need {
		// Probed on 3.14: a, b, *c = [1] -> ValueError: not enough values
		// to unpack (expected at least 2, got 1).
		return nil, Raise(ValueError, "not enough values to unpack (expected at least %d, got %d)",
			need, len(vals))
	}
	mid := len(vals) - after
	out := make([]Object, 0, need+1)
	out = append(out, vals[:before]...)
	out = append(out, NewList(append([]Object(nil), vals[before:mid]...)))
	return append(out, vals[mid:]...), nil
}
