// Package runtime provides the builtin functions and process services
// that emitted unagi programs call into. It layers on pkg/objects and
// keeps CPython 3.14 behavior, including error messages.
package runtime

import (
	"io"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/unagi/pkg/objects"
)

// Stdout and Stderr are swappable so tests and hosts can capture output.
var (
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)

// Print implements print(*args) with the default sep and end. The str
// conversion can raise: print(2**20000) hits the 4300-digit limit.
func Print(args ...objects.Object) error {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		s, err := objects.StrE(a)
		if err != nil {
			return err
		}
		b.WriteString(s)
	}
	b.WriteByte('\n')
	_, err := io.WriteString(Stdout, b.String())
	return err
}

// PrintKw implements print(*args, sep=..., end=...). The file and flush
// keywords resolve at compile time: file must be the literal None and
// flush is dropped because Stdout never buffers here.
func PrintKw(args []objects.Object, sep, end objects.Object) error {
	sepS, err := printOpt("sep", sep, " ")
	if err != nil {
		return err
	}
	endS, err := printOpt("end", end, "\n")
	if err != nil {
		return err
	}
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteString(sepS)
		}
		s, serr := objects.StrE(a)
		if serr != nil {
			return serr
		}
		b.WriteString(s)
	}
	b.WriteString(endS)
	_, werr := io.WriteString(Stdout, b.String())
	return werr
}

// printOpt resolves one print separator option. Probed wording:
// sep must be None or a string, not int.
func printOpt(name string, o objects.Object, dflt string) (string, error) {
	if o == objects.None {
		return dflt, nil
	}
	if s, ok := objects.AsStr(o); ok {
		return s, nil
	}
	return "", objects.Raise(objects.TypeError, "%s must be None or a string, not %s", name, o.TypeName())
}

// Len implements len(o) returning a boxed int.
func Len(o objects.Object) (objects.Object, error) {
	n, err := objects.Len(o)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(n)), nil
}

// Next implements the next() builtin, next(it) or next(it, default).
func Next(args ...objects.Object) (objects.Object, error) {
	return objects.NextValue(args)
}

// Range implements range(stop), range(start, stop) and
// range(start, stop, step).
func Range(args ...objects.Object) (objects.Object, error) {
	if len(args) == 0 {
		return nil, objects.Raise(objects.TypeError, "range expected at least 1 argument, got 0")
	}
	if len(args) > 3 {
		return nil, objects.Raise(objects.TypeError, "range expected at most 3 arguments, got %d", len(args))
	}
	vals := make([]int64, len(args))
	for i, a := range args {
		v, ok := objects.AsInt(a)
		if !ok {
			// CPython handles range(2**100); this runtime keeps range on
			// int64 and reports the honest overflow instead of wrapping.
			// Documented divergence in the numbers-tower log.
			if objects.IsBigInt(a) {
				return nil, objects.Raise(objects.OverflowError,
					"Python int too large to convert to C ssize_t")
			}
			return nil, objects.Raise(objects.TypeError,
				"'%s' object cannot be interpreted as an integer", a.TypeName())
		}
		vals[i] = v
	}
	start, stop, step := int64(0), int64(0), int64(1)
	switch len(args) {
	case 1:
		stop = vals[0]
	case 2:
		start, stop = vals[0], vals[1]
	case 3:
		start, stop, step = vals[0], vals[1], vals[2]
	}
	if step == 0 {
		return nil, objects.Raise(objects.ValueError, "range() arg 3 must not be zero")
	}
	return objects.NewRange(start, stop, step), nil
}

// StrOf implements str(o). It can raise: str(2**20000) exceeds the
// 4300-digit int conversion limit.
func StrOf(o objects.Object) (objects.Object, error) {
	s, err := objects.StrE(o)
	if err != nil {
		return nil, err
	}
	return objects.NewStr(s), nil
}

// ReprOf implements repr(o), with the same digit-limit behavior as str.
func ReprOf(o objects.Object) (objects.Object, error) {
	s, err := objects.ReprE(o)
	if err != nil {
		return nil, err
	}
	return objects.NewStr(s), nil
}

// IntOf implements int(o) for str, float, bool and int arguments.
func IntOf(o objects.Object) (objects.Object, error) {
	if s, ok := objects.AsStr(o); ok {
		return intFromStr(o, s, 10, 10)
	}
	if o.TypeName() == "int" {
		// Ints pass through whole, spilled or not.
		return o, nil
	}
	if i, ok := objects.AsInt(o); ok {
		return objects.NewInt(i), nil
	}
	if f, ok := objects.AsFloat(o); ok {
		if math.IsInf(f, 0) {
			return nil, objects.Raise(objects.OverflowError, "cannot convert float infinity to integer")
		}
		if math.IsNaN(f) {
			return nil, objects.Raise(objects.ValueError, "cannot convert float NaN to integer")
		}
		t := math.Trunc(f)
		if t >= -9.2e18 && t <= 9.2e18 {
			return objects.NewInt(int64(t)), nil
		}
		// Probed: int(1e308) is the exact 309-digit value.
		b, _ := new(big.Float).SetFloat64(t).Int(nil)
		return objects.NewIntFromBig(b), nil
	}
	return nil, objects.Raise(objects.TypeError,
		"int() argument must be a string, a bytes-like object or a real number, not '%s'", o.TypeName())
}

// IntOfBase implements int(x, base). Probed check order on 3.14: the
// base type first, then its range, then x must be a string.
func IntOfBase(x, base objects.Object) (objects.Object, error) {
	b, ok := objects.AsInt(base)
	if !ok {
		if objects.IsBigInt(base) {
			// A spilled base clamps through AsSsize_t and lands in the
			// range error, probed with int("12", 2**100)-alikes.
			return nil, objects.Raise(objects.ValueError, "int() base must be >= 2 and <= 36, or 0")
		}
		return nil, objects.Raise(objects.TypeError,
			"'%s' object cannot be interpreted as an integer", base.TypeName())
	}
	if (b != 0 && b < 2) || b > 36 {
		return nil, objects.Raise(objects.ValueError, "int() base must be >= 2 and <= 36, or 0")
	}
	s, ok := objects.AsStr(x)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "int() can't convert non-string with explicit base")
	}
	digitBase := b
	if digitBase == 0 {
		digitBase = 10
	}
	return intFromStr(x, s, b, digitBase)
}

// FloatOf implements float(o) for str, int, bool and float arguments.
func FloatOf(o objects.Object) (objects.Object, error) {
	if s, ok := objects.AsStr(o); ok {
		trimmed := strings.TrimFunc(s, unicode.IsSpace)
		// Python accepts any Unicode decimal digit: float("１２") is 12.0.
		trimmed = asciiDigits(trimmed)
		bad := trimmed == ""
		// strconv accepts hex float syntax that Python rejects.
		lower := strings.ToLower(strings.TrimLeft(trimmed, "+-"))
		if strings.HasPrefix(lower, "0x") {
			bad = true
		}
		var v float64
		if !bad {
			var err error
			v, err = strconv.ParseFloat(trimmed, 64)
			// Out of range parses to an infinity, which Python allows.
			if err != nil && !strings.Contains(err.Error(), "out of range") {
				bad = true
			}
		}
		if bad {
			return nil, objects.Raise(objects.ValueError,
				"could not convert string to float: %s", objects.Repr(o))
		}
		return objects.NewFloat(v), nil
	}
	if b, ok := objects.AsBigInt(o); ok && objects.IsBigInt(o) {
		f, _ := new(big.Float).SetInt(b).Float64()
		if math.IsInf(f, 0) {
			// Probed: float(10**400) overflows instead of returning inf.
			return nil, objects.Raise(objects.OverflowError, "int too large to convert to float")
		}
		return objects.NewFloat(f), nil
	}
	if f, ok := objects.AsFloat(o); ok {
		return objects.NewFloat(f), nil
	}
	return nil, objects.Raise(objects.TypeError,
		"float() argument must be a string or a real number, not '%s'", o.TypeName())
}

// BoolOf implements bool(o), consulting a user __bool__/__len__ through the
// fallible truth protocol.
func BoolOf(o objects.Object) (objects.Object, error) {
	t, err := objects.TruthOf(o)
	if err != nil {
		return nil, err
	}
	return objects.NewBool(t), nil
}

// Abs implements abs(o) for int, bool and float arguments.
func Abs(o objects.Object) (objects.Object, error) {
	if objects.IsBigInt(o) {
		b, _ := objects.AsBigInt(o)
		if b.Sign() < 0 {
			return objects.NewIntFromBig(new(big.Int).Neg(b)), nil
		}
		return o, nil
	}
	if i, ok := objects.AsInt(o); ok {
		if i == math.MinInt64 {
			// abs(-2**63) spills, like every negation of the minimum.
			return objects.NewIntFromBig(new(big.Int).Neg(big.NewInt(i))), nil
		}
		if i < 0 {
			i = -i
		}
		return objects.NewInt(i), nil
	}
	if f, ok := objects.AsFloat(o); ok {
		return objects.NewFloat(math.Abs(f)), nil
	}
	return nil, objects.Raise(objects.TypeError, "bad operand type for abs(): '%s'", o.TypeName())
}

// IsInstance implements isinstance(obj, cls); the class-membership walk lives
// in pkg/objects next to the class object it inspects.
func IsInstance(obj, cls objects.Object) (objects.Object, error) {
	return objects.IsInstance(obj, cls)
}

// IsSubclass implements issubclass(sub, cls).
func IsSubclass(sub, cls objects.Object) (objects.Object, error) {
	return objects.IsSubclass(sub, cls)
}

// builtins maps names to function objects for the case where a builtin
// is passed around as a value. Variadic ones use a negative arity so
// Call skips the count check and they validate themselves. The map is
// allocated up front because several files register into it from their
// own init functions.
var builtins = make(map[string]objects.Object)

func register(m map[string]objects.Object) {
	for name, f := range m {
		builtins[name] = f
	}
}

func init() {
	register(map[string]objects.Object{
		"print": objects.NewFunc("print", -1, func(args []objects.Object) (objects.Object, error) {
			if err := Print(args...); err != nil {
				return nil, err
			}
			return objects.None, nil
		}),
		"len": exactlyOne("len", Len),
		"range": objects.NewFunc("range", -1, func(args []objects.Object) (objects.Object, error) {
			return Range(args...)
		}),
		"str": objects.NewFunc("str", -1, func(args []objects.Object) (objects.Object, error) {
			switch len(args) {
			case 0:
				return objects.NewStr(""), nil
			case 1:
				return StrOf(args[0])
			}
			return nil, objects.Raise(objects.TypeError, "str() takes at most 1 argument (%d given)", len(args))
		}),
		"repr": exactlyOne("repr", ReprOf),
		"int": objects.NewFunc("int", -1, func(args []objects.Object) (objects.Object, error) {
			switch len(args) {
			case 0:
				return objects.NewInt(0), nil
			case 1:
				return IntOf(args[0])
			case 2:
				return IntOfBase(args[0], args[1])
			}
			return nil, objects.Raise(objects.TypeError, "int() takes at most 2 arguments (%d given)", len(args))
		}),
		"float": objects.NewFunc("float", -1, func(args []objects.Object) (objects.Object, error) {
			switch len(args) {
			case 0:
				return objects.NewFloat(0), nil
			case 1:
				return FloatOf(args[0])
			}
			return nil, objects.Raise(objects.TypeError, "float expected at most 1 argument, got %d", len(args))
		}),
		"bool": objects.NewFunc("bool", -1, func(args []objects.Object) (objects.Object, error) {
			switch len(args) {
			case 0:
				return objects.False, nil
			case 1:
				return BoolOf(args[0])
			}
			return nil, objects.Raise(objects.TypeError, "bool expected at most 1 argument, got %d", len(args))
		}),
		"abs": exactlyOne("abs", Abs),
		"isinstance": objects.NewFunc("isinstance", 2, func(args []objects.Object) (objects.Object, error) {
			return IsInstance(args[0], args[1])
		}),
		"issubclass": objects.NewFunc("issubclass", 2, func(args []objects.Object) (objects.Object, error) {
			return IsSubclass(args[0], args[1])
		}),
		"next": objects.NewFunc("next", -1, objects.NextValue),
	})
}

// Builtin looks up a builtin by name as a callable object.
func Builtin(name string) (objects.Object, bool) {
	f, ok := builtins[name]
	return f, ok
}

// BuiltinFn returns a builtin's function object. The lowering only emits
// names from its own table, so a miss is a compiler bug, not user error.
func BuiltinFn(name string) objects.Object {
	f, ok := builtins[name]
	if !ok {
		panic("unagi: unknown builtin " + name)
	}
	return f
}
