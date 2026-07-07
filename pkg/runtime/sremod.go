package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _sre is a built-in module, the C accelerator behind the pure-Python re
// package. re parses and compiles a pattern into the SRE bytecode in
// re._parser and re._compiler, then hands the finished bytecode to
// _sre.compile, which builds the re.Pattern the matcher runs against. This
// slice lands compile and the compiled-pattern object; the matcher follows.

func init() {
	moduleTable["_sre"] = &moduleEntry{builtin: true, exec: initSre}
}

func initSre(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	// The engine identifiers and limits the Python layer reads: MAGIC stamps the
	// bytecode version re._compiler emits, CODESIZE is the width of a bytecode
	// word, MAXREPEAT is the unbounded-count sentinel, and MAXGROUPS caps the
	// group count.
	for name, v := range map[string]int64{
		"MAGIC":     objects.SreMagic,
		"CODESIZE":  objects.SreCodeSize,
		"MAXREPEAT": objects.SreMaxRepeat,
		"MAXGROUPS": objects.SreMaxGroups,
	} {
		if err := set(name, objects.NewInt(v)); err != nil {
			return err
		}
	}

	// compile(pattern, flags, code, groups, groupindex, indexgroup): build a
	// compiled pattern from the bytecode re._compiler produced. pattern is the
	// source str, bytes, or None kept for repr; flags is the SRE flag bits; code
	// is the bytecode list; groups is the capture-group count; groupindex maps
	// each named group to its number and indexgroup maps each number back to its
	// name.
	compile := objects.NewFunc("compile", -1, func(a []objects.Object) (objects.Object, error) {
		if len(a) > 6 {
			return nil, objects.Raise(objects.TypeError,
				"compile() takes at most 6 arguments (%d given)", len(a))
		}
		if len(a) < 6 {
			return nil, objects.Raise(objects.TypeError,
				"compile() takes exactly 6 arguments (%d given)", len(a))
		}

		pattern := a[0]
		isbytes := false
		switch {
		case isStr(pattern):
			isbytes = false
		case isBytesPattern(pattern):
			isbytes = true
		case pattern == objects.None:
			isbytes = false
		default:
			return nil, objects.Raise(objects.TypeError,
				"first argument must be string or compiled pattern")
		}

		flags, ok := objects.AsInt(a[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError,
				"compile() argument 'flags' must be int, not %s", a[1].TypeName())
		}

		code, err := decodeCode(a[2])
		if err != nil {
			return nil, err
		}

		groups, ok := objects.AsInt(a[3])
		if !ok {
			return nil, objects.Raise(objects.TypeError,
				"compile() argument 'groups' must be int, not %s", a[3].TypeName())
		}

		groupindex := a[4]
		indexgroup := a[5]
		if indexgroup.TypeName() != "tuple" {
			return nil, objects.Raise(objects.TypeError,
				"compile() argument 'indexgroup' must be tuple, not %s", indexgroup.TypeName())
		}

		return objects.NewPattern(pattern, uint32(flags), code, int(groups),
			groupindex, indexgroup, isbytes), nil
	})
	if err := set("compile", compile); err != nil {
		return err
	}

	return nil
}

// decodeCode reads the bytecode list _compiler.py passes as the code argument
// into a slice of engine words. Each element is a non-negative int that fits in
// a bytecode word, the same range CPython's compile checks.
func decodeCode(o objects.Object) ([]uint32, error) {
	elts, err := materialize(o)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "code argument must be a list")
	}
	out := make([]uint32, len(elts))
	for i, e := range elts {
		v, ok := objects.AsInt(e)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "code[%d] must be int", i)
		}
		if v < 0 || v > 0xFFFFFFFF {
			return nil, objects.Raise(objects.OverflowError, "code[%d]=%d out of range", i, v)
		}
		out[i] = uint32(v)
	}
	return out, nil
}

// isStr reports whether o is a str object.
func isStr(o objects.Object) bool {
	_, ok := objects.AsStr(o)
	return ok
}

// isBytesPattern reports whether o is a bytes object, the form a bytes pattern
// arrives as.
func isBytesPattern(o objects.Object) bool {
	_, ok := objects.AsBytes(o)
	return ok
}
