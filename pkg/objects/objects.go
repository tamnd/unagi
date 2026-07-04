// Package objects implements the boxed Python object model for unagi.
// Every runtime value the emitted Go code touches is an Object here.
// CPython 3.14 is the oracle for results, reprs and error messages.
package objects

import (
	"fmt"
	"math/big"
)

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
	UnboundLocalError = "UnboundLocalError"
	AttributeError    = "AttributeError"
	RuntimeError      = "RuntimeError"
	OverflowError     = "OverflowError"
	RecursionError    = "RecursionError"
)

// Frame is one traceback entry: where the exception passed through on
// its way out. Frames collect innermost raise site first.
type Frame struct {
	File string
	Line int
	Func string
}

// Exception is the error type raised by all runtime operations and the
// object bound by `except ... as e`. Kind is the Python class name and
// Args the constructor arguments, so str/repr can match CPython.
type Exception struct {
	Kind            string
	Args            []Object
	Frames          []Frame
	Cause           *Exception // raise ... from X
	Context         *Exception // implicit chaining
	SuppressContext bool       // raise ... from None, or an explicit cause
	// Notes holds PEP 678 add_note strings. The traceback renderer prints
	// each one verbatim after the final exception line.
	Notes []string
	// Group holds the sub-exceptions when this is an ExceptionGroup or
	// BaseExceptionGroup; nil for every other exception. Args still keeps
	// the two constructor arguments, so repr echoes the given sequence.
	Group []*Exception
	// Reraised marks a bare `raise` so the next TB call skips its frame.
	// CPython 3.14 keeps the original raise-site line for the re-raising
	// function and adds no entry for the bare raise itself.
	Reraised bool
}

func (e *Exception) TypeName() string { return e.Kind }

// Text is str(e). Probed on 3.14: zero args give "", one arg gives
// str(arg) except KeyError which gives repr(arg), more args give the
// str of the args tuple. Groups append their sub-exception count.
func (e *Exception) Text() string {
	if e.Group != nil {
		s := "s"
		if len(e.Group) == 1 {
			s = ""
		}
		return fmt.Sprintf("%s (%d sub-exception%s)", Str(e.Args[0]), len(e.Group), s)
	}
	switch len(e.Args) {
	case 0:
		return ""
	case 1:
		if e.Kind == KeyError {
			return Repr(e.Args[0])
		}
		return Str(e.Args[0])
	}
	return Str(NewTuple(e.Args))
}

// Error is the final traceback line: "Kind: str", or the bare kind when
// str(e) is empty, matching `raise ValueError` vs `raise ValueError("x")`.
func (e *Exception) Error() string {
	if s := e.Text(); s != "" {
		return e.Kind + ": " + s
	}
	return e.Kind
}

// Raise builds an *Exception with one formatted string argument. This is
// the constructor every preformatted-message call site uses.
func Raise(kind, format string, a ...any) *Exception {
	return &Exception{Kind: kind, Args: []Object{NewStr(fmt.Sprintf(format, a...))}}
}

// NewException builds an *Exception carrying explicit argument objects,
// the ExceptionClass(args...) path.
func NewException(kind string, args []Object) *Exception {
	return &Exception{Kind: kind, Args: args}
}

type noneObject struct{}

func (*noneObject) TypeName() string { return "NoneType" }

type ellipsisObject struct{}

func (*ellipsisObject) TypeName() string { return "ellipsis" }

type boolObject struct{ v bool }

func (*boolObject) TypeName() string { return "bool" }

// intObject holds small values in v; big is non-nil exactly when the
// value does not fit int64, and then v is zero. See int.go.
type intObject struct {
	v   int64
	big *big.Int
}

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
	None     Object = &noneObject{}
	True     Object = &boolObject{v: true}
	False    Object = &boolObject{v: false}
	Ellipsis Object = &ellipsisObject{}
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
	if u, ok := f.(*functionObject); ok {
		return u.bind(args, nil, nil)
	}
	if m, ok := f.(*boundMethod); ok {
		return m.fn.bind(append([]Object{m.self}, args...), nil, nil)
	}
	if c, ok := f.(*classObject); ok {
		return Instantiate(c, args, nil, nil)
	}
	if inst, ok := f.(*instanceObject); ok {
		res, defined, err := instanceSpecial(inst, "__call__", args...)
		if err != nil {
			return nil, err
		}
		if defined {
			return res, nil
		}
		return nil, Raise(TypeError, "'%s' object is not callable", f.TypeName())
	}
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

// AsInt extracts an int64-sized integer value from an int or bool
// object. Spilled big ints return false; callers that must handle any
// magnitude go through AsBigInt.
func AsInt(o Object) (int64, bool) {
	switch x := o.(type) {
	case *intObject:
		if x.big != nil {
			return 0, false
		}
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
// A spilled int converts through big.Float and comes back as an
// infinity when it is out of range; arithmetic paths that must raise
// instead use asFloatChecked.
func AsFloat(o Object) (float64, bool) {
	switch x := o.(type) {
	case *floatObject:
		return x.v, true
	case *intObject:
		if x.big != nil {
			f, _ := new(big.Float).SetInt(x.big).Float64()
			return f, true
		}
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
