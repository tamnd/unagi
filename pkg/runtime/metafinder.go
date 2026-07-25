package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// The native meta_path finder. unagi resolves an `import name` statement in its
// own Go importer, so sys.meta_path starts empty. But importlib.import_module and
// anything else that drives the pure import machinery (pkgutil, plugin loaders,
// the encodings search fallback) loop sys.meta_path finders and find nothing, so
// they raise ModuleNotFoundError for a name the statement would resolve fine.
//
// This installs one finder that bridges the two: its find_spec runs the native
// importer, and on success hands back an importlib ModuleSpec whose loader yields
// the already-built native module. So the pure machinery gets a real spec and a
// module, and import_module returns the same object the statement would.
//
// The finder sits after any real finders a program installs, so a user meta_path
// entry still wins; it only answers names the runtime can import natively, which
// is every builtin, native-shim and vendored module.

// nativeMetaFinder builds the finder object sys.meta_path carries. It is a plain
// attribute bag exposing find_spec, the one method the import machinery calls on
// a meta_path entry.
func nativeMetaFinder() objects.Object {
	return objects.NewSimpleNamespace(
		[]string{"find_spec"},
		[]objects.Object{objects.NewFunc("find_spec", -1, metaFindSpec)},
	)
}

// metaFindSpec is find_spec(name, path=None, target=None). It tries the native
// import and, on success, returns importlib.machinery.ModuleSpec(name, loader).
// A name the runtime cannot import returns None so the next finder gets a turn,
// the same protocol a real finder follows.
func metaFindSpec(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 {
		return objects.None, nil
	}
	name, ok := objects.AsStr(args[0])
	if !ok {
		return objects.None, nil
	}
	if _, err := ImportModule(name); err != nil {
		// Not a name the runtime backs; defer to the rest of meta_path.
		return objects.None, nil
	}
	mach, err := ImportModule("importlib.machinery")
	if err != nil {
		return objects.None, nil
	}
	specCls, err := objects.LoadAttr(mach, "ModuleSpec")
	if err != nil {
		return objects.None, nil
	}
	loader := objects.NewSimpleNamespace(
		[]string{"create_module", "exec_module"},
		[]objects.Object{
			objects.NewFunc("create_module", 1, metaCreateModule),
			objects.NewFunc("exec_module", 1, metaExecModule),
		},
	)
	return objects.Call(specCls, []objects.Object{args[0], loader})
}

// metaCreateModule is loader.create_module(spec). The native importer already
// built and cached the module, so it hands that object back rather than letting
// the machinery make a fresh empty one.
func metaCreateModule(args []objects.Object) (objects.Object, error) {
	name, err := objects.LoadAttr(args[0], "name")
	if err != nil {
		return objects.None, nil
	}
	ns, ok := objects.AsStr(name)
	if !ok {
		return objects.None, nil
	}
	mod, err := ImportModule(ns)
	if err != nil {
		return objects.None, nil
	}
	return mod, nil
}

// metaExecModule is loader.exec_module(module). The native import ran the body
// already, so there is nothing left to execute.
func metaExecModule(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}
