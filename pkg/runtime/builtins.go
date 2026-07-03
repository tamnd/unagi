package runtime

import (
	"math"
	"math/big"
	"math/bits"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// asIndex extracts an int the way CPython __index__ consumers do,
// accepting int and bool and rejecting everything else with the probed
// "cannot be interpreted as an integer" message.
func asIndex(o objects.Object) (int64, error) {
	if i, ok := objects.AsInt(o); ok {
		return i, nil
	}
	return 0, objects.Raise(objects.TypeError,
		"'%s' object cannot be interpreted as an integer", o.TypeName())
}

// Min implements min(iterable) and min(a, b, ...).
func Min(args []objects.Object) (objects.Object, error) {
	return minMax(args, "min", objects.OpLt)
}

// Max implements max(iterable) and max(a, b, ...).
func Max(args []objects.Object) (objects.Object, error) {
	return minMax(args, "max", objects.OpGt)
}

// minMax keeps the first winner: a candidate only displaces the current
// best when it compares strictly less (or greater), so min(True, 1) is
// True and min(1, True) is 1, exactly as probed on 3.14.
func minMax(args []objects.Object, name string, op objects.CmpOp) (objects.Object, error) {
	if len(args) == 0 {
		// Probed: min() -> TypeError: min expected at least 1 argument, got 0.
		return nil, objects.Raise(objects.TypeError, "%s expected at least 1 argument, got 0", name)
	}
	items := args
	if len(args) == 1 {
		var err error
		items, err = materialize(args[0])
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			// Probed 3.14 wording: min() iterable argument is empty.
			return nil, objects.Raise(objects.ValueError, "%s() iterable argument is empty", name)
		}
	}
	best := items[0]
	for _, it := range items[1:] {
		r, err := objects.Compare(op, it, best)
		if err != nil {
			return nil, err
		}
		if objects.Truth(r) {
			best = it
		}
	}
	return best, nil
}

// Sum implements sum(iterable) and sum(iterable, start). A str start is
// refused up front like CPython; list and tuple starts work because
// Add handles their concatenation.
func Sum(args []objects.Object) (objects.Object, error) {
	switch len(args) {
	case 0:
		return nil, objects.Raise(objects.TypeError, "sum() takes at least 1 positional argument (0 given)")
	case 1, 2:
	default:
		return nil, objects.Raise(objects.TypeError, "sum() takes at most 2 arguments (%d given)", len(args))
	}
	acc := objects.NewInt(0)
	if len(args) == 2 {
		if _, ok := objects.AsStr(args[1]); ok {
			// Probed: sum() can't sum strings [use ''.join(seq) instead],
			// raised even when the elements are not strings.
			return nil, objects.Raise(objects.TypeError, "sum() can't sum strings [use ''.join(seq) instead]")
		}
		acc = args[1]
	}
	items, err := materialize(args[0])
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		acc, err = objects.Add(acc, it)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

// Round implements round(x) and round(x, ndigits) with CPython's
// round-half-even everywhere.
func Round(args []objects.Object) (objects.Object, error) {
	switch len(args) {
	case 0:
		return nil, objects.Raise(objects.TypeError, "round() missing required argument 'number' (pos 1)")
	case 1, 2:
	default:
		return nil, objects.Raise(objects.TypeError, "round() takes at most 2 arguments (%d given)", len(args))
	}
	x := args[0]
	if i, ok := objects.AsInt(x); ok {
		if len(args) == 1 {
			// round(True) is 1, an int, so always rebox.
			return objects.NewInt(i), nil
		}
		nd, err := asIndex(args[1])
		if err != nil {
			return nil, err
		}
		if nd >= 0 {
			return objects.NewInt(i), nil
		}
		return objects.NewInt(roundIntNeg(i, -nd)), nil
	}
	if f, ok := objects.AsFloat(x); ok {
		if len(args) == 1 {
			if math.IsInf(f, 0) {
				return nil, objects.Raise(objects.OverflowError, "cannot convert float infinity to integer")
			}
			if math.IsNaN(f) {
				return nil, objects.Raise(objects.ValueError, "cannot convert float NaN to integer")
			}
			// The int64 conversion shares IntOf's overflow stance: values
			// beyond int64 (round(1e308)) are a known limitation.
			return objects.NewInt(int64(math.RoundToEven(f))), nil
		}
		nd, err := asIndex(args[1])
		if err != nil {
			return nil, err
		}
		return roundFloat(f, nd)
	}
	// Probed: round("a") -> TypeError: type str doesn't define __round__ method.
	return nil, objects.Raise(objects.TypeError, "type %s doesn't define __round__ method", x.TypeName())
}

// roundIntNeg rounds an int to a multiple of 10**digits, half to even,
// via floor division so ties land like round(150, -2) == round(250, -2)
// == 200 and round(-25, -1) == -20, all probed.
func roundIntNeg(i, digits int64) int64 {
	if digits > 18 {
		// 10**19 overflows int64. Anything below 5e18 rounds to 0 here;
		// larger magnitudes would need a bigint, a known limitation.
		return 0
	}
	p := int64(1)
	for k := int64(0); k < digits; k++ {
		p *= 10
	}
	q := i / p
	if i%p != 0 && i < 0 {
		q--
	}
	r := i - q*p
	switch {
	case 2*r > p:
		q++
	case 2*r == p && q&1 != 0:
		q++
	}
	return q * p
}

// roundFloat rounds a float to nd decimal digits through exact decimal
// arithmetic on big.Rat, not math.Round, so round(2.675, 2) == 2.67
// exactly as CPython's dtoa-based rounding gives.
func roundFloat(f float64, nd int64) (objects.Object, error) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		// Probed: round(inf, 2) is inf, round(nan, 2) is nan.
		return objects.NewFloat(f), nil
	}
	// A float64 has at most 1074 fractional decimal digits, and 10**309
	// exceeds twice the largest float, so extreme nd values short-cut.
	if nd > 1100 {
		return objects.NewFloat(f), nil
	}
	if nd < -400 {
		return objects.NewFloat(math.Copysign(0, f)), nil
	}
	r := new(big.Rat).SetFloat64(f)
	digits := nd
	if digits < 0 {
		digits = -digits
	}
	scale := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(digits), nil))
	if nd >= 0 {
		r.Mul(r, scale)
	} else {
		r.Quo(r, scale)
	}
	// Denominators are positive, so DivMod is floor division; then ties
	// go to the even quotient.
	q, rem := new(big.Int).DivMod(r.Num(), r.Denom(), new(big.Int))
	rem.Lsh(rem, 1)
	switch rem.Cmp(r.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	if q.Sign() == 0 {
		// Keep the input's sign on zero: round(-0.5, 0) is -0.0.
		return objects.NewFloat(math.Copysign(0, f)), nil
	}
	out := new(big.Rat).SetInt(q)
	if nd >= 0 {
		out.Quo(out, scale)
	} else {
		out.Mul(out, scale)
	}
	v, _ := out.Float64()
	if math.IsInf(v, 0) {
		// Probed: round(1.7976931348623157e308, -308) overflows.
		return nil, objects.Raise(objects.OverflowError, "rounded value too large to represent")
	}
	return objects.NewFloat(v), nil
}

// DivMod implements divmod(a, b) as a (quotient, remainder) tuple with
// the same floor semantics as the // and % operators.
func DivMod(a, b objects.Object) (objects.Object, error) {
	if _, ok := objects.AsFloat(a); !ok {
		return nil, divmodUnsupported(a, b)
	}
	if _, ok := objects.AsFloat(b); !ok {
		return nil, divmodUnsupported(a, b)
	}
	q, err := objects.FloorDiv(a, b)
	if err != nil {
		return nil, err
	}
	r, err := objects.Mod(a, b)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{q, r}), nil
}

func divmodUnsupported(a, b objects.Object) error {
	return objects.Raise(objects.TypeError,
		"unsupported operand type(s) for divmod(): '%s' and '%s'", a.TypeName(), b.TypeName())
}

// Pow3 implements three-argument pow, integers only. A negative exponent
// takes the modular inverse route pow supports since 3.8.
func Pow3(base, exp, mod objects.Object) (objects.Object, error) {
	bi, bok := objects.AsInt(base)
	ei, eok := objects.AsInt(exp)
	mi, mok := objects.AsInt(mod)
	if !bok || !eok || !mok {
		// Probed: a float anywhere gives the integers-only message, while
		// types with no pow slot at all list all three type names.
		if base.TypeName() == "float" || exp.TypeName() == "float" || mod.TypeName() == "float" {
			return nil, objects.Raise(objects.TypeError,
				"pow() 3rd argument not allowed unless all arguments are integers")
		}
		return nil, objects.Raise(objects.TypeError,
			"unsupported operand type(s) for ** or pow(): '%s', '%s', '%s'",
			base.TypeName(), exp.TypeName(), mod.TypeName())
	}
	if mi == 0 {
		return nil, objects.Raise(objects.ValueError, "pow() 3rd argument cannot be 0")
	}
	am := mi
	if am < 0 {
		am = -am
	}
	b := floorModInt64(bi, am)
	if ei < 0 {
		g, x := extGCD(b, am)
		if g != 1 {
			return nil, objects.Raise(objects.ValueError, "base is not invertible for the given modulus")
		}
		b = floorModInt64(x, am)
		ei = -ei
	}
	r := powMod(b, ei, am)
	// The result carries the sign of the modulus, floor style, so
	// pow(2, 3, -5) is -2.
	if mi < 0 && r != 0 {
		r -= am
	}
	return objects.NewInt(r), nil
}

func floorModInt64(a, m int64) int64 {
	r := a % m
	if r < 0 {
		r += m
	}
	return r
}

// extGCD returns gcd(a, m) and x with a*x congruent to the gcd mod m.
func extGCD(a, m int64) (g, x int64) {
	oldR, r := a, m
	oldS, s := int64(1), int64(0)
	for r != 0 {
		q := oldR / r
		oldR, r = r, oldR-q*r
		oldS, s = s, oldS-q*s
	}
	return oldR, oldS
}

// powMod computes b**e mod m for m >= 1 with a 128-bit intermediate
// product, so no modulus in int64 range overflows.
func powMod(b, e, m int64) int64 {
	if m == 1 {
		return 0
	}
	um := uint64(m)
	r := uint64(1)
	ub := uint64(b)
	for e > 0 {
		if e&1 == 1 {
			r = mulMod(r, ub, um)
		}
		ub = mulMod(ub, ub, um)
		e >>= 1
	}
	return int64(r)
}

func mulMod(a, b, m uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	return bits.Rem64(hi, lo, m)
}

// Bin implements bin(o).
func Bin(o objects.Object) (objects.Object, error) { return baseRepr(o, 2, "0b") }

// Oct implements oct(o).
func Oct(o objects.Object) (objects.Object, error) { return baseRepr(o, 8, "0o") }

// Hex implements hex(o).
func Hex(o objects.Object) (objects.Object, error) { return baseRepr(o, 16, "0x") }

// baseRepr formats an int in a base with the prefix after the sign, so
// bin(-5) is "-0b101". Bools count as ints: bin(True) is "0b1".
func baseRepr(o objects.Object, base int, prefix string) (objects.Object, error) {
	i, err := asIndex(o)
	if err != nil {
		return nil, err
	}
	s := strconv.FormatInt(i, base)
	if strings.HasPrefix(s, "-") {
		return objects.NewStr("-" + prefix + s[1:]), nil
	}
	return objects.NewStr(prefix + s), nil
}

// Ord implements ord(o) for a one-character str, by code point.
func Ord(o objects.Object) (objects.Object, error) {
	s, ok := objects.AsStr(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"ord() expected string of length 1, but %s found", o.TypeName())
	}
	runes := []rune(s)
	if len(runes) != 1 {
		return nil, objects.Raise(objects.TypeError,
			"ord() expected a character, but string of length %d found", len(runes))
	}
	return objects.NewInt(int64(runes[0])), nil
}

// Chr implements chr(o) for 0..0x10FFFF. CPython also allows lone
// surrogates, but a Go string cannot hold one as valid UTF-8, so those
// are refused honestly instead of silently encoding U+FFFD.
func Chr(o objects.Object) (objects.Object, error) {
	i, err := asIndex(o)
	if err != nil {
		return nil, err
	}
	if i < 0 || i > 0x10FFFF {
		return nil, objects.Raise(objects.ValueError, "chr() arg not in range(0x110000)")
	}
	if i >= 0xD800 && i <= 0xDFFF {
		return nil, objects.Raise(objects.ValueError,
			"chr() arg is a surrogate code point, not representable in this runtime")
	}
	return objects.NewStr(string(rune(i))), nil
}

// Sorted implements sorted(o) with no key or reverse: a stable
// ascending sort into a new list. Compare raises for unorderable pairs
// and the first such error wins.
func Sorted(o objects.Object) (objects.Object, error) {
	items, err := materialize(o)
	if err != nil {
		return nil, err
	}
	var sortErr error
	sort.SliceStable(items, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		r, err := objects.Compare(objects.OpLt, items[i], items[j])
		if err != nil {
			sortErr = err
			return false
		}
		return objects.Truth(r)
	})
	if sortErr != nil {
		return nil, sortErr
	}
	return objects.NewList(items), nil
}

// ListOf implements list() and list(iterable).
func ListOf(args []objects.Object) (objects.Object, error) {
	elts, err := ctorElts("list", args)
	if err != nil {
		return nil, err
	}
	return objects.NewList(elts), nil
}

// TupleOf implements tuple() and tuple(iterable).
func TupleOf(args []objects.Object) (objects.Object, error) {
	elts, err := ctorElts("tuple", args)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple(elts), nil
}

// SetOf implements set() and set(iterable).
func SetOf(args []objects.Object) (objects.Object, error) {
	elts, err := ctorElts("set", args)
	if err != nil {
		return nil, err
	}
	return objects.NewSet(elts)
}

// FrozensetOf implements frozenset() and frozenset(iterable).
func FrozensetOf(args []objects.Object) (objects.Object, error) {
	elts, err := ctorElts("frozenset", args)
	if err != nil {
		return nil, err
	}
	return objects.NewFrozenset(elts)
}

// ctorElts is the shared zero-or-one-iterable argument handling for the
// container constructors.
func ctorElts(name string, args []objects.Object) ([]objects.Object, error) {
	switch len(args) {
	case 0:
		return nil, nil
	case 1:
		return materialize(args[0])
	}
	return nil, objects.Raise(objects.TypeError,
		"%s expected at most 1 argument, got %d", name, len(args))
}

// DictOf implements dict(), dict(mapping) and dict(iterable-of-pairs),
// without keyword arguments.
func DictOf(args []objects.Object) (objects.Object, error) {
	switch len(args) {
	case 0:
		return objects.NewDict(nil, nil)
	case 1:
	default:
		return nil, objects.Raise(objects.TypeError,
			"dict expected at most 1 argument, got %d", len(args))
	}
	src := args[0]
	if src.TypeName() == "dict" {
		// Copy preserving insertion order: iteration yields keys.
		keys, err := materialize(src)
		if err != nil {
			return nil, err
		}
		vals := make([]objects.Object, len(keys))
		for i, k := range keys {
			v, err := objects.GetItem(src, k)
			if err != nil {
				return nil, err
			}
			vals[i] = v
		}
		return objects.NewDict(keys, vals)
	}
	items, err := materialize(src)
	if err != nil {
		return nil, err
	}
	keys := make([]objects.Object, 0, len(items))
	vals := make([]objects.Object, 0, len(items))
	for idx, item := range items {
		pair, err := materialize(item)
		if err != nil {
			// Probed on 3.14: dict([1]) -> TypeError: object is not
			// iterable, with no element index or type name.
			return nil, objects.Raise(objects.TypeError, "object is not iterable")
		}
		if len(pair) != 2 {
			return nil, objects.Raise(objects.ValueError,
				"dictionary update sequence element #%d has length %d; 2 is required", idx, len(pair))
		}
		keys = append(keys, pair[0])
		vals = append(vals, pair[1])
	}
	return objects.NewDict(keys, vals)
}

func init() {
	register(map[string]objects.Object{
		"min":       objects.NewFunc("min", -1, Min),
		"max":       objects.NewFunc("max", -1, Max),
		"sum":       objects.NewFunc("sum", -1, Sum),
		"round":     objects.NewFunc("round", -1, Round),
		"enumerate": objects.NewFunc("enumerate", -1, Enumerate),
		"zip":       objects.NewFunc("zip", -1, Zip),
		"list":      objects.NewFunc("list", -1, ListOf),
		"tuple":     objects.NewFunc("tuple", -1, TupleOf),
		"dict":      objects.NewFunc("dict", -1, DictOf),
		"set":       objects.NewFunc("set", -1, SetOf),
		"frozenset": objects.NewFunc("frozenset", -1, FrozensetOf),
		"divmod": objects.NewFunc("divmod", 2, func(args []objects.Object) (objects.Object, error) {
			return DivMod(args[0], args[1])
		}),
		"bin": objects.NewFunc("bin", 1, func(args []objects.Object) (objects.Object, error) {
			return Bin(args[0])
		}),
		"oct": objects.NewFunc("oct", 1, func(args []objects.Object) (objects.Object, error) {
			return Oct(args[0])
		}),
		"hex": objects.NewFunc("hex", 1, func(args []objects.Object) (objects.Object, error) {
			return Hex(args[0])
		}),
		"ord": objects.NewFunc("ord", 1, func(args []objects.Object) (objects.Object, error) {
			return Ord(args[0])
		}),
		"chr": objects.NewFunc("chr", 1, func(args []objects.Object) (objects.Object, error) {
			return Chr(args[0])
		}),
		"sorted": objects.NewFunc("sorted", 1, func(args []objects.Object) (objects.Object, error) {
			return Sorted(args[0])
		}),
		"reversed": objects.NewFunc("reversed", 1, func(args []objects.Object) (objects.Object, error) {
			return Reversed(args[0])
		}),
	})
}
