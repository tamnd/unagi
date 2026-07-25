package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _symtable is the C accelerator symtable.py opens with `import _symtable` and
// `from _symtable import (USE, DEF_GLOBAL, ...)` at module scope. The module was
// missing, so `import symtable` failed with ModuleNotFoundError.
//
// The module ships two things: a block of integer flag constants (the symbol
// USE/DEF_* bits, the SCOPE_OFF/SCOPE_MASK packing, the FREE/LOCAL/CELL scope
// codes, and the TYPE_* block kinds) and one function, symtable(source, filename,
// compile_type), which compiles source and returns the top symbol-table block.
//
// symtable.py reads only the constants at import; it calls _symtable.symtable
// lazily inside its own symtable() wrapper. So the constants alone unblock the
// import. The symtable() function itself compiles a source string into a symbol
// table, which requires parsing that source at runtime. An AOT unagi program is
// compiled Go with no runtime Python parser, the same reason _ast carries no
// parser, marshal is reduced-surface, and compile/exec/eval do not exist here.
// So symtable() is provided but raises at the call rather than fabricating a
// table, and every value it would return is honest: `import symtable`, the
// SymbolTable class hierarchy, and the flag constants are all real, and the one
// operation that needs a source parser fails cleanly with a message pointing at
// the reason.
//
// The constant values are CPython 3.14.6's, the interpreter the conformance
// harness runs, and are fixed by the C symtable implementation, so they are
// platform-stable by construction.

func init() {
	moduleTable["_symtable"] = &moduleEntry{builtin: true, exec: initSymtable}
}

func initSymtable(m *objects.Module) error {
	consts := []struct {
		name string
		val  int64
	}{
		// Symbol-use and definition bits (Include/internal/pycore_symtable.h).
		{"USE", 16},
		{"DEF_GLOBAL", 1},
		{"DEF_NONLOCAL", 8},
		{"DEF_LOCAL", 2},
		{"DEF_PARAM", 4},
		{"DEF_TYPE_PARAM", 1024},
		{"DEF_FREE_CLASS", 64},
		{"DEF_IMPORT", 128},
		{"DEF_BOUND", 134},
		{"DEF_ANNOT", 256},
		{"DEF_COMP_ITER", 512},
		{"DEF_COMP_CELL", 2048},
		// Scope is packed into flags at SCOPE_OFF, SCOPE_MASK wide.
		{"SCOPE_OFF", 12},
		{"SCOPE_MASK", 15},
		// Scope codes stored in those bits.
		{"FREE", 4},
		{"LOCAL", 1},
		{"GLOBAL_IMPLICIT", 3},
		{"GLOBAL_EXPLICIT", 2},
		{"CELL", 5},
		// Block kinds a table's .type reports.
		{"TYPE_FUNCTION", 0},
		{"TYPE_CLASS", 1},
		{"TYPE_MODULE", 2},
		{"TYPE_ANNOTATION", 3},
		{"TYPE_TYPE_ALIAS", 4},
		{"TYPE_TYPE_PARAMETERS", 5},
		{"TYPE_TYPE_VARIABLE", 6},
	}
	for _, c := range consts {
		if err := objects.StoreAttr(m, c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}

	// symtable(source, filename, compile_type) compiles source to a symbol table.
	// Building one means parsing the source, which the AOT runtime cannot do (no
	// runtime Python parser, the same wall as compile/exec and _ast.parse), so it
	// raises rather than returning a fabricated table.
	fn := objects.NewFuncKw("symtable", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		return nil, objects.Raise("NotImplementedError",
			"_symtable.symtable needs to parse source at runtime, which this build does not carry (no runtime Python parser, the same reason compile/exec and ast.parse are unavailable)")
	})
	return objects.StoreAttr(m, "symtable", fn)
}
