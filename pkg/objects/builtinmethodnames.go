package objects

// The builtin container types answer a bound-method read (f = s.add) with the
// same callable a direct call (s.add(x)) dispatches, so obj.method reads back as
// a first-class value the way list and int already do. Each name set tracks the
// switch in that type's CallMethodT dispatch exactly: a name is listed only when
// the dispatch handles it, so hasattr and the call agree and an unknown name is
// still the plain AttributeError CPython raises.

// strMethodNames tracks strMethod. casefold, format_map and maketrans are not in
// this tier's dispatch, so they are absent here too.
var strMethodNames = map[string]bool{
	"capitalize": true, "center": true, "count": true, "encode": true,
	"endswith": true, "expandtabs": true, "find": true, "format": true,
	"index": true, "isalnum": true, "isalpha": true, "isascii": true,
	"isdecimal": true, "isdigit": true, "isidentifier": true, "islower": true,
	"isnumeric": true, "isprintable": true, "isspace": true, "istitle": true,
	"isupper": true, "join": true, "ljust": true, "lower": true,
	"lstrip": true, "partition": true, "removeprefix": true, "removesuffix": true,
	"replace": true, "rfind": true, "rindex": true, "rjust": true,
	"rpartition": true, "rsplit": true, "rstrip": true, "split": true,
	"splitlines": true, "startswith": true, "strip": true, "swapcase": true,
	"title": true, "translate": true, "upper": true, "zfill": true,
}

// bytesMethodNames tracks the bytes dispatch. expandtabs is not dispatched for
// bytes in this tier, so it is left out to keep the read and the call agreeing.
var bytesMethodNames = map[string]bool{
	"capitalize": true, "center": true, "count": true, "decode": true,
	"endswith": true, "find": true, "hex": true, "index": true,
	"isalnum": true, "isalpha": true, "isascii": true, "isdigit": true,
	"islower": true, "isspace": true, "istitle": true, "isupper": true,
	"join": true, "ljust": true, "lower": true, "lstrip": true,
	"partition": true, "removeprefix": true, "removesuffix": true, "replace": true,
	"rfind": true, "rindex": true, "rjust": true, "rpartition": true,
	"rsplit": true, "rstrip": true, "split": true, "splitlines": true,
	"startswith": true, "strip": true, "swapcase": true, "title": true,
	"translate": true, "upper": true, "zfill": true,
}

// bytearrayMethodNames is the bytes surface plus the in-place mutators bytearray
// adds.
var bytearrayMethodNames = map[string]bool{
	"append": true, "clear": true, "copy": true, "extend": true,
	"insert": true, "pop": true, "remove": true, "reverse": true,
	"capitalize": true, "center": true, "count": true, "decode": true,
	"endswith": true, "find": true, "hex": true, "index": true,
	"isalnum": true, "isalpha": true, "isascii": true, "isdigit": true,
	"islower": true, "isspace": true, "istitle": true, "isupper": true,
	"join": true, "ljust": true, "lower": true, "lstrip": true,
	"partition": true, "removeprefix": true, "removesuffix": true, "replace": true,
	"rfind": true, "rindex": true, "rjust": true, "rpartition": true,
	"rsplit": true, "rstrip": true, "split": true, "splitlines": true,
	"startswith": true, "strip": true, "swapcase": true, "title": true,
	"translate": true, "upper": true, "zfill": true,
}

// setMethodNames is commonSetMethod plus the mutators in setMethod.
var setMethodNames = map[string]bool{
	"copy": true, "union": true, "intersection": true, "difference": true,
	"symmetric_difference": true, "issubset": true, "issuperset": true, "isdisjoint": true,
	"add": true, "clear": true, "difference_update": true, "discard": true,
	"intersection_update": true, "pop": true, "remove": true,
	"symmetric_difference_update": true, "update": true,
}

// frozensetMethodNames is commonSetMethod, the non-mutating surface a frozenset
// shares with set.
var frozensetMethodNames = map[string]bool{
	"copy": true, "union": true, "intersection": true, "difference": true,
	"symmetric_difference": true, "issubset": true, "issuperset": true, "isdisjoint": true,
}

// dictMethodNames tracks dictMethod, the surface every dict kind carries.
var dictMethodNames = map[string]bool{
	"clear": true, "copy": true, "fromkeys": true, "get": true,
	"items": true, "keys": true, "pop": true, "popitem": true,
	"setdefault": true, "update": true, "values": true,
}

// counterExtraMethodNames and orderedExtraMethodNames are the names each dict
// subclass adds on top of dictMethodNames, so a Counter reads back most_common
// and an OrderedDict reads back move_to_end.
var counterExtraMethodNames = map[string]bool{
	"elements": true, "most_common": true, "subtract": true, "total": true,
}

var orderedExtraMethodNames = map[string]bool{
	"move_to_end": true,
}

// tupleMethodNames is the two-method surface a plain tuple carries.
var tupleMethodNames = map[string]bool{
	"count": true, "index": true,
}

// builtinTypeMethodNames maps a builtin type name to its instance-method set, so
// a method read off the type (int.bit_length, str.upper) resolves to the unbound
// method the same names bind on an instance. bool shares int's methods, being a
// subtype. The classmethod and staticmethod forms (dict.fromkeys, int.from_bytes)
// resolve ahead of this through builtinTypeClassmethod, so only the plain
// instance descriptors land here.
var builtinTypeMethodNames = map[string]map[string]bool{
	"int": intMethodNames, "bool": intMethodNames, "float": floatMethodNames,
	"str": strMethodNames, "bytes": bytesMethodNames, "bytearray": bytearrayMethodNames,
	"list": listMethodNames, "tuple": tupleMethodNames,
	"set": setMethodNames, "frozenset": frozensetMethodNames, "dict": dictMethodNames,
}
