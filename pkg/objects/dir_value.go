package objects

// dir() of a builtin value type reports the fixed attribute set that type
// carries, the same list CPython's dir() builds by walking the type's MRO. The
// numbers and binary-data builtins have no writable per-instance directory, so
// each list is a constant transcribed from CPython 3.14 and verified against it.
// The scalar dunder threads (#972 through #976, #998) already expose every one
// of these names through getattr, so dir() and hasattr agree.

// intDirNames is dir() of an int. bool derives from int and adds no attribute
// of its own, so dir(bool) is identical and reuses this list.
var intDirNames = []string{
	"__abs__", "__add__", "__and__", "__bool__", "__ceil__", "__class__",
	"__delattr__", "__dir__", "__divmod__", "__doc__", "__eq__", "__float__",
	"__floor__", "__floordiv__", "__format__", "__ge__", "__getattribute__",
	"__getnewargs__", "__getstate__", "__gt__", "__hash__", "__index__",
	"__init__", "__init_subclass__", "__int__", "__invert__", "__le__",
	"__lshift__", "__lt__", "__mod__", "__mul__", "__ne__", "__neg__", "__new__",
	"__or__", "__pos__", "__pow__", "__radd__", "__rand__", "__rdivmod__",
	"__reduce__", "__reduce_ex__", "__repr__", "__rfloordiv__", "__rlshift__",
	"__rmod__", "__rmul__", "__ror__", "__round__", "__rpow__", "__rrshift__",
	"__rshift__", "__rsub__", "__rtruediv__", "__rxor__", "__setattr__",
	"__sizeof__", "__str__", "__sub__", "__subclasshook__", "__truediv__",
	"__trunc__", "__xor__", "as_integer_ratio", "bit_count", "bit_length",
	"conjugate", "denominator", "from_bytes", "imag", "is_integer", "numerator",
	"real", "to_bytes",
}

// floatDirNames is dir() of a float.
var floatDirNames = []string{
	"__abs__", "__add__", "__bool__", "__ceil__", "__class__", "__delattr__",
	"__dir__", "__divmod__", "__doc__", "__eq__", "__float__", "__floor__",
	"__floordiv__", "__format__", "__ge__", "__getattribute__", "__getformat__",
	"__getnewargs__", "__getstate__", "__gt__", "__hash__", "__init__",
	"__init_subclass__", "__int__", "__le__", "__lt__", "__mod__", "__mul__",
	"__ne__", "__neg__", "__new__", "__pos__", "__pow__", "__radd__",
	"__rdivmod__", "__reduce__", "__reduce_ex__", "__repr__", "__rfloordiv__",
	"__rmod__", "__rmul__", "__round__", "__rpow__", "__rsub__", "__rtruediv__",
	"__setattr__", "__sizeof__", "__str__", "__sub__", "__subclasshook__",
	"__truediv__", "__trunc__", "as_integer_ratio", "conjugate", "from_number",
	"fromhex", "hex", "imag", "is_integer", "real",
}

// complexDirNames is dir() of a complex.
var complexDirNames = []string{
	"__abs__", "__add__", "__bool__", "__class__", "__complex__", "__delattr__",
	"__dir__", "__doc__", "__eq__", "__format__", "__ge__", "__getattribute__",
	"__getnewargs__", "__getstate__", "__gt__", "__hash__", "__init__",
	"__init_subclass__", "__le__", "__lt__", "__mul__", "__ne__", "__neg__",
	"__new__", "__pos__", "__pow__", "__radd__", "__reduce__", "__reduce_ex__",
	"__repr__", "__rmul__", "__rpow__", "__rsub__", "__rtruediv__", "__setattr__",
	"__sizeof__", "__str__", "__sub__", "__subclasshook__", "__truediv__",
	"conjugate", "from_number", "imag", "real",
}

// bytesDirNames is dir() of a bytes.
var bytesDirNames = []string{
	"__add__", "__buffer__", "__bytes__", "__class__", "__contains__",
	"__delattr__", "__dir__", "__doc__", "__eq__", "__format__", "__ge__",
	"__getattribute__", "__getitem__", "__getnewargs__", "__getstate__", "__gt__",
	"__hash__", "__init__", "__init_subclass__", "__iter__", "__le__", "__len__",
	"__lt__", "__mod__", "__mul__", "__ne__", "__new__", "__reduce__",
	"__reduce_ex__", "__repr__", "__rmod__", "__rmul__", "__setattr__",
	"__sizeof__", "__str__", "__subclasshook__", "capitalize", "center", "count",
	"decode", "endswith", "expandtabs", "find", "fromhex", "hex", "index",
	"isalnum", "isalpha", "isascii", "isdigit", "islower", "isspace", "istitle",
	"isupper", "join", "ljust", "lower", "lstrip", "maketrans", "partition",
	"removeprefix", "removesuffix", "replace", "rfind", "rindex", "rjust",
	"rpartition", "rsplit", "rstrip", "split", "splitlines", "startswith",
	"strip", "swapcase", "title", "translate", "upper", "zfill",
}

// bytearrayDirNames is dir() of a bytearray.
var bytearrayDirNames = []string{
	"__add__", "__alloc__", "__buffer__", "__class__", "__contains__",
	"__delattr__", "__delitem__", "__dir__", "__doc__", "__eq__", "__format__",
	"__ge__", "__getattribute__", "__getitem__", "__getstate__", "__gt__",
	"__hash__", "__iadd__", "__imul__", "__init__", "__init_subclass__",
	"__iter__", "__le__", "__len__", "__lt__", "__mod__", "__mul__", "__ne__",
	"__new__", "__reduce__", "__reduce_ex__", "__release_buffer__", "__repr__",
	"__rmod__", "__rmul__", "__setattr__", "__setitem__", "__sizeof__", "__str__",
	"__subclasshook__", "append", "capitalize", "center", "clear", "copy",
	"count", "decode", "endswith", "expandtabs", "extend", "find", "fromhex",
	"hex", "index", "insert", "isalnum", "isalpha", "isascii", "isdigit",
	"islower", "isspace", "istitle", "isupper", "join", "ljust", "lower",
	"lstrip", "maketrans", "partition", "pop", "remove", "removeprefix",
	"removesuffix", "replace", "resize", "reverse", "rfind", "rindex", "rjust",
	"rpartition", "rsplit", "rstrip", "split", "splitlines", "startswith",
	"strip", "swapcase", "title", "translate", "upper", "zfill",
}

// memoryviewDirNames is dir() of a memoryview.
var memoryviewDirNames = []string{
	"__buffer__", "__class__", "__class_getitem__", "__delattr__", "__delitem__",
	"__dir__", "__doc__", "__enter__", "__eq__", "__exit__", "__format__",
	"__ge__", "__getattribute__", "__getitem__", "__getstate__", "__gt__",
	"__hash__", "__init__", "__init_subclass__", "__iter__", "__le__", "__len__",
	"__lt__", "__ne__", "__new__", "__reduce__", "__reduce_ex__",
	"__release_buffer__", "__repr__", "__setattr__", "__setitem__", "__sizeof__",
	"__str__", "__subclasshook__", "_from_flags", "c_contiguous", "cast",
	"contiguous", "count", "f_contiguous", "format", "hex", "index", "itemsize",
	"nbytes", "ndim", "obj", "readonly", "release", "shape", "strides",
	"suboffsets", "tobytes", "tolist", "toreadonly",
}

// builtinValueDirNames returns the dir() name list for a plain numbers or
// binary-data builtin value, or ok false for anything else (a subclass instance
// flows through the instance path instead). The returned slice is a fresh copy
// so a caller may sort or extend it without touching the shared table.
func builtinValueDirNames(o Object) ([]string, bool) {
	var src []string
	switch o.(type) {
	case *intObject:
		src = intDirNames
	case *boolObject:
		src = intDirNames
	case *floatObject:
		src = floatDirNames
	case *complexObject:
		src = complexDirNames
	case *bytesObject:
		src = bytesDirNames
	case *bytearrayObject:
		src = bytearrayDirNames
	case *memoryviewObject:
		src = memoryviewDirNames
	default:
		return nil, false
	}
	return append([]string(nil), src...), true
}
