package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _types is a built-in module, the C accelerator behind the pure-Python types
// module. types.py opens with `from _types import *` and only falls back to
// building each type object by introspection when that import fails. The
// fallback leans on reflection the ahead-of-time compiler cannot reproduce
// (type(int | str), type(type.__dict__), the traceback and frame types), so we
// provide _types in Go and the fallback never runs.
//
// Each name is the type object for one built-in kind. A constructor-less kind
// resolves to the same stable type singleton the interpreter hands back from
// type(), so type(x) lines up with the matching name here for every kind the
// interpreter can actually produce. MappingProxyType is the one that is also a
// live constructor: types and enum call it to wrap a member map in a read-only
// view.

func init() {
	moduleTable["_types"] = &moduleEntry{builtin: true, exec: initTypes}
}

// typesExports lists every name `from _types import *` binds, matching the C
// _types module on 3.14. The build seeds the star surface from the same list
// so the import binds the whole set at compile time.
var typesExports = []struct{ name, kind string }{
	{"AsyncGeneratorType", "async_generator"},
	{"BuiltinFunctionType", "builtin_function_or_method"},
	{"BuiltinMethodType", "builtin_function_or_method"},
	{"CapsuleType", "PyCapsule"},
	{"CellType", "cell"},
	{"ClassMethodDescriptorType", "classmethod_descriptor"},
	{"CodeType", "code"},
	{"CoroutineType", "coroutine"},
	{"EllipsisType", "ellipsis"},
	{"FrameType", "frame"},
	{"FunctionType", "function"},
	{"GeneratorType", "generator"},
	{"GenericAlias", "types.GenericAlias"},
	{"GetSetDescriptorType", "getset_descriptor"},
	{"LambdaType", "function"},
	{"MappingProxyType", "mappingproxy"},
	{"MemberDescriptorType", "member_descriptor"},
	{"MethodDescriptorType", "method_descriptor"},
	{"MethodType", "method"},
	{"MethodWrapperType", "method-wrapper"},
	{"ModuleType", "module"},
	{"NoneType", "NoneType"},
	{"NotImplementedType", "NotImplementedType"},
	{"SimpleNamespace", "types.SimpleNamespace"},
	{"TracebackType", "traceback"},
	{"UnionType", "typing.Union"},
	{"WrapperDescriptorType", "wrapper_descriptor"},
}

func initTypes(m *objects.Module) error {
	for _, e := range typesExports {
		if err := objects.StoreAttr(m, e.name, objects.TypeSingleton(e.kind)); err != nil {
			return err
		}
	}
	return nil
}
