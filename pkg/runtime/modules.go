package runtime

import (
	"path/filepath"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// The import table. The generated modtable.go registers every sibling module
// the build compiled, keyed by import name; ImportModule executes a body at
// most once and every later import gets the same module object, the
// sys.modules behavior without sys itself yet.

// moduleEntry is one compiled module: its source path, whether it is a
// package (__init__.py) rather than a plain module, its Exec, and the module
// object once the first import created it.
type moduleEntry struct {
	file string
	pkg  bool
	exec func(*objects.Module) error
	mod  *objects.Module
}

var moduleTable = map[string]*moduleEntry{}

// RegisterModule adds one compiled module to the import table. The generated
// modtable.go calls it from init for every module in the program.
func RegisterModule(name, file string, pkg bool, exec func(*objects.Module) error) {
	moduleTable[name] = &moduleEntry{file: file, pkg: pkg, exec: exec}
}

// ImportModule is the import statement: walk the dotted name outward-in,
// importing each ancestor once, and return the leaf module. Each module
// object is in the table before its body starts, CPython's insert-before-exec
// order, so a circular import sees the partial module rather than recursing;
// a body that raises removes the entry again so a later import retries from
// scratch. A finished submodule is bound as an attribute on its parent, which
// is why import a.b.c makes a.b.c reachable from a.
func ImportModule(name string) (objects.Object, error) {
	var parent *objects.Module
	var m *objects.Module
	prefix := ""
	for _, seg := range strings.Split(name, ".") {
		if prefix == "" {
			prefix = seg
		} else {
			prefix = prefix + "." + seg
		}
		mod, err := importOne(prefix, seg, parent)
		if err != nil {
			return nil, err
		}
		m = mod
		parent = mod
	}
	return m, nil
}

// importOne imports a single dotted prefix whose ancestors are already
// imported, binding the result on the parent module when this call ran the
// body. The registry-hit path skips the parent bind, matching CPython where a
// cycle can read the submodule through sys.modules before the parent
// attribute exists.
func importOne(name, seg string, parent *objects.Module) (*objects.Module, error) {
	ent, ok := moduleTable[name]
	if !ok {
		return nil, moduleMissing(name)
	}
	if ent.mod != nil {
		return ent.mod, nil
	}
	m := objects.NewModule(name, ent.file)
	seedModuleAttrs(m, name, ent)
	ent.mod = m
	m.StartInit()
	if err := ent.exec(m); err != nil {
		ent.mod = nil
		return nil, err
	}
	m.FinishInit()
	if parent != nil {
		if err := objects.StoreAttr(parent, seg, m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// seedModuleAttrs sets the identity attributes a body can read about itself:
// __package__ is the parent name, the module's own name for a package, and
// the empty string at top level; a package also carries __path__ holding its
// directory, which is what marks it as a package to user code.
func seedModuleAttrs(m *objects.Module, name string, ent *moduleEntry) {
	pkgName := ""
	if ent.pkg {
		pkgName = name
	} else if i := strings.LastIndexByte(name, '.'); i >= 0 {
		pkgName = name[:i]
	}
	_ = objects.StoreAttr(m, "__package__", objects.NewStr(pkgName))
	if ent.pkg {
		path := objects.NewList([]objects.Object{objects.NewStr(filepath.Dir(ent.file))})
		_ = objects.StoreAttr(m, "__path__", path)
	}
}

// moduleMissing is the ModuleNotFoundError for a dotted prefix with no table
// entry. CPython names the first missing prefix, and appends the
// not-a-package suffix when the parent imported fine but is a plain module,
// both wordings probed on 3.14.
func moduleMissing(name string) error {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		parent := name[:i]
		if pent, ok := moduleTable[parent]; ok && !pent.pkg {
			return objects.Raise(objects.ModuleNotFoundError,
				"No module named '%s'; '%s' is not a package", name, parent)
		}
	}
	return objects.Raise(objects.ModuleNotFoundError, "No module named '%s'", name)
}

// ImportRoot is the plain import statement without as: import a.b.c executes
// the whole chain but binds the root module a in the importing scope.
func ImportRoot(name string) (objects.Object, error) {
	if _, err := ImportModule(name); err != nil {
		return nil, err
	}
	root := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		root = name[:i]
	}
	return moduleTable[root].mod, nil
}

// ImportFrom is `from module import name`: import the module, then read one
// attribute off it. When the attribute is missing but the name resolves to a
// compiled submodule, import that instead, CPython's fromlist fallback, which
// also leaves the submodule bound on the package. A plain miss raises
// ImportError with the module's path, or with the consider-renaming hint
// while the module body is still running, both wordings probed on 3.14 with
// script-adjacent files.
func ImportFrom(module, name string) (objects.Object, error) {
	mo, err := ImportModule(module)
	if err != nil {
		return nil, err
	}
	m := mo.(*objects.Module)
	if v, ok := m.Get(name); ok {
		return v, nil
	}
	if _, ok := moduleTable[module+"."+name]; ok {
		return ImportModule(module + "." + name)
	}
	if m.Initializing() {
		return nil, objects.Raise(objects.ImportError,
			"cannot import name '%s' from '%s' (consider renaming '%s' if it has the same name as a library you intended to import)",
			name, module, m.File())
	}
	return nil, objects.Raise(objects.ImportError,
		"cannot import name '%s' from '%s' (%s)", name, module, m.File())
}

// LoadModuleName reads a name the module's compile never saw statically: an
// attribute an importer set on the module object after import. A miss falls
// back to builtins and then raises NameError, the LOAD_GLOBAL order.
func LoadModuleName(m *objects.Module, name string) (objects.Object, error) {
	if m != nil {
		if v, ok := m.Get(name); ok {
			return v, nil
		}
	}
	if f, ok := Builtin(name); ok {
		return f, nil
	}
	return nil, nameNotDefined(name)
}
