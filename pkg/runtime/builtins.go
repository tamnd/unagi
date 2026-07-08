package runtime

import (
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// asIndex extracts an int the way CPython __index__ consumers do,
// accepting int and bool and rejecting everything else with the probed
// "cannot be interpreted as an integer" message. A spilled int reports
// the ssize_t overflow; callers with their own big-int behavior (round,
// chr, the base reprs) check before calling.
func asIndex(o objects.Object) (int64, error) {
	if i, ok := objects.AsInt(o); ok {
		return i, nil
	}
	if v, ok := objects.BuiltinValue(o); ok {
		// An int subclass indexes as its payload, so range(MyInt(3)) and the base
		// reprs read the underlying int.
		return asIndex(v)
	}
	if objects.IsBigInt(o) {
		return 0, objects.Raise(objects.OverflowError, "Python int too large to convert to C ssize_t")
	}
	return 0, objects.Raise(objects.TypeError,
		"'%s' object cannot be interpreted as an integer", o.TypeName())
}

// Min implements min(iterable) and min(a, b, ...).
func Min(args []objects.Object) (objects.Object, error) {
	return minMax(args, "min", objects.OpLt, objects.None, nil)
}

// Max implements max(iterable) and max(a, b, ...).
func Max(args []objects.Object) (objects.Object, error) {
	return minMax(args, "max", objects.OpGt, objects.None, nil)
}

// MinKw implements min with key= and default=. The default sentinel is a
// Go nil so an explicit default=None still counts as given.
func MinKw(args []objects.Object, key, dflt objects.Object) (objects.Object, error) {
	return minMax(args, "min", objects.OpLt, key, dflt)
}

// MaxKw implements max with key= and default=.
func MaxKw(args []objects.Object, key, dflt objects.Object) (objects.Object, error) {
	return minMax(args, "max", objects.OpGt, key, dflt)
}

// minMax keeps the first winner: a candidate only displaces the current
// best when it compares strictly less (or greater), so min(True, 1) is
// True and min(1, True) is 1, exactly as probed on 3.14. Probed order for
// the keyword forms: an empty iterable returns the default before the key
// is checked, and a non-None key fails the callable check on non-empty
// input because M1 has no function values.
func minMax(args []objects.Object, name string, op objects.CmpOp, key, dflt objects.Object) (objects.Object, error) {
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
			if dflt != nil {
				return dflt, nil
			}
			// Probed 3.14 wording: min() iterable argument is empty.
			return nil, objects.Raise(objects.ValueError, "%s() iterable argument is empty", name)
		}
	}
	if key != objects.None {
		return nil, objects.Raise(objects.TypeError, "'%s' object is not callable", key.TypeName())
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
	if bx, ok := objects.AsBigInt(x); ok {
		if len(args) == 1 {
			// round(True) is 1, an int, so always rebox; a plain int
			// comes back unchanged whatever its size.
			return objects.NewIntFromBig(bx), nil
		}
		nd, err := roundDigits(args[1])
		if err != nil {
			return nil, err
		}
		if nd >= 0 {
			return objects.NewIntFromBig(bx), nil
		}
		return objects.NewIntFromBig(roundIntNeg(bx, -nd)), nil
	}
	if f, ok := objects.AsFloat(x); ok {
		if len(args) == 1 {
			if math.IsInf(f, 0) {
				return nil, objects.Raise(objects.OverflowError, "cannot convert float infinity to integer")
			}
			if math.IsNaN(f) {
				return nil, objects.Raise(objects.ValueError, "cannot convert float NaN to integer")
			}
			r := math.RoundToEven(f)
			if r >= -9.2e18 && r <= 9.2e18 {
				return objects.NewInt(int64(r)), nil
			}
			// Probed: round(1e308) is the exact integer, like int().
			b, _ := new(big.Float).SetFloat64(r).Int(nil)
			return objects.NewIntFromBig(b), nil
		}
		nd, err := roundDigits(args[1])
		if err != nil {
			return nil, err
		}
		return roundFloat(f, nd)
	}
	// Probed: round("a") -> TypeError: type str doesn't define __round__ method.
	return nil, objects.Raise(objects.TypeError, "type %s doesn't define __round__ method", x.TypeName())
}

// roundDigits reads the ndigits argument, which unlike an index accepts
// any int size: round(1, 2**100) is 1. Spilled values clamp to sentinels
// the shortcut paths absorb.
func roundDigits(o objects.Object) (int64, error) {
	if objects.IsBigInt(o) {
		b, _ := objects.AsBigInt(o)
		if b.Sign() > 0 {
			return 1 << 62, nil
		}
		return -(1 << 62), nil
	}
	return asIndex(o)
}

// roundIntNeg rounds an int to a multiple of 10**digits, half to even,
// via floor division so ties land like round(150, -2) == round(250, -2)
// == 200 and round(-25, -1) == -20, all probed. A digits count at or
// past the bit length makes the quotient vanish, so the result is 0
// without materializing the power; CPython instead tries to build
// 10**digits and effectively hangs, a documented divergence for
// pathological ndigits.
func roundIntNeg(b *big.Int, digits int64) *big.Int {
	if digits >= int64(b.BitLen()) {
		return big.NewInt(0)
	}
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(digits), nil)
	q, r := new(big.Int).DivMod(b, p, new(big.Int))
	// DivMod is Euclidean with a positive divisor: 0 <= r < p, which is
	// exactly the floor split the tie rules below want.
	r.Lsh(r, 1)
	switch r.Cmp(p) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	return q.Mul(q, p)
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
// takes the modular inverse route pow supports since 3.8. Probed:
// pow(2**100, 2, 7) is 4, pow(2, -3, 7919) is 990, pow(2**100, -1, 9)
// is 4, and pow(2, 3, -5) is -2 because the result follows the sign of
// the modulus, floor style.
func Pow3(base, exp, mod objects.Object) (objects.Object, error) {
	bb, bok := objects.AsBigInt(base)
	eb, eok := objects.AsBigInt(exp)
	mb, mok := objects.AsBigInt(mod)
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
	if mb.Sign() == 0 {
		return nil, objects.Raise(objects.ValueError, "pow() 3rd argument cannot be 0")
	}
	am := new(big.Int).Abs(mb)
	// Exp with a negative exponent goes through the modular inverse and
	// reports a missing one as nil.
	r := new(big.Int).Exp(bb, eb, am)
	if r == nil {
		return nil, objects.Raise(objects.ValueError, "base is not invertible for the given modulus")
	}
	if mb.Sign() < 0 && r.Sign() != 0 {
		r.Sub(r, am)
	}
	return objects.NewIntFromBig(r), nil
}

// Bin implements bin(o).
func Bin(o objects.Object) (objects.Object, error) { return baseRepr(o, 2, "0b") }

// Oct implements oct(o).
func Oct(o objects.Object) (objects.Object, error) { return baseRepr(o, 8, "0o") }

// Hex implements hex(o).
func Hex(o objects.Object) (objects.Object, error) { return baseRepr(o, 16, "0x") }

// baseRepr formats an int in a base with the prefix after the sign, so
// bin(-5) is "-0b101". Bools count as ints: bin(True) is "0b1". The
// power-of-two bases are exempt from the 4300-digit limit, so
// bin(2**100000) works.
func baseRepr(o objects.Object, base int, prefix string) (objects.Object, error) {
	if b, ok := objects.AsBigInt(o); ok && objects.IsBigInt(o) {
		s := new(big.Int).Abs(b).Text(base)
		if b.Sign() < 0 {
			return objects.NewStr("-" + prefix + s), nil
		}
		return objects.NewStr(prefix + s), nil
	}
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
	if objects.IsBigInt(o) {
		// Probed: chr(2**100) is the range ValueError, not an overflow.
		return nil, objects.Raise(objects.ValueError, "chr() arg not in range(0x110000)")
	}
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

// SortedKw implements sorted(iterable, key=..., reverse=...). A non-None key is
// called once per element and the elements sort by those keys, the
// decorate-sort-undecorate CPython runs. reverse goes by truthiness and the
// descending comparator keeps the stable-sort equal-element order, the same
// result as CPython's reverse-sort-reverse dance.
func SortedKw(o, key, reverse objects.Object) (objects.Object, error) {
	items, err := materialize(o)
	if err != nil {
		return nil, err
	}
	keys := items
	if key != objects.None {
		if !objects.Callable(key) {
			return nil, objects.Raise(objects.TypeError, "'%s' object is not callable", key.TypeName())
		}
		keys = make([]objects.Object, len(items))
		for i, it := range items {
			keys[i], err = objects.Call(key, []objects.Object{it})
			if err != nil {
				return nil, err
			}
		}
	}
	desc, err := objects.TruthOf(reverse)
	if err != nil {
		return nil, err
	}
	// Sort an index permutation so the computed keys stay aligned with their
	// items through the stable reordering.
	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	var sortErr error
	sort.SliceStable(order, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		a, b := keys[order[i]], keys[order[j]]
		if desc {
			a, b = b, a
		}
		r, err := objects.Compare(objects.OpLt, a, b)
		if err != nil {
			sortErr = err
			return false
		}
		return objects.Truth(r)
	})
	if sortErr != nil {
		return nil, sortErr
	}
	out := make([]objects.Object, len(items))
	for i, idx := range order {
		out[i] = items[idx]
	}
	return objects.NewList(out), nil
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

// BytesOf implements bytes(), bytes(int), bytes(iterable-of-ints),
// bytes(bytes-like) and bytes(str, encoding).
func BytesOf(args []objects.Object) (objects.Object, error) {
	return objects.BytesOf(args)
}

// ByteArrayOf implements bytearray() and its constructor overloads, mirroring
// BytesOf but returning a mutable bytearray.
func ByteArrayOf(args []objects.Object) (objects.Object, error) {
	return objects.ByteArrayOf(args)
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
	if objects.IsDict(src) || objects.IsDictBackedInstance(src) {
		// Copy preserving insertion order: iteration yields keys. A dict
		// subclass such as defaultdict copies through the same mapping path,
		// producing a plain dict. A user dict subclass whose instances carry a
		// mapping store copies the same way, by its keys.
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

// HashOf implements hash(o) with CPython's PYTHONHASHSEED=0 values.
func HashOf(o objects.Object) (objects.Object, error) {
	h, err := objects.PyHash(o)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(h), nil
}

// Format implements the format builtin over the objects format engine.
func Format(args []objects.Object) (objects.Object, error) {
	if len(args) == 0 {
		return nil, objects.Raise(objects.TypeError, "format expected at least 1 argument, got 0")
	}
	if len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "format expected at most 2 arguments, got %d", len(args))
	}
	spec := ""
	if len(args) == 2 {
		s, ok := objects.AsStr(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError,
				"format() argument 2 must be str, not %s", args[1].TypeName())
		}
		spec = s
	}
	return objects.Format(args[0], spec)
}

// Super is the super type read as a value: super(type, obj) builds the bound
// proxy the same way the super() call form does, so a program that stores super
// in a name or passes it around still gets a working two-argument super. The
// zero-argument form needs the calling frame the compiler threads into the call
// site, which a value call has lost, so it raises the RuntimeError CPython does;
// the one-argument unbound form is not supported yet.
func Super(args []objects.Object) (objects.Object, error) {
	switch len(args) {
	case 0:
		return nil, objects.Raise(objects.RuntimeError, "super(): no arguments")
	case 2:
		return objects.NewSuper(args[0], args[1])
	case 1:
		return nil, objects.Raise(objects.TypeError, "super() with a single argument is not supported yet")
	default:
		return nil, objects.Raise(objects.TypeError,
			"super() takes at most 2 arguments (%d given)", len(args))
	}
}

func init() {
	register(map[string]objects.Object{
		"format":    objects.NewFunc("format", -1, Format),
		"min":       objects.NewFunc("min", -1, Min),
		"max":       objects.NewFunc("max", -1, Max),
		"sum":       objects.NewFunc("sum", -1, Sum),
		"round":     objects.NewFunc("round", -1, Round),
		"enumerate": objects.NewFunc("enumerate", -1, Enumerate),
		"zip":       objects.NewFunc("zip", -1, Zip),
		"list":      objects.NewFunc("list", -1, ListOf),
		"bytes":     objects.NewFunc("bytes", -1, BytesOf),
		"bytearray": objects.NewFunc("bytearray", -1, ByteArrayOf),
		"tuple":     objects.NewFunc("tuple", -1, TupleOf),
		"dict":      objects.NewFunc("dict", -1, DictOf),
		"set":       objects.NewFunc("set", -1, SetOf),
		"frozenset": objects.NewFunc("frozenset", -1, FrozensetOf),
		"super":     objects.NewFunc("super", -1, Super),
		// The fixed-arity ones validate their own counts because funcObject's
		// generic wording ("takes 1 positional argument but 2 were given")
		// differs from the CPython strings probed for these, which the static
		// call lowering in pkg/lower also emits.
		"divmod": objects.NewFunc("divmod", -1, func(args []objects.Object) (objects.Object, error) {
			if len(args) != 2 {
				return nil, objects.Raise(objects.TypeError, "divmod expected 2 arguments, got %d", len(args))
			}
			return DivMod(args[0], args[1])
		}),
		"pow": objects.NewFunc("pow", -1, func(args []objects.Object) (objects.Object, error) {
			switch len(args) {
			case 0:
				return nil, objects.Raise(objects.TypeError, "pow() missing required argument 'base' (pos 1)")
			case 1:
				return nil, objects.Raise(objects.TypeError, "pow() missing required argument 'exp' (pos 2)")
			case 2:
				return objects.Pow(args[0], args[1])
			case 3:
				return Pow3(args[0], args[1], args[2])
			}
			return nil, objects.Raise(objects.TypeError, "pow() takes at most 3 arguments (%d given)", len(args))
		}),
		"bin":      exactlyOne("bin", Bin),
		"oct":      exactlyOne("oct", Oct),
		"hex":      exactlyOne("hex", Hex),
		"ord":      exactlyOne("ord", Ord),
		"chr":      exactlyOne("chr", Chr),
		"hash":     exactlyOne("hash", HashOf),
		"sorted":   exactlyOneWorded("sorted", Sorted),
		"reversed": exactlyOneWorded("reversed", Reversed),
	})
}

// exactlyOne wraps a METH_O style builtin: "len() takes exactly one
// argument (2 given)", parentheses included.
func exactlyOne(name string, fn func(objects.Object) (objects.Object, error)) objects.Object {
	return objects.NewFunc(name, -1, func(args []objects.Object) (objects.Object, error) {
		if len(args) != 1 {
			return nil, objects.Raise(objects.TypeError,
				"%s() takes exactly one argument (%d given)", name, len(args))
		}
		return fn(args[0])
	})
}

// exactlyOneWorded wraps the argument-clinic style: "sorted expected 1
// argument, got 2", no parentheses.
func exactlyOneWorded(name string, fn func(objects.Object) (objects.Object, error)) objects.Object {
	return objects.NewFunc(name, -1, func(args []objects.Object) (objects.Object, error) {
		if len(args) != 1 {
			return nil, objects.Raise(objects.TypeError,
				"%s expected 1 argument, got %d", name, len(args))
		}
		return fn(args[0])
	})
}
