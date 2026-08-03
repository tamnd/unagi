package runtime

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// The import table and the live registry. The generated modtable.go registers
// every compiled module in moduleTable, keyed by import name; the module
// objects the imports created live in a real Python dict, the object
// sys.modules exposes. The import machinery reads that dict before consulting
// the table and re-reads it after a body runs, which is what makes pokes,
// deletes, None entries, and sys.modules[__name__] = obj self-replacement
// behave like CPython.

// moduleEntry is one compiled module: its source path, whether it is a
// package (__init__.py) rather than a plain module, and its Exec. A namespace
// package (a directory with no __init__.py) has no source and no exec: ns is
// true and file holds the directory for its __path__. A builtin entry is a
// module the runtime provides itself, like sys: no file, and its exec fills
// in the attributes from Go.
type moduleEntry struct {
	file    string
	pkg     bool
	ns      bool
	builtin bool
	exec    func(*objects.Module) error
}

var moduleTable = map[string]*moduleEntry{}

// frozenImportlibAliases maps the frozen bootstrap module names to the importlib
// source modules they are the frozen form of. CPython embeds these as frozen
// modules and registers them under both names as the same object; here the two
// sources import normally, so the frozen name resolves to the same module. This
// is what zipimport pulls in (import _frozen_importlib_external as
// _bootstrap_external).
var frozenImportlibAliases = map[string]string{
	"_frozen_importlib":          "importlib._bootstrap",
	"_frozen_importlib_external": "importlib._bootstrap_external",
}

// modules is the live registry dict. Deleting an entry makes the next import
// run the body again, storing None halts the import, and storing any other
// object makes the import hand that object out.
var modules = newModulesDict()

func newModulesDict() objects.Object {
	d, err := objects.NewDict(nil, nil)
	if err != nil {
		panic(err)
	}
	return d
}

func modulesGet(name string) (objects.Object, bool) {
	v, err := objects.GetItem(modules, objects.NewStr(name))
	if err != nil {
		// A str key cannot fail to hash, so the only error here is the
		// KeyError for a missing entry.
		return nil, false
	}
	return v, true
}

func modulesSet(name string, v objects.Object) {
	_ = objects.SetItem(modules, objects.NewStr(name), v)
}

func modulesDel(name string) {
	_ = objects.DelItem(modules, objects.NewStr(name))
}

// ShimmedModules returns the names the runtime provides itself as Go modules,
// the built-in and whole-module shims registered from init. The build reads
// this set to keep those names off the source floor: a name the runtime backs
// in Go must not also be compiled from the vendored .py, or the two would race
// to register the same table key and the source form would drag in accelerators
// that do not exist yet. The list is sorted so callers get a deterministic set.
// It reflects this binary's runtime, which is the same pkg/runtime copied into
// the program under build, so the compiler and the program agree on the set.
func ShimmedModules() []string {
	names := make([]string, 0, len(moduleTable))
	for name, ent := range moduleTable {
		if ent.builtin {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// mainModule is the running program's top-level module, the object
// `import __main__` and sys.modules['__main__'] hand back. The generated main
// sets it through SetMainModule when the program calls globals() and so builds
// its module object; a program that never does leaves it nil and the first
// `import __main__` synthesizes an empty stand-in.
var mainModule *objects.Module

// SetMainModule records the running program's top-level module so a later
// `import __main__` returns the same live namespace the body writes to. The
// generated main calls it once, right after it builds the __main__ module, so
// __main__.__dict__ is globals() for a program that reads either.
func SetMainModule(m *objects.Module) {
	mainModule = m
	modulesSet("__main__", m)
}

// ensureMainModule returns the top-level module, building an empty one the first
// time a program that never set it imports __main__. The stand-in is a real
// module with a valid __dict__, so the import and attribute reads work; it just
// holds no top-level bindings, since a program that does not call globals() keeps
// those in Go variables the runtime cannot enumerate.
func ensureMainModule() objects.Object {
	if mainModule == nil {
		mainModule = objects.NewModule("__main__", "")
		modulesSet("__main__", mainModule)
	}
	return mainModule
}

// RegisterModule adds one compiled module to the import table. The generated
// modtable.go calls it from init for every module in the program.
func RegisterModule(name, file string, pkg bool, exec func(*objects.Module) error) {
	moduleTable[name] = &moduleEntry{file: file, pkg: pkg, exec: exec}
}

// RegisterNamespace adds a PEP 420 namespace package to the import table. dir
// is the directory that stands in for the package; it becomes the sole entry
// of __path__. A namespace package has no body to run, so importing it only
// creates the module object and seeds its identity attributes.
func RegisterNamespace(name, dir string) {
	moduleTable[name] = &moduleEntry{file: dir, pkg: true, ns: true}
}

// ImportModule is the import statement: walk the dotted name outward-in,
// importing each ancestor once, and return the leaf. Each module object is in
// the registry before its body starts, CPython's insert-before-exec order, so
// a circular import sees the partial module rather than recursing; a body
// that raises removes the entry again so a later import retries from scratch.
// A finished submodule is bound as an attribute on its parent, which is why
// import a.b.c makes a.b.c reachable from a.
func ImportModule(name string) (objects.Object, error) {
	var parent objects.Object
	var m objects.Object
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

// dunderImport is the __import__ builtin, the hook the import statement lowers
// through and that the encodings search function calls directly to pull a codec
// submodule in by name. It imports the whole dotted path and, matching CPython,
// returns the top package when fromlist is empty and the leaf module when it is
// not: encodings calls __import__('encodings.utf_8', fromlist=('*',)) and wants
// the utf_8 submodule back. Only absolute imports (level 0) are modeled; the
// relative form the statement handles through its own lowering is not reachable
// here.
func dunderImport(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "__import__() missing required argument 'name'")
	}
	name, ok := objects.AsStr(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "__import__() argument 'name' must be str, not %s", pos[0].TypeName())
	}
	// fromlist and level are positional 3 and 4, or keyword; globals and locals
	// (positional 1 and 2) are ignored the way CPython's builtin ignores them.
	var fromlist objects.Object
	level := int64(0)
	if len(pos) >= 4 {
		fromlist = pos[3]
	}
	if len(pos) >= 5 {
		if lv, ok := objects.AsInt(pos[4]); ok {
			level = lv
		}
	}
	for i, kn := range kwNames {
		switch kn {
		case "fromlist":
			fromlist = kwVals[i]
		case "level":
			if lv, ok := objects.AsInt(kwVals[i]); ok {
				level = lv
			}
		case "globals", "locals":
			// Accepted and ignored, as the builtin does.
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for __import__()", kn)
		}
	}
	if level != 0 {
		return nil, objects.Raise(objects.ImportError, "__import__: relative import is not supported")
	}
	leaf, err := ImportModule(name)
	if err != nil {
		return nil, err
	}
	if importHasFromList(fromlist) {
		return leaf, nil
	}
	top := strings.SplitN(name, ".", 2)[0]
	if v, ok := modulesGet(top); ok {
		return v, nil
	}
	return leaf, nil
}

// importHasFromList reports whether the fromlist argument names anything, so
// __import__ returns the leaf module rather than the top package. None and an
// empty sequence both mean no fromlist.
func importHasFromList(fromlist objects.Object) bool {
	if fromlist == nil || fromlist == objects.None {
		return false
	}
	n, err := objects.Len(fromlist)
	if err != nil {
		return true
	}
	return n > 0
}

// importOne imports a single dotted prefix whose ancestors are already
// imported, binding the result on the parent when this call ran the body. The
// registry-hit path skips the parent bind, matching CPython where a cycle can
// read the submodule through sys.modules before the parent attribute exists.
// The value bound and returned is whatever the registry holds after the body
// finished, so a body that replaced its own entry hands out the replacement.
func importOne(name, seg string, parent objects.Object) (objects.Object, error) {
	if name == "genericpath" {
		if _, ok := modulesGet("os"); !ok {
			// genericpath imports os, os imports posixpath, and posixpath runs
			// `from genericpath import *`. When genericpath is the cold entry, its
			// body is stuck at its own `import os` line when posixpath reaches back
			// for it, so the star-import sees a half-built module and raises. CPython
			// never hits this because its startup imports os first, which loads
			// genericpath fully through the same cycle (os is the entry, so
			// genericpath completes before anything reads it). Do the same here: pull
			// os in first, which populates genericpath as a side effect, then fall
			// through to the registry hit below. The guard reads os from sys.modules,
			// where the import machinery registers it before running its body, so the
			// recursive genericpath import this triggers finds os present and does not
			// re-enter this branch.
			if _, err := ImportModule("os"); err != nil {
				return nil, err
			}
		}
	}
	if v, ok := modulesGet(name); ok {
		if v == objects.None {
			// A None entry means an earlier import was deliberately stopped;
			// wording probed on 3.14.
			return nil, objects.Raise(objects.ImportError,
				"import of %s halted; None in sys.modules", name)
		}
		return v, nil
	}
	ent, ok := moduleTable[name]
	if !ok {
		if name == "__main__" {
			// The top-level program is __main__. There is no table entry for it,
			// so `import __main__` resolves to the running module the generated
			// main registered through SetMainModule, or, for a program that never
			// built its module object (it does not call globals()), a valid empty
			// stand-in so the import and __main__.__dict__ still work.
			return ensureMainModule(), nil
		}
		if target, ok := frozenImportlibAliases[name]; ok {
			// _frozen_importlib and _frozen_importlib_external are the frozen forms
			// of importlib._bootstrap and importlib._bootstrap_external; CPython
			// registers them as the same module objects under both names. The two
			// sources already import here, so alias the frozen name to the same
			// object and register it in sys.modules, which is what zipimport (and
			// any other consumer that reaches for the bootstrap by its frozen name)
			// pulls in. Module identity is preserved, matching CPython.
			mod, err := ImportModule(target)
			if err != nil {
				return nil, err
			}
			modulesSet(name, mod)
			if parent != nil {
				if err := objects.StoreAttr(parent, seg, mod); err != nil {
					return nil, err
				}
			}
			return mod, nil
		}
		if strings.HasPrefix(name, "_sysconfigdata_") {
			// sysconfig imports a build-time data module whose name it computes
			// from sys.abiflags/sys.platform/_multiarch, so the exact name is
			// platform specific and cannot be a static table entry. An AOT Go
			// binary was never produced by a CPython build, so there is no real
			// build database; synthesize one under whatever name was asked for so
			// sysconfig, and pydoc and zoneinfo through it, import.
			m := objects.NewBuiltinModule(name)
			if err := initSysconfigData(m); err != nil {
				return nil, err
			}
			modulesSet(name, m)
			return m, nil
		}
		return nil, moduleMissing(name)
	}
	var m *objects.Module
	if ent.builtin {
		m = objects.NewBuiltinModule(name)
	} else {
		m = objects.NewModule(name, ent.file)
		seedModuleAttrs(m, name, ent)
	}
	modulesSet(name, m)
	if ent.ns {
		// A namespace package has no body. Its module object is already
		// complete once the identity attributes are seeded, so bind it on the
		// parent and return without a run.
		if parent != nil {
			if err := objects.StoreAttr(parent, seg, m); err != nil {
				return nil, err
			}
		}
		return m, nil
	}
	m.StartInit()
	if err := ent.exec(m); err != nil {
		modulesDel(name)
		return nil, err
	}
	m.FinishInit()
	final, ok := modulesGet(name)
	if !ok {
		// The body deleted its own registry entry. CPython surfaces the raw
		// KeyError from its post-exec re-read, probed on 3.14; the key object
		// is the single argument so str(e) is the repr of the name.
		return nil, objects.NewException(objects.KeyError, []objects.Object{objects.NewStr(name)})
	}
	if name == "importlib._bootstrap" {
		// CPython's interpreter startup runs _bootstrap._setup(sys, _imp) to bind
		// sys and _imp into the bootstrap's namespace. unagi resolves imports in Go,
		// so importlib/__init__.py takes its `import _frozen_importlib as _bootstrap`
		// success branch and never reaches the _setup call in the ImportError branch.
		// Run it here, once, right after the body, so the pure import machinery works.
		if err := setupBootstrap(final); err != nil {
			modulesDel(name)
			return nil, err
		}
	}
	if parent != nil {
		if err := objects.StoreAttr(parent, seg, final); err != nil {
			return nil, err
		}
	}
	return final, nil
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
	if ent.ns {
		// A namespace package's directory is its __path__ entry, and CPython
		// gives it a None __file__ rather than a path, since there is no
		// __init__.py backing it.
		_ = objects.StoreAttr(m, "__path__", objects.NewList([]objects.Object{objects.NewStr(ent.file)}))
		_ = objects.StoreAttr(m, "__file__", objects.None)
		return
	}
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
	msg := fmt.Sprintf("No module named '%s'", name)
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		parent := name[:i]
		if pent, ok := moduleTable[parent]; ok && !pent.pkg {
			msg = fmt.Sprintf("No module named '%s'; '%s' is not a package", name, parent)
		}
	}
	// Carry the missing name on .name the way CPython's ModuleNotFoundError does,
	// so a caller that inspects it, site.execsitecustomize checks exc.name to
	// swallow a missing optional module silently, sees the name it expects.
	kw, err := objects.NewDict([]objects.Object{objects.NewStr("name")}, []objects.Object{objects.NewStr(name)})
	if err != nil {
		return err
	}
	exc, err := NewExcKw(objects.ModuleNotFoundError, []objects.Object{objects.NewStr(msg)}, kw)
	if err != nil {
		return err
	}
	return RaiseObj(exc)
}

// ImportRoot is the plain import statement without as: import a.b.c executes
// the whole chain but binds the root module a in the importing scope. The
// binding comes off the registry, so a root that replaced its own entry binds
// the replacement.
func ImportRoot(name string) (objects.Object, error) {
	if _, err := ImportModule(name); err != nil {
		return nil, err
	}
	root := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		root = name[:i]
	}
	if v, ok := modulesGet(root); ok {
		return v, nil
	}
	// A deeper body deleted the root's entry between exec and this read; the
	// raw KeyError matches CPython's registry lookup.
	return nil, objects.NewException(objects.KeyError, []objects.Object{objects.NewStr(root)})
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
	m, ok := mo.(*objects.Module)
	if !ok {
		return importFromObject(mo, name)
	}
	if v, ok := m.Get(name); ok {
		return v, nil
	}
	if _, ok := moduleTable[module+"."+name]; ok {
		return ImportModule(module + "." + name)
	}
	// PEP 562: `from module import name` consults a module-level __getattr__ for a
	// name absent from the namespace, the same hook attribute access uses, so
	// typing's lazily built Match and Pattern import. A miss the hook signals with
	// AttributeError falls through to the ordinary ImportError wording below.
	if hook, ok := m.Get("__getattr__"); ok && objects.Callable(hook) {
		v, err := objects.Call(hook, []objects.Object{objects.NewStr(name)})
		if err == nil {
			return v, nil
		}
		if !isAttributeError(err) {
			return nil, err
		}
	}
	if m.Initializing() {
		return nil, objects.Raise(objects.ImportError,
			"cannot import name '%s' from '%s' (consider renaming '%s' if it has the same name as a library you intended to import)",
			name, module, m.File())
	}
	return nil, objects.Raise(objects.ImportError,
		"cannot import name '%s' from '%s' (%s)", name, module, m.File())
}

// importFromObject is from-import off a registry entry that is not a module:
// a body replaced sys.modules[__name__] with an arbitrary object, and the
// import reads the name as a plain attribute of it. The miss wording takes
// the object's own __name__ and __file__, falling back to CPython's
// unknown-module placeholders, probed on 3.14 against an instance carrying
// neither attribute.
func importFromObject(mo objects.Object, name string) (objects.Object, error) {
	v, err := objects.LoadAttr(mo, name)
	if err == nil {
		return v, nil
	}
	if !isAttributeError(err) {
		return nil, err
	}
	shown := "<unknown module name>"
	if nv, nerr := objects.LoadAttr(mo, "__name__"); nerr == nil {
		if s, ok := objects.AsStr(nv); ok {
			shown = s
		}
	}
	location := "unknown location"
	if fv, ferr := objects.LoadAttr(mo, "__file__"); ferr == nil {
		if s, ok := objects.AsStr(fv); ok {
			location = s
		}
	}
	return nil, objects.Raise(objects.ImportError,
		"cannot import name '%s' from '%s' (%s)", name, shown, location)
}

// RelativeImportError raises the ImportError for a relative import the
// compile resolved as impossible. The wording is chosen at compile time:
// no-known-parent for the entry script and top-level modules, beyond-top-level
// when the dots walk past the package tree, both probed on 3.14. The raise
// happens here rather than at compile time because CPython only errors when
// the statement executes.
func RelativeImportError(msg string) (objects.Object, error) {
	return nil, objects.Raise(objects.ImportError, "%s", msg)
}

// StarLoad reads one name for `from m import *` under the default rule, where
// only names actually bound at star time transfer. ok is false when the name
// is currently unbound, and the caller leaves its own binding untouched;
// CPython's star import skips such names rather than clearing them. The
// __all__ form does not come through here: it reads each listed name with a
// normal attribute load so a missing one raises AttributeError.
func StarLoad(mod objects.Object, name string) (objects.Object, bool) {
	m, ok := mod.(*objects.Module)
	if !ok {
		return nil, false
	}
	return m.Get(name)
}

// StarImportDynamic copies the public names a `from m import *` did not bind
// statically from the source module into the importer, so a name the source
// injected at runtime, the globals().update shape re._constants uses, is
// visible in the importer too. The static names are already bound as module
// variables, so they are skipped; everything else public lands in the
// importer's overflow store, where its module-scope reads find it.
func StarImportDynamic(dst *objects.Module, srcObj objects.Object, static []string) error {
	src, ok := srcObj.(*objects.Module)
	if !ok {
		return nil
	}
	skip := make(map[string]bool, len(static))
	for _, n := range static {
		skip[n] = true
	}
	for _, n := range src.PublicNames() {
		if skip[n] {
			continue
		}
		if v, ok := src.Get(n); ok {
			dst.SetGlobal(n, v)
		}
	}
	return nil
}

// LoadModuleName reads a name the module's compile never saw statically: an
// attribute an importer set on the module object after import. A miss falls
// back to builtins and then raises NameError, the LOAD_GLOBAL order. The
// fallback also consults the live builtins module, so a name injected into it at
// runtime (the _ and gettext family that gettext.install writes into
// builtins.__dict__) resolves from inside an imported module the same way it
// does from the main module, matching CPython's __builtins__ fallback.
func LoadModuleName(m *objects.Module, name string) (objects.Object, error) {
	if m != nil {
		if v, ok := m.Get(name); ok {
			return v, nil
		}
	}
	if f, ok := Builtin(name); ok {
		return f, nil
	}
	if builtinsModule != nil {
		if f, ok := builtinsModule.Get(name); ok {
			return f, nil
		}
	}
	return nil, nameNotDefined(name)
}
