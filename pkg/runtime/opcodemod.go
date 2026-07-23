package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _opcode is the introspection accelerator CPython implements in C; opcode.py
// opens with `import _opcode` and builds its hasarg/hasconst/... tables by
// filtering opmap through the module's has_* predicates. There is no pure-Python
// fallback, so opcode, and through it dis, inspect, dataclasses, traceback,
// unittest, and logging, cannot import until this exists.
//
// The module answers over the fixed 3.14.6 opcode grammar: the has_* predicates
// classify an opcode number, is_valid reports membership, and get_nb_ops,
// get_intrinsic1_descs, get_intrinsic2_descs, and get_special_method_names hand
// back static tables. The classification comes from introspecting the oracle
// _opcode (opcodetables.go), so it is platform stable by construction.
//
// What _opcode does not carry under AOT is stack_effect over live bytecode: the
// compiled world has no code objects to disassemble, so opcode.py imports the
// name but only dis calls it, lazily, on a code object that never exists here.
// It is exposed as a callable that raises when called, the same reduced-surface
// stance marshal and ast.parse take, and enough to unblock the import chain.

func init() {
	moduleTable["_opcode"] = &moduleEntry{builtin: true, exec: initOpcode}
}

func initOpcode(m *objects.Module) error {
	// The seven has_* predicates share one shape: read the opcode number and
	// answer whether it is in the predicate's set.
	for name, set := range opcodePredicates {
		fn := objects.NewFunc(name, 1, func(args []objects.Object) (objects.Object, error) {
			op, ok := objects.AsIntValue(args[0])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "an integer is required")
			}
			return objects.NewBool(set[op]), nil
		})
		if err := objects.StoreAttr(m, name, fn); err != nil {
			return err
		}
	}

	isValid := objects.NewFunc("is_valid", 1, func(args []objects.Object) (objects.Object, error) {
		op, ok := objects.AsIntValue(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "an integer is required")
		}
		return objects.NewBool(opcodeValid[op]), nil
	})
	if err := objects.StoreAttr(m, "is_valid", isValid); err != nil {
		return err
	}

	// The static tables opcode.py reads into _intrinsic_*_descs, _nb_ops, and
	// _special_method_names.
	descs := func(xs []string) objects.Object {
		elts := make([]objects.Object, len(xs))
		for i, s := range xs {
			elts[i] = objects.NewStr(s)
		}
		return objects.NewList(elts)
	}
	nbOps := make([]objects.Object, len(opcodeNbOps))
	for i, p := range opcodeNbOps {
		nbOps[i] = objects.NewTuple([]objects.Object{objects.NewStr(p[0]), objects.NewStr(p[1])})
	}
	statics := []struct {
		name string
		val  objects.Object
	}{
		{"get_intrinsic1_descs", descs(opcodeIntrinsic1Descs)},
		{"get_intrinsic2_descs", descs(opcodeIntrinsic2Descs)},
		{"get_special_method_names", descs(opcodeSpecialMethodNames)},
		{"get_nb_ops", objects.NewList(nbOps)},
	}
	for _, s := range statics {
		val := s.val
		fn := objects.NewFunc(s.name, 0, func([]objects.Object) (objects.Object, error) {
			return val, nil
		})
		if err := objects.StoreAttr(m, s.name, fn); err != nil {
			return err
		}
	}

	// stack_effect and get_executor are present so opcode.py's `from _opcode
	// import stack_effect` and dis.py's `from _opcode import get_executor` bind,
	// but both work over live bytecode that AOT does not produce, so a call raises
	// rather than mis-serve a made-up result. dis reaches them only lazily during
	// a disassembly, on a code object that never exists here, never at import.
	for _, name := range []string{"stack_effect", "get_executor"} {
		fn := objects.NewFunc(name, -1, func([]objects.Object) (objects.Object, error) {
			return nil, objects.Raise("NotImplementedError", "%s is unavailable under ahead-of-time compilation", name)
		})
		if err := objects.StoreAttr(m, name, fn); err != nil {
			return err
		}
	}
	return nil
}
