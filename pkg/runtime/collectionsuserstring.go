package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// UserString wraps a str in self.data and subclasses Sequence. It carries the
// full str surface by delegating to the wrapped value: the methods that return
// a string wrap the result back in self.__class__, and the rest (the predicates,
// the searches, split and join) hand the raw result straight back, matching the
// pure-Python class method for method.

// userStringWrapMethods delegate to the wrapped str and rewrap the result in the
// wrapper class, so a subclass stays its own type through the call.
var userStringWrapMethods = []string{
	"capitalize", "casefold", "center", "expandtabs", "ljust", "lower",
	"lstrip", "removeprefix", "removesuffix", "replace", "rjust", "rstrip",
	"strip", "swapcase", "title", "translate", "upper", "zfill",
}

// userStringRawMethods delegate to the wrapped str and return the plain result:
// counts, predicates, searches, and the split and join family.
var userStringRawMethods = []string{
	"count", "encode", "endswith", "find", "format", "format_map", "index",
	"isalpha", "isalnum", "isascii", "isdecimal", "isdigit", "isidentifier",
	"islower", "isnumeric", "isprintable", "isspace", "istitle", "isupper",
	"join", "partition", "rfind", "rindex", "rpartition", "split", "rsplit",
	"splitlines", "startswith",
}

func buildUserString(base objects.Object) (objects.Object, error) {
	names := []string{
		"__init__", "__str__", "__repr__", "__hash__", "__int__", "__float__",
		"__eq__", "__lt__", "__le__", "__gt__", "__ge__",
		"__contains__", "__len__", "__getitem__",
		"__add__", "__radd__", "__mul__", "__rmul__", "__mod__", "__rmod__",
	}
	vals := []objects.Object{
		objects.NewMethod("__init__", 2, userStringInit),
		objects.NewMethod("__str__", 1, userStringStr),
		objects.NewMethod("__repr__", 1, userReprData),
		objects.NewMethod("__hash__", 1, userStringHash),
		objects.NewMethod("__int__", 1, userStringConv("int")),
		objects.NewMethod("__float__", 1, userStringConv("float")),
		userStringCmp("__eq__", objects.OpEq),
		userStringCmp("__lt__", objects.OpLt),
		userStringCmp("__le__", objects.OpLe),
		userStringCmp("__gt__", objects.OpGt),
		userStringCmp("__ge__", objects.OpGe),
		objects.NewMethod("__contains__", 2, userStringContains),
		objects.NewMethod("__len__", 1, userDictLen),
		objects.NewMethod("__getitem__", 2, userStringGetItem),
		objects.NewMethod("__add__", 2, userStringAdd),
		objects.NewMethod("__radd__", 2, userStringRAdd),
		objects.NewMethod("__mul__", 2, userStringMul),
		objects.NewMethod("__rmul__", 2, userStringMul),
		objects.NewMethod("__mod__", 2, userStringMod),
		objects.NewMethod("__rmod__", 2, userStringRMod),
	}
	for _, name := range userStringWrapMethods {
		names = append(names, name)
		vals = append(vals, userStringDelegate(name, true))
	}
	for _, name := range userStringRawMethods {
		names = append(names, name)
		vals = append(vals, userStringDelegate(name, false))
	}
	return objects.NewClass("UserString", "collections.UserString",
		[]objects.Object{base}, names, vals, nil, nil)
}

// unwrapUserString unwraps a UserString operand to its data, the isinstance
// guard the string methods make before handing an argument to the wrapped str.
func unwrapUserString(o objects.Object) (objects.Object, error) {
	if isUser(o, userStringClass) {
		return userData(o)
	}
	return o, nil
}

func userStringInit(args []objects.Object) (objects.Object, error) {
	self, seq := args[0], args[1]
	switch {
	case seq.TypeName() == "str":
		return objects.None, userSetData(self, seq)
	case isUser(seq, userStringClass):
		d, err := userData(seq)
		if err != nil {
			return nil, err
		}
		return objects.None, userSetData(self, d)
	default:
		return objects.None, userSetData(self, objects.NewStr(objects.Str(seq)))
	}
}

func userStringStr(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewStr(objects.Str(data)), nil
}

func userStringHash(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	h, err := objects.PyHash(data)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(h), nil
}

// userStringConv builds __int__ or __float__ by running the wrapped str through
// the matching builtin, the way int(userstring) and float(userstring) do.
func userStringConv(fn string) func(args []objects.Object) (objects.Object, error) {
	return func(args []objects.Object) (objects.Object, error) {
		data, err := userData(args[0])
		if err != nil {
			return nil, err
		}
		return objects.Call(builtins[fn], []objects.Object{data})
	}
}

func userStringCmp(name string, op objects.CmpOp) objects.Object {
	return objects.NewMethod(name, 2, func(args []objects.Object) (objects.Object, error) {
		data, err := userData(args[0])
		if err != nil {
			return nil, err
		}
		rhs, err := unwrapUserString(args[1])
		if err != nil {
			return nil, err
		}
		return objects.Compare(op, data, rhs)
	})
}

func userStringContains(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	char, err := unwrapUserString(args[1])
	if err != nil {
		return nil, err
	}
	return objects.Contains(data, char)
}

func userStringGetItem(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	part, err := objects.GetItem(data, args[1])
	if err != nil {
		return nil, err
	}
	return userReconstruct(args[0], part)
}

// userStringConcat wraps self.data joined with the other operand, drawing a
// non-str operand through str() the way the class does.
func userStringConcat(self, other objects.Object, selfFirst bool) (objects.Object, error) {
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	var rhs objects.Object
	if isUser(other, userStringClass) {
		if rhs, err = userData(other); err != nil {
			return nil, err
		}
	} else if other.TypeName() == "str" {
		rhs = other
	} else {
		rhs = objects.NewStr(objects.Str(other))
	}
	var sum objects.Object
	if selfFirst {
		sum, err = objects.Add(data, rhs)
	} else {
		sum, err = objects.Add(rhs, data)
	}
	if err != nil {
		return nil, err
	}
	return userReconstruct(self, sum)
}

func userStringAdd(args []objects.Object) (objects.Object, error) {
	return userStringConcat(args[0], args[1], true)
}

func userStringRAdd(args []objects.Object) (objects.Object, error) {
	return userStringConcat(args[0], args[1], false)
}

func userStringMul(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	prod, err := objects.Mul(data, args[1])
	if err != nil {
		return nil, err
	}
	return userReconstruct(args[0], prod)
}

func userStringMod(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	res, err := objects.Mod(data, args[1])
	if err != nil {
		return nil, err
	}
	return userReconstruct(args[0], res)
}

func userStringRMod(args []objects.Object) (objects.Object, error) {
	self, template := args[0], args[1]
	res, err := objects.Mod(objects.NewStr(objects.Str(template)), self)
	if err != nil {
		return nil, err
	}
	return userReconstruct(self, res)
}

// userStringDelegate builds a method that forwards to the wrapped str method of
// the same name, unwrapping any UserString arguments first. When wrap is set the
// result is rebuilt in the wrapper class; otherwise it is returned as is.
func userStringDelegate(name string, wrap bool) objects.Object {
	return objects.NewMethodKw(name, func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		self := pos[0]
		data, err := userData(self)
		if err != nil {
			return nil, err
		}
		rest := make([]objects.Object, len(pos)-1)
		for i, a := range pos[1:] {
			if rest[i], err = unwrapUserString(a); err != nil {
				return nil, err
			}
		}
		res, err := objects.CallMethodKw(data, name, rest, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		if wrap {
			return userReconstruct(self, res)
		}
		return res, nil
	})
}
