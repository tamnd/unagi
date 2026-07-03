// Package objects implements the boxed Python object model for unagi.
// Every runtime value the emitted Go code touches is an Object here.
// CPython 3.14 is the oracle for results, reprs and error messages.
package objects

import "fmt"

// Object is the universal boxed value.
type Object interface {
	TypeName() string
}

// Exception kind names. These are the Python exception class names and
// they show up verbatim in Error() output and tracebacks.
const (
	TypeError         = "TypeError"
	ValueError        = "ValueError"
	ZeroDivisionError = "ZeroDivisionError"
	IndexError        = "IndexError"
	KeyError          = "KeyError"
	NameError         = "NameError"
	AttributeError    = "AttributeError"
	RuntimeError      = "RuntimeError"
)

// Exception is the error type raised by all runtime operations.
type Exception struct {
	Kind, Msg string
}

func (e *Exception) Error() string { return e.Kind + ": " + e.Msg }

// Raise builds an *Exception with a formatted message.
func Raise(kind, format string, a ...any) *Exception {
	return &Exception{Kind: kind, Msg: fmt.Sprintf(format, a...)}
}

type noneObject struct{}

func (*noneObject) TypeName() string { return "NoneType" }

type boolObject struct{ v bool }

func (*boolObject) TypeName() string { return "bool" }

type intObject struct{ v int64 }

func (*intObject) TypeName() string { return "int" }

type floatObject struct{ v float64 }

func (*floatObject) TypeName() string { return "float" }

type strObject struct{ v string }

func (*strObject) TypeName() string { return "str" }

type listObject struct{ elts []Object }

func (*listObject) TypeName() string { return "list" }

type tupleObject struct{ elts []Object }

func (*tupleObject) TypeName() string { return "tuple" }

type funcObject struct {
	name  string
	arity int // negative means variadic, no arity check
	fn    func(args []Object) (Object, error)
}

func (*funcObject) TypeName() string { return "function" }

type rangeObject struct{ start, stop, step int64 }

func (*rangeObject) TypeName() string { return "range" }

// The singletons. Identity checks (Is) rely on these being unique pointers.
var (
	None  Object = &noneObject{}
	True  Object = &boolObject{v: true}
	False Object = &boolObject{v: false}
)

// Small ints -5..256 are cached so `is` behaves like CPython for them.
const (
	smallIntMin = -5
	smallIntMax = 256
)

var smallInts [smallIntMax - smallIntMin + 1]*intObject

func init() {
	for i := range smallInts {
		smallInts[i] = &intObject{v: int64(i + smallIntMin)}
	}
}

// NewBool returns the True or False singleton.
func NewBool(b bool) Object {
	if b {
		return True
	}
	return False
}

// NewInt boxes an int64, reusing the small-int cache.
func NewInt(v int64) Object {
	if v >= smallIntMin && v <= smallIntMax {
		return smallInts[v-smallIntMin]
	}
	return &intObject{v: v}
}

// NewFloat boxes a float64.
func NewFloat(v float64) Object { return &floatObject{v: v} }

// NewStr boxes a string.
func NewStr(s string) Object { return &strObject{v: s} }

// NewList builds a list that owns the given slice.
func NewList(elts []Object) Object { return &listObject{elts: elts} }

// NewTuple builds a tuple that owns the given slice.
func NewTuple(elts []Object) Object { return &tupleObject{elts: elts} }

// NewFunc wraps a Go function as a callable object. A negative arity
// disables the positional argument count check; builtins use that.
func NewFunc(name string, arity int, fn func(args []Object) (Object, error)) Object {
	return &funcObject{name: name, arity: arity, fn: fn}
}

// NewRange builds a range object. The caller must reject a zero step.
func NewRange(start, stop, step int64) Object {
	return &rangeObject{start: start, stop: stop, step: step}
}

// Call invokes a function object with positional arguments.
func Call(f Object, args []Object) (Object, error) {
	fn, ok := f.(*funcObject)
	if !ok {
		return nil, Raise(TypeError, "'%s' object is not callable", f.TypeName())
	}
	if fn.arity >= 0 && len(args) != fn.arity {
		noun := "arguments"
		if fn.arity == 1 {
			noun = "argument"
		}
		verb := "were"
		if len(args) == 1 {
			verb = "was"
		}
		return nil, Raise(TypeError, "%s() takes %d positional %s but %d %s given",
			fn.name, fn.arity, noun, len(args), verb)
	}
	return fn.fn(args)
}

// AsInt extracts an integer value from an int or bool object.
func AsInt(o Object) (int64, bool) {
	switch x := o.(type) {
	case *intObject:
		return x.v, true
	case *boolObject:
		if x.v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// AsFloat extracts a numeric value from a float, int or bool object.
func AsFloat(o Object) (float64, bool) {
	switch x := o.(type) {
	case *floatObject:
		return x.v, true
	case *intObject:
		return float64(x.v), true
	case *boolObject:
		if x.v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// AsStr extracts the raw string from a str object.
func AsStr(o Object) (string, bool) {
	if x, ok := o.(*strObject); ok {
		return x.v, true
	}
	return "", false
}

func (r *rangeObject) length() int64 {
	if r.step > 0 {
		if r.stop <= r.start {
			return 0
		}
		return (r.stop - r.start + r.step - 1) / r.step
	}
	if r.stop >= r.start {
		return 0
	}
	return (r.start - r.stop - r.step - 1) / (-r.step)
}
