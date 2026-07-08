package objects

// Module is a Python module object. Its namespace is split in two: slots are
// live pointers into the generated Go package's module-scope variables, bound
// by the module's Exec before the body runs, so an attribute read or write
// from outside and a global read inside the module body see the same storage.
// extra holds everything else: the identity attributes seeded at creation and
// any new name assigned from outside, in insertion order. A nil slot value
// means the name is currently unbound, the state before its first assignment
// and after a del.
type Module struct {
	name       string
	file       string
	slots      map[string]*Object
	slotOrder  []string
	extra      map[string]Object
	extraOrder []string
	// globalsView is the single dict globals() hands back, built on first call
	// and re-synced to the live slots and overflow store on every call so its
	// identity is stable (globals() is globals()) while its contents stay
	// current, the way CPython returns the one module __dict__.
	globalsView *dictObject
	// initializing is true while the body runs; a partial module reached
	// through an import cycle reports missing attributes with the
	// consider-renaming hint the way CPython does for a script-adjacent file.
	initializing bool
}

func (*Module) TypeName() string { return "module" }

// NewModule builds an empty module with its identity attributes seeded the
// way CPython's module_from_spec does, so m.__name__ and m.__file__ read back
// and stay overwritable like ordinary attributes.
func NewModule(name, file string) *Module {
	m := &Module{name: name, file: file, slots: map[string]*Object{}, extra: map[string]Object{}}
	m.setExtra("__name__", NewStr(name))
	m.setExtra("__file__", NewStr(file))
	// __doc__ starts None, CPython's default for a module with no docstring.
	// A module with a body overwrites the slot the moment its synthetic
	// __doc__ assignment runs; a bodyless namespace package keeps this None.
	m.setExtra("__doc__", None)
	return m
}

// NewBuiltinModule builds a module the runtime provides itself, like sys.
// A built-in module has no source file, so __file__ is never seeded and
// reading it raises AttributeError the way CPython's sys.__file__ does;
// __package__ is the empty string since built-ins live at top level.
func NewBuiltinModule(name string) *Module {
	m := &Module{name: name, slots: map[string]*Object{}, extra: map[string]Object{}}
	m.setExtra("__name__", NewStr(name))
	m.setExtra("__doc__", None)
	m.setExtra("__package__", NewStr(""))
	return m
}

// Name is the module's import name, for error messages.
func (m *Module) Name() string { return m.name }

// File is the source path the module was compiled from.
func (m *Module) File() string { return m.file }

// Bind registers one module-scope name as a live slot over the generated
// package variable holding it. Exec calls it for every module-scope binding
// before the body runs, which is what gives an import cycle CPython's
// partial-module view.
func (m *Module) Bind(name string, slot *Object) {
	if _, ok := m.slots[name]; !ok {
		m.slotOrder = append(m.slotOrder, name)
	}
	m.slots[name] = slot
}

// StartInit and FinishInit bracket the body run for the partial-module
// attribute wording; Initializing reports that state so from-import misses
// can pick their wording too.
func (m *Module) StartInit()         { m.initializing = true }
func (m *Module) FinishInit()        { m.initializing = false }
func (m *Module) Initializing() bool { return m.initializing }

func (m *Module) setExtra(name string, v Object) {
	if _, ok := m.extra[name]; !ok {
		m.extraOrder = append(m.extraOrder, name)
	}
	m.extra[name] = v
}

// Get reads one attribute, reporting whether it is currently bound.
func (m *Module) Get(name string) (Object, bool) {
	if slot, ok := m.slots[name]; ok && *slot != nil {
		return *slot, true
	}
	if v, ok := m.extra[name]; ok {
		return v, true
	}
	return nil, false
}

// GlobalsDict returns the module namespace as a dict, the value Python's
// globals() builtin gives. unagi keeps module globals in live Go variables
// rather than one dict object, so the returned dict is seeded with the names
// bound when it is called: reads, iteration, and membership match CPython, and
// type(globals()) is dict holds because the result is an ordinary dict. It is
// tied to the module through owner, so a write back to it, globals()[name] =
// value or globals().update(...), carries into the module storage and a later
// module-scope read finds the injected name. Order is the identity attributes
// first, then the module-scope names in the order they were bound; a name held
// in both a slot and the overflow store keeps its live slot value.
func (m *Module) GlobalsDict() Object {
	if m.globalsView == nil {
		m.globalsView = &dictObject{index: map[string]int{}}
	}
	d := m.globalsView
	// Re-sync the cached dict to the current namespace in place, keeping its
	// identity. Detaching owner first means the seeding writes nothing back; the
	// names already live in the module, and a mirror would only echo them.
	d.owner = nil
	d.entries = d.entries[:0]
	d.index = map[string]int{}
	for _, n := range m.extraOrder {
		_ = d.set(NewStr(n), m.extra[n])
	}
	for _, n := range m.slotOrder {
		if slot := m.slots[n]; slot != nil && *slot != nil {
			_ = d.set(NewStr(n), *slot)
		}
	}
	// From here a write to the dict carries into the module storage.
	d.owner = m
	return d
}

// missingAttr is the AttributeError for a read of an unbound module name. A
// module still executing its body reports the consider-renaming hint, the
// message CPython 3.14 gives when the unfinished module is a script-adjacent
// file, which is the only kind this tier compiles.
func (m *Module) missingAttr(name string) error {
	if m.initializing {
		return Raise(AttributeError,
			"module '%s' has no attribute '%s' (consider renaming '%s' if it has the same name as a library you intended to import)",
			m.name, name, m.file)
	}
	return Raise(AttributeError, "module '%s' has no attribute '%s'", m.name, name)
}

// moduleLoadAttr is the LoadAttr arm for modules.
func moduleLoadAttr(m *Module, name string) (Object, error) {
	if v, ok := m.Get(name); ok {
		return v, nil
	}
	return nil, m.missingAttr(name)
}

// moduleStoreAttr writes through a slot when the name is module scope in the
// generated code, so functions inside the module observe the new binding;
// any other name lands in the overflow store.
func moduleStoreAttr(m *Module, name string, val Object) error {
	if slot, ok := m.slots[name]; ok {
		*slot = val
		return nil
	}
	m.setExtra(name, val)
	return nil
}

// moduleDelAttr unbinds an attribute. Deleting an unbound name gets the
// generic object wording, not the module-naming one, matching 3.14.
func moduleDelAttr(m *Module, name string) error {
	if slot, ok := m.slots[name]; ok && *slot != nil {
		*slot = nil
		return nil
	}
	if _, ok := m.extra[name]; ok {
		delete(m.extra, name)
		for i, n := range m.extraOrder {
			if n == name {
				m.extraOrder = append(m.extraOrder[:i], m.extraOrder[i+1:]...)
				break
			}
		}
		return nil
	}
	return Raise(AttributeError, "'module' object has no attribute '%s'", name)
}
