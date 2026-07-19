package objects

// The object-reduction protocol pickles a value CPython cannot encode
// structurally by recording how to rebuild it: a callable found by qualified
// name (GLOBAL / STACK_GLOBAL) applied to an argument tuple (REDUCE). This is
// how a set or a frozenset pickles under protocols 2 and 3, which have no
// EMPTY_SET / FROZENSET opcodes — `builtins.set` (or `builtins.frozenset`)
// applied to a list of the elements in set-iteration order. The same machinery
// will carry user-defined classes and functions in later slices; this slice
// wires it to the two set types so protocols 2 and 3 stop refusing them.

// Reduction opcodes, spelled as CPython's pickle module names them.
const (
	opGlobal      = 'c'  // 0x63 push a global by newline-terminated module then qualname
	opStackGlobal = 0x93 // push a global whose module and qualname are on the stack (protocol 4+)
	opReduce      = 'R'  // 0x52 apply the callable below the argument tuple, replacing both
)

// compatReverseImport mirrors the entries of _compat_pickle.REVERSE_IMPORT_MAPPING
// that unagi pickles through reduction. CPython applies it only when fix_imports
// is on, which it gates to protocols below 3, so a Python-2 module name lands in
// a protocol-2 pickle and the modern name in protocol 3.
var compatReverseImport = map[string]string{
	"builtins": "__builtin__",
}

// compatForwardImport is the load-side inverse: a Python-2 module name in an old
// pickle maps back to its Python-3 name so find_class resolves the same global.
var compatForwardImport = map[string]string{
	"__builtin__": "builtins",
}

// pickleGlobalRef is the memo key and loader stand-in for a callable pickled by
// qualified name. On the dump side it gives the GLOBAL a stable identity so a
// second reference fetches it back from the memo instead of rewriting it; on the
// load side it carries the resolved module and qualname to REDUCE.
type pickleGlobalRef struct {
	module   string
	qualname string
}

func (*pickleGlobalRef) TypeName() string { return "type" }

// globalRef interns one pickleGlobalRef per qualified name for this pickle, so
// the same global used twice shares an identity and the memo can fetch it back.
func (p *pickler) globalRef(module, qualname string) *pickleGlobalRef {
	key := module + "\x00" + qualname
	if p.globals == nil {
		p.globals = map[string]*pickleGlobalRef{}
	}
	if r, ok := p.globals[key]; ok {
		return r
	}
	r := &pickleGlobalRef{module: module, qualname: qualname}
	p.globals[key] = r
	return r
}

// moduleNameObj returns the interned strObject for a module name, so every global
// referencing that module saves the same object and the pickler's identity-keyed
// memo fetches it back after the first, matching CPython's shared __module__.
func (p *pickler) moduleNameObj(module string) *strObject {
	if p.moduleNames == nil {
		p.moduleNames = map[string]*strObject{}
	}
	if s, ok := p.moduleNames[module]; ok {
		return s
	}
	s := NewStr(module).(*strObject)
	p.moduleNames[module] = s
	return s
}

// saveGlobal writes a reference to a callable by its module and qualname. Under
// protocol 4+ the two names go out as memoized strings followed by STACK_GLOBAL;
// under protocols 2 and 3 they go out as the newline-terminated GLOBAL body,
// with the module name run through the reverse import map for protocol 2 (where
// fix_imports is on) so `builtins` becomes `__builtin__`. The global is memoized
// either way, so a repeated reference fetches it back.
func (p *pickler) saveGlobal(module, qualname string) error {
	ref := p.globalRef(module, qualname)
	if p.memoGet(ref) {
		return nil
	}
	if p.proto >= 4 {
		// The module name is memoized by identity in CPython, and every class in a
		// module shares one __module__ string object, so a second class in the same
		// module fetches the name from the memo instead of re-emitting it. Interning
		// the module strObject per pickle reproduces that shared identity; the
		// qualname is a per-class object, so it is written fresh each time.
		if err := p.saveStr(module, p.moduleNameObj(module)); err != nil {
			return err
		}
		if err := p.saveStr(qualname, NewStr(qualname)); err != nil {
			return err
		}
		p.framer.write(opStackGlobal)
		p.memoize(ref)
		return nil
	}
	m := module
	if p.proto < 3 {
		if mapped, ok := compatReverseImport[m]; ok {
			m = mapped
		}
	}
	p.framer.write(opGlobal)
	p.framer.write([]byte(m)...)
	p.framer.write('\n')
	p.framer.write([]byte(qualname)...)
	p.framer.write('\n')
	p.memoize(ref)
	return nil
}

// saveReduce writes the reduction of o: the callable found at module.qualname
// applied to args. It mirrors CPython's save_reduce for the plain
// (callable, args) case — save the global, save the argument tuple, emit REDUCE,
// then memoize the result so a later reference to o fetches it back.
func (p *pickler) saveReduce(module, qualname string, args []Object, o Object) error {
	if err := p.saveGlobal(module, qualname); err != nil {
		return err
	}
	if err := p.save(NewTuple(args)); err != nil {
		return err
	}
	p.framer.write(opReduce)
	p.memoize(o)
	return nil
}

// reduceGlobal rebuilds the object a REDUCE opcode describes: it applies the
// resolved global to its argument tuple. This slice resolves the two set types
// pickled by the protocol-2/3 set reducers; any other global is an
// UnpicklingError until the slice that backs it lands.
func reduceGlobal(ref *pickleGlobalRef, args *tupleObject) (Object, error) {
	switch ref.module + "." + ref.qualname {
	case "builtins.set":
		elts, err := reduceIterableArg(ref, args)
		if err != nil {
			return nil, err
		}
		return NewSet(elts)
	case "builtins.frozenset":
		elts, err := reduceIterableArg(ref, args)
		if err != nil {
			return nil, err
		}
		return NewFrozenset(elts)
	}
	return nil, newUnpicklingError("cannot unpickle global %s.%s", ref.module, ref.qualname)
}

// reduceIterableArg unpacks the single-list argument tuple the set and frozenset
// reducers emit, returning the list's elements for reconstruction.
func reduceIterableArg(ref *pickleGlobalRef, args *tupleObject) ([]Object, error) {
	if len(args.elts) != 1 {
		return nil, newUnpicklingError("%s.%s expects one argument, got %d", ref.module, ref.qualname, len(args.elts))
	}
	lst, ok := args.elts[0].(*listObject)
	if !ok {
		return nil, newUnpicklingError("%s.%s expects a list argument, got %s", ref.module, ref.qualname, args.elts[0].TypeName())
	}
	return lst.elts, nil
}
