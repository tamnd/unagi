// Package runtime provides the builtin functions and process services
// that emitted unagi programs call into. It layers on pkg/objects and
// keeps CPython 3.14 behavior, including error messages.
package runtime

import (
	"io"
	"math"
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

// Print implements print(*args) with the default sep and end.
func Print(args ...objects.Object) error {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(objects.Str(a))
	}
	b.WriteByte('\n')
	_, err := io.WriteString(Stdout, b.String())
	return err
}

// Len implements len(o) returning a boxed int.
func Len(o objects.Object) (objects.Object, error) {
	n, err := objects.Len(o)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(n)), nil
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

// StrOf implements str(o).
func StrOf(o objects.Object) objects.Object { return objects.NewStr(objects.Str(o)) }

// ReprOf implements repr(o).
func ReprOf(o objects.Object) objects.Object { return objects.NewStr(objects.Repr(o)) }

// IntOf implements int(o) for str, float, bool and int arguments.
func IntOf(o objects.Object) (objects.Object, error) {
	if s, ok := objects.AsStr(o); ok {
		trimmed := strings.TrimFunc(s, unicode.IsSpace)
		v, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, objects.Raise(objects.ValueError,
				"invalid literal for int() with base 10: %s", objects.Repr(o))
		}
		return objects.NewInt(v), nil
	}
	if i, ok := objects.AsInt(o); ok {
		return objects.NewInt(i), nil
	}
	if f, ok := objects.AsFloat(o); ok {
		if math.IsNaN(f) {
			return nil, objects.Raise(objects.ValueError, "cannot convert float NaN to integer")
		}
		return objects.NewInt(int64(math.Trunc(f))), nil
	}
	return nil, objects.Raise(objects.TypeError,
		"int() argument must be a string, a bytes-like object or a real number, not '%s'", o.TypeName())
}

// FloatOf implements float(o) for str, int, bool and float arguments.
func FloatOf(o objects.Object) (objects.Object, error) {
	if s, ok := objects.AsStr(o); ok {
		trimmed := strings.TrimFunc(s, unicode.IsSpace)
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
	if f, ok := objects.AsFloat(o); ok {
		return objects.NewFloat(f), nil
	}
	return nil, objects.Raise(objects.TypeError,
		"float() argument must be a string or a real number, not '%s'", o.TypeName())
}

// BoolOf implements bool(o).
func BoolOf(o objects.Object) objects.Object { return objects.NewBool(objects.Truth(o)) }

// Abs implements abs(o) for int, bool and float arguments.
func Abs(o objects.Object) (objects.Object, error) {
	if i, ok := objects.AsInt(o); ok {
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
		"len": objects.NewFunc("len", 1, func(args []objects.Object) (objects.Object, error) {
			return Len(args[0])
		}),
		"range": objects.NewFunc("range", -1, func(args []objects.Object) (objects.Object, error) {
			return Range(args...)
		}),
		"str": objects.NewFunc("str", -1, func(args []objects.Object) (objects.Object, error) {
			switch len(args) {
			case 0:
				return objects.NewStr(""), nil
			case 1:
				return StrOf(args[0]), nil
			}
			return nil, objects.Raise(objects.TypeError, "str() takes at most 1 argument (%d given)", len(args))
		}),
		"repr": objects.NewFunc("repr", 1, func(args []objects.Object) (objects.Object, error) {
			return ReprOf(args[0]), nil
		}),
		"int": objects.NewFunc("int", -1, func(args []objects.Object) (objects.Object, error) {
			switch len(args) {
			case 0:
				return objects.NewInt(0), nil
			case 1:
				return IntOf(args[0])
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
				return BoolOf(args[0]), nil
			}
			return nil, objects.Raise(objects.TypeError, "bool expected at most 1 argument, got %d", len(args))
		}),
		"abs": objects.NewFunc("abs", 1, func(args []objects.Object) (objects.Object, error) {
			return Abs(args[0])
		}),
	})
}

// Builtin looks up a builtin by name as a callable object.
func Builtin(name string) (objects.Object, bool) {
	f, ok := builtins[name]
	return f, ok
}
