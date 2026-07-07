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

	ImportError         = "ImportError"
	ModuleNotFoundError = "ModuleNotFoundError"
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
	// Class is the exception's class object when it is a user-defined
	// exception subclass, so type(e), isinstance, and except matching key on
	// the real class identity rather than the Kind string. It is nil for the
	// built-in exceptions, whose Kind alone resolves back to a class through
	// ExcClass.
	Class *classObject
	// Dict is the exception's __dict__, the per-instance attribute store a
	// custom __init__ writes self.name into and a caught exception exposes.
	// Every exception carries one in CPython; here it is allocated on first
	// write so a plain built-in raise stays a bare struct until something
	// actually sets an attribute. DictOrder records insertion order so
	// __dict__ and vars() report attributes the way CPython's ordered dict
	// does.
	Dict      map[string]Object
	DictOrder []string
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

// ExcMessageLine is the final traceback line with a user __str__ dispatched:
// "Kind: str(e)", or the bare kind when str is empty. Error uses the built-in
// Text, so the traceback renderer calls this instead to honour a subclass that
// overrides __str__.
func ExcMessageLine(e *Exception) string {
	if s := Str(e); s != "" {
		return e.Kind + ": " + s
	}
	return e.Kind
}

// Raise builds an *Exception with one formatted string argument. This is
// the constructor every preformatted-message call site uses, and every call
// site raises what it builds, so the implicit context chains on here the way
// CPython's PyErr_SetObject does: the exception being handled right now, if
// any, becomes the new one's context. A fresh object cannot form a cycle, so
// the plain assignment needs none of chainInto's unlinking.
func Raise(kind, format string, a ...any) *Exception {
	return &Exception{Kind: kind, Args: []Object{NewStr(fmt.Sprintf(format, a...))}, Context: CurrentHandled()}
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

// tupleObject is an immutable sequence. named is nil for an ordinary tuple and
// points at the field metadata for a collections.namedtuple instance, which is
// a tuple subclass: it keeps every tuple behavior (indexing, iteration,
// equality with a plain tuple, and the tuple hash) and only adds the field
// names, the named repr, and the _fields/_make/_replace/_asdict helpers.
type tupleObject struct {
	elts  []Object
	named *namedType
}

func (x *tupleObject) TypeName() string {
	if x.named != nil {
		return x.named.name
	}
	return "tuple"
}

type funcObject struct {
	name  string
	arity int // negative means variadic, no arity check
	fn    func(args []Object) (Object, error)
	// attrs holds attributes a builtin attaches to itself, the way itertools.chain
	// carries chain.from_iterable. It stays nil for the ordinary builtins, which
	// expose no attributes beyond __name__/__qualname__.
	attrs map[string]Object
}

func (*funcObject) TypeName() string { return "function" }

// typeObject is a first-class type value for the object kinds that have no
// constructor to double as their type: NoneType, ellipsis, the function and
// method kinds, generators, and so on. The constructor-backed builtins (int,
// str, list, ...) and user classes are their own type objects already, so
// TypeOf hands those back directly and only reaches here for the rest. Its own
// type is `type`, matching CPython where type(type(None)) is type.
type typeObject struct{ name string }

func (*typeObject) TypeName() string { return "type" }

// typeSingletons caches one typeObject per name so identity is stable:
// type(None) is type(None) holds because both return the same pointer.
var typeSingletons = map[string]*typeObject{}

// TypeSingleton returns the cached type value for a kind that has no
// constructor, creating it on first use. Callers pass the CPython type name
// (NoneType, ellipsis, function, builtin_function_or_method, ...).
func TypeSingleton(name string) Object {
	if t, ok := typeSingletons[name]; ok {
		return t
	}
	t := &typeObject{name: name}
	typeSingletons[name] = t
	return t
}

// ClassOf returns the class of a user instance, the type object a user value's
// type() reports. A raised exception reports its built-in exception class, so
// type(e) is ValueError holds. ok is false for every other value, which TypeOf
// resolves by other means.
func ClassOf(o Object) (Object, bool) {
	if inst, ok := o.(*instanceObject); ok {
		return inst.cls, true
	}
	if e, ok := o.(*Exception); ok {
		if c, ok := excClassOf(e); ok {
			return c, true
		}
	}
	return nil, false
}

// excClassOf resolves a raised exception to its class object: the carried
// Class for a user subclass, otherwise the built-in class its Kind names.
func excClassOf(e *Exception) (*classObject, bool) {
	if e.Class != nil {
		return e.Class, true
	}
	return ExcClass(e.Kind)
}

// IsTypeValue reports whether o is itself a type object: a user or built-in
// class or one of the typeObject singletons. type() of any of these is the
// `type` metatype.
func IsTypeValue(o Object) bool {
	switch o.(type) {
	case *classObject, *typeObject:
		return true
	}
	return false
}

// BuiltinFuncName returns the name of a builtin function object, the funcObject
// the runtime registers for names like int, len, and type. ok is false for
// every other object, including user functions.
func BuiltinFuncName(o Object) (string, bool) {
	if f, ok := o.(*funcObject); ok {
		return f.name, true
	}
	return "", false
}

// IsBuiltinTypeName reports whether name is a builtin whose constructor doubles
// as a type object (int, str, list, ...), so TypeOf can hand back that
// constructor as the value's type.
func IsBuiltinTypeName(name string) bool { return builtinTypeReprs[name] }

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

// SetBuiltinAttr attaches an attribute to a builtin function object, the hook a
// runtime module uses to hang a helper off a builtin, such as
// chain.from_iterable. It is distinct from StoreAttr, which still refuses
// attribute assignment on a builtin the way user code sees it. fn must be a
// builtin function value from NewFunc.
func SetBuiltinAttr(fn Object, name string, v Object) error {
	f, ok := fn.(*funcObject)
	if !ok {
		return Raise(TypeError, "SetBuiltinAttr expects a builtin function, got %s", fn.TypeName())
	}
	if f.attrs == nil {
		f.attrs = make(map[string]Object)
	}
	f.attrs[name] = v
	return nil
}

// builtinTypeReprs and builtinFuncReprs split the builtins that can be read as
// values by how CPython reprs them: the type constructors print as classes and
// the plain builtins as built-in functions. A funcObject name in neither set is
// an internal helper and keeps the generic function repr.
var builtinTypeReprs = map[string]bool{
	"range": true, "str": true, "int": true, "float": true, "bool": true,
	"complex": true, "reversed": true, "enumerate": true, "zip": true,
	"list": true, "tuple": true, "dict": true, "set": true, "frozenset": true,
	"bytes": true, "bytearray": true, "type": true, "slice": true,
	"memoryview": true,
}

var builtinFuncReprs = map[string]bool{
	"print": true, "len": true, "repr": true, "abs": true, "min": true,
	"max": true, "sum": true, "round": true, "divmod": true, "pow": true,
	"bin": true, "oct": true, "hex": true, "ord": true, "chr": true,
	"hash": true, "sorted": true, "format": true, "next": true,
	"isinstance": true, "issubclass": true,
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
	if t, ok := f.(*namedTupleType); ok {
		return t.build.bind(args, nil, nil)
	}
	if m, ok := f.(*boundMethod); ok {
		return m.fn.bind(append([]Object{m.self}, args...), nil, nil)
	}
	if p, ok := f.(*partialObject); ok {
		return partialCall(p, args, nil, nil)
	}
	if c, ok := f.(*lruCacheObject); ok {
		return lruCall(c, args, nil, nil)
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
	if q, ok := f.(*quitterObject); ok {
		return q.call(args)
	}
	if p, ok := f.(*printerObject); ok {
		return p.call(args)
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

// Callable reports whether Call would dispatch f rather than raise the
// "not callable" TypeError: the function, bound-method, class and builtin
// objects are always callable, and an instance is callable exactly when its
// class defines __call__. It mirrors the Call type switch above so callable()
// never disagrees with an actual call.
func Callable(f Object) bool {
	switch x := f.(type) {
	case *functionObject, *boundMethod, *classObject, *funcObject:
		return true
	case *namedTupleType, *partialObject, *lruCacheObject:
		return true
	case *quitterObject, *printerObject:
		return true
	case *instanceObject:
		_, ok := x.cls.lookup("__call__")
		return ok
	}
	return false
}

// IsDict reports whether o is a dict or a dict subclass such as
// collections.defaultdict, so a caller that special-cases a mapping catches
// every dict-backed object regardless of its type name.
func IsDict(o Object) bool {
	_, ok := o.(*dictObject)
	return ok
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
