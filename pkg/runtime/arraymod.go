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
	return objects.StoreAttr(m, "typecodes", objects.NewStr("bBuwhHiIlLqQfd"))
}
