package runtime

import "github.com/tamnd/unagi/pkg/objects"

// The import table. The generated modtable.go registers every sibling module
// the build compiled, keyed by import name; ImportModule executes a body at
// most once and every later import gets the same module object, the
// sys.modules behavior without sys itself yet.

// moduleEntry is one compiled module: its source path, its Exec, and the
// module object once the first import created it.
type moduleEntry struct {
	file string
	exec func(*objects.Module) error
	mod  *objects.Module
}

var moduleTable = map[string]*moduleEntry{}

// RegisterModule adds one compiled module to the import table. The generated
// modtable.go calls it from init for every module in the program.
func RegisterModule(name, file string, exec func(*objects.Module) error) {
	moduleTable[name] = &moduleEntry{file: file, exec: exec}
}

// ImportModule is the import statement: return the already-created module, or
// create it, publish it, and run its body. The module object is in the table
// before the body starts, CPython's insert-before-exec order, so a circular
// import sees the partial module rather than recursing; a body that raises
// removes the entry again so a later import retries from scratch.
func ImportModule(name string) (objects.Object, error) {
	ent, ok := moduleTable[name]
	if !ok {
		return nil, objects.Raise(objects.ModuleNotFoundError, "No module named '%s'", name)
	}
	if ent.mod != nil {
		return ent.mod, nil
	}
	m := objects.NewModule(name, ent.file)
	ent.mod = m
	m.StartInit()
	if err := ent.exec(m); err != nil {
		ent.mod = nil
		return nil, err
	}
	m.FinishInit()
	return m, nil
}

// ImportFrom is `from module import name`: import the module, then read one
// attribute off it. A miss raises ImportError with the module's path, or with
// the consider-renaming hint while the module body is still running, both
// wordings probed on 3.14 with script-adjacent files.
func ImportFrom(module, name string) (objects.Object, error) {
	mo, err := ImportModule(module)
	if err != nil {
		return nil, err
	}
	m := mo.(*objects.Module)
	if v, ok := m.Get(name); ok {
		return v, nil
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
