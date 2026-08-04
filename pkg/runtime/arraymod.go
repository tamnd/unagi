package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// array is a C module in CPython, so the runtime provides it in Go. It exposes
// one type, array.array, a dense typed sequence of machine values, plus the
// typecodes string and the ArrayType alias. The type object itself lives in
// pkg/objects behind objects.NewArray, which validates the typecode and the
// initializer; this file registers the callable and the module surface.
//
// array.array is a real builtin type, not a plain constructor: its name carries
// the module the way CPython's tp_name does, so it reprs as a class, answers
// isinstance and issubclass, and type(a) resolves it. It is registered into the
// global builtin table too, since type() looks the type object up by its dotted
// name; that name never leaks as a bare builtin because it is not an identifier.
var arrayType objects.Object

func init() {
	moduleTable["array"] = &moduleEntry{builtin: true, exec: initArray}

	// array(typecode, [initializer]): typecode is a length-1 str naming the
	// element kind, and the optional initializer seeds the array from a
	// bytes-like buffer, a str (unicode codes only), or any iterable.
	arrayType = objects.NewFuncKw("array.array", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(kwNames) > 0 {
			return nil, objects.Raise(objects.TypeError, "array.array() takes no keyword arguments")
		}
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "array() takes at least 1 argument (0 given)")
		}
		if len(pos) > 2 {
			return nil, objects.Raise(objects.TypeError, "array() takes at most 2 arguments (%d given)", len(pos))
		}
		var init objects.Object
		if len(pos) == 2 {
			init = pos[1]
		}
		// CPython 3.14 deprecates the 'u' (wchar_t) type code in favour of 'w', and
		// warns while building the array, before the initializer is validated, so a
		// bad initializer still leaves the warning behind. A filter that promotes it
		// to an error aborts the construction the way CPython's does.
		if s, ok := objects.AsStr(pos[0]); ok && s == "u" {
			if err := arrayUDeprecationWarn(); err != nil {
				return nil, err
			}
		}
		return objects.NewArray(pos[0], init)
	})

	builtins["array.array"] = arrayType
}

// initArray populates the array module: the array type under both its own name
// and the ArrayType alias, and the typecodes string of every accepted code.
func initArray(m *objects.Module) error {
	if err := objects.StoreAttr(m, "array", arrayType); err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "ArrayType", arrayType); err != nil {
		return err
	}
	// _array_reconstructor is the pickling hook array.__reduce_ex__ names to
	// rebuild an array from its raw machine bytes; it is public on the module so
	// an unpickler can import it. The same callable is handed to pkg/objects as
	// the reduction callable array.__reduce_ex__ emits from protocol 3 up, so the
	// reduction and the module attribute resolve to one object.
	reconstructor := objects.NewFuncKw("_array_reconstructor", arrayReconstructor)
	objects.ArrayReconstructor = reconstructor
	if err := objects.StoreAttr(m, "_array_reconstructor", reconstructor); err != nil {
		return err
	}
	return objects.StoreAttr(m, "typecodes", objects.NewStr("bBuwhHiIlLqQfd"))
}

// arrayUDeprecationMsg is verbatim from CPython 3.14 so a caller filtering or
// matching on it — unittest.assertWarns, a warnings filter — sees exactly the
// text it does there. The 'u' code is scheduled for removal in Python 3.16.
const arrayUDeprecationMsg = "The 'u' type code is deprecated and will be removed in Python 3.16"

// arrayUDeprecationWarn routes the 'u' type-code DeprecationWarning through the
// public warnings module, so it flows through the same filters and
// catch_warnings capture as any other warning. It mirrors the invert-bool hook:
// best-effort, since unagi bundles only the modules a program imports, so a
// script that never touches warnings has no warnings module to route through and
// the deprecation is silently skipped while the array still builds. Any program
// that observes warnings has imported it and sees the warning fire.
func arrayUDeprecationWarn() error {
	w, err := ImportModule("warnings")
	if err != nil {
		if isModuleNotFound(err) {
			return nil
		}
		return err
	}
	fn, err := objects.LoadAttr(w, "warn")
	if err != nil {
		return err
	}
	cat, ok := objects.ExcClass("DeprecationWarning")
	if !ok {
		return objects.Raise(objects.RuntimeError, "DeprecationWarning class unavailable")
	}
	_, err = objects.CallKwT(objects.MainThread(), fn,
		[]objects.Object{objects.NewStr(arrayUDeprecationMsg), cat},
		[]string{"stacklevel"}, []objects.Object{objects.NewInt(2)})
	return err
}
