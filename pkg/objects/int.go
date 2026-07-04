package objects

import (
	"math"
	"math/big"
	"math/bits"
	"strconv"
)

// The arbitrary precision integer core. An intObject keeps small values
// in an int64 and spills to a big.Int only when the value does not fit;
// the big field is non-nil exactly when the value is outside the int64
// range, and every constructor and arithmetic result restores that
// invariant, so two equal ints always share one representation.

// NewIntFromBig boxes a big.Int, normalizing back to the small form when
// the value fits. It takes ownership of b.
func NewIntFromBig(b *big.Int) Object {
	if b.IsInt64() {
		return NewInt(b.Int64())
	}
	return &intObject{big: b}
}

// NewIntText boxes a decimal literal that may exceed int64. The lexer
// normalizes every integer literal to plain decimal, so a parse failure
// here is a compiler bug, not user input.
func NewIntText(s string) Object {
	b, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("objects.NewIntText: bad decimal literal " + s)
	}
	return NewIntFromBig(b)
}

// AsBigInt extracts any int-ish value (int or bool) as a big.Int. The
// result must be treated as read-only: for spilled ints it aliases the
// object's own storage.
func AsBigInt(o Object) (*big.Int, bool) {
	switch x := o.(type) {
	case *intObject:
		if x.big != nil {
			return x.big, true
		}
		return big.NewInt(x.v), true
	case *boolObject:
		if x.v {
			return big.NewInt(1), true
		}
		return big.NewInt(0), true
	}
	return nil, false
}

// IsBigInt reports whether o is an int too large for int64. Index and
// repeat-count consumers use this to pick the "cannot fit" wordings.
func IsBigInt(o Object) bool {
	x, ok := o.(*intObject)
	return ok && x.big != nil
}

func isIntish(o Object) bool {
	switch o.(type) {
	case *intObject, *boolObject:
		return true
	}
	return false
}

func isNumeric(o Object) bool {
	switch o.(type) {
	case *intObject, *boolObject, *floatObject:
		return true
	}
	return false
}

// bothBig converts an int-ish pair to big form. Callers try the small
// path first, so in practice at least one side has spilled.
func bothBig(a, b Object) (*big.Int, *big.Int, bool) {
	x, ok := AsBigInt(a)
	if !ok {
		return nil, nil, false
	}
	y, ok := AsBigInt(b)
	if !ok {
		return nil, nil, false
	}
	return x, y, true
}

// Checked int64 arithmetic. A false result means the operation
// overflowed and the caller promotes to big.Int.

func addChk(a, b int64) (int64, bool) {
	r := a + b
	return r, (a^r)&(b^r) >= 0
}

func subChk(a, b int64) (int64, bool) {
	r := a - b
	return r, (a^b)&(a^r) >= 0
}

func mulChk(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	r := a * b
	if a == math.MinInt64 || b == math.MinInt64 {
		return r, a == 1 || b == 1
	}
	return r, r/b == a
}

// asFloatChecked converts a numeric operand for a mixed float operation.
// A spilled int outside the float64 range raises the OverflowError the
// int-to-float conversion raises in CPython.
func asFloatChecked(o Object) (float64, bool, error) {
	if x, ok := o.(*intObject); ok && x.big != nil {
		f, _ := new(big.Float).SetInt(x.big).Float64()
		if math.IsInf(f, 0) {
			return 0, true, Raise(OverflowError, "int too large to convert to float")
		}
		return f, true, nil
	}
	f, ok := AsFloat(o)
	return f, ok, nil
}

// bigFloorDivMod applies Python floor semantics on big operands: the
// remainder takes the divisor's sign, same as the int64 helpers.
func bigFloorDivMod(x, y *big.Int) (*big.Int, *big.Int) {
	q, r := new(big.Int).QuoRem(x, y, new(big.Int))
	if r.Sign() != 0 && (r.Sign() < 0) != (y.Sign() < 0) {
		q.Sub(q, big.NewInt(1))
		r.Add(r, y)
	}
	return q, r
}

// intCmp compares two int-ish values of any size.
func intCmp(a, b Object) int {
	if ai, ok := AsInt(a); ok {
		if bi, ok2 := AsInt(b); ok2 {
			return cmpI(ai, bi)
		}
	}
	x, _ := AsBigInt(a)
	y, _ := AsBigInt(b)
	return x.Cmp(y)
}

// numCmp compares two numbers exactly, never rounding an int through
// float64: 2**53 + 1 > 2.0**53 holds just as it does in CPython.
// unordered reports a NaN operand; ok is false for non-numeric pairs.
func numCmp(a, b Object) (c int, unordered, ok bool) {
	aInt, bInt := isIntish(a), isIntish(b)
	if aInt && bInt {
		return intCmp(a, b), false, true
	}
	af, aFloat := a.(*floatObject)
	bf, bFloat := b.(*floatObject)
	switch {
	case aFloat && bFloat:
		if math.IsNaN(af.v) || math.IsNaN(bf.v) {
			return 0, true, true
		}
		return cmpF(af.v, bf.v), false, true
	case aFloat && bInt:
		c, unordered = floatIntCmp(af.v, b)
		return c, unordered, true
	case bFloat && aInt:
		c, unordered = floatIntCmp(bf.v, a)
		return -c, unordered, true
	}
	return 0, false, false
}

// floatIntCmp orders a float against an int of any size, exactly.
func floatIntCmp(f float64, i Object) (int, bool) {
	if math.IsNaN(f) {
		return 0, true
	}
	if math.IsInf(f, 1) {
		return 1, false
	}
	if math.IsInf(f, -1) {
		return -1, false
	}
	if v, ok := AsInt(i); ok {
		if v >= -(1<<53) && v <= 1<<53 {
			return cmpF(f, float64(v)), false
		}
	}
	b, _ := AsBigInt(i)
	fr := new(big.Rat).SetFloat64(f)
	return fr.Cmp(new(big.Rat).SetInt(b)), false
}

// smallPowFits reports whether base**exp stays comfortably inside int64,
// so the repeated-squaring fast path cannot overflow.
func smallPowFits(base, exp int64) bool {
	if exp < 0 || exp > 62 {
		return false
	}
	mag := base
	if mag < 0 {
		if mag == math.MinInt64 {
			return false
		}
		mag = -mag
	}
	if mag <= 1 {
		return true
	}
	return int64(bits.Len64(uint64(mag)))*exp <= 62
}

// maxStrDigits mirrors sys.int_max_str_digits: decimal conversion in
// either direction refuses past 4300 digits.
const maxStrDigits = 4300

// maxStrDigitsBits bounds the bit length of any int with at most 4300
// decimal digits; values past it fail the limit without rendering.
const maxStrDigitsBits = 14284

// intDecimal renders an int in decimal, enforcing the conversion limit
// with CPython's int-to-str wording.
func intDecimal(x *intObject) (string, error) {
	if x.big == nil {
		return strconv.FormatInt(x.v, 10), nil
	}
	if x.big.BitLen() > maxStrDigitsBits {
		return "", errStrDigits()
	}
	s := x.big.String()
	digits := len(s)
	if x.big.Sign() < 0 {
		digits--
	}
	if digits > maxStrDigits {
		return "", errStrDigits()
	}
	return s, nil
}

func errStrDigits() error {
	return Raise(ValueError,
		"Exceeds the limit (%d digits) for integer string conversion; use sys.set_int_max_str_digits() to increase the limit",
		maxStrDigits)
}

// intDecimalLoose renders without the limit, for the infallible Repr
// path that error formatting and debugging use.
func intDecimalLoose(x *intObject) string {
	if x.big == nil {
		return strconv.FormatInt(x.v, 10)
	}
	return x.big.String()
}
