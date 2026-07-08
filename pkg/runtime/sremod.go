package runtime

import (
	"unicode"

	"github.com/tamnd/unagi/pkg/objects"
)

// _sre is a built-in module, the C accelerator behind the pure-Python re
// package. re parses and compiles a pattern into the SRE bytecode in
// re._parser and re._compiler, then hands the finished bytecode to
// _sre.compile, which builds the re.Pattern the matcher runs against. This
// slice lands compile and the compiled-pattern object; the matcher follows.

func init() {
	moduleTable["_sre"] = &moduleEntry{builtin: true, exec: initSre}

	// re.sub compiles a string replacement into a template through the re
	// package's own _compile_template, which runs _parser.parse_template and
	// _sre.template. Wiring it here lets the native Pattern.sub reach that
	// pipeline; a Pattern only exists after re has imported, so the hook is
	// always in place by the time a substitution runs.
	objects.CompileReTemplate = func(pattern, repl objects.Object) (objects.Object, error) {
		re, err := ImportModule("re")
		if err != nil {
			return nil, err
		}
		fn, err := objects.LoadAttr(re, "_compile_template")
		if err != nil {
			return nil, err
		}
		return objects.Call(fn, []objects.Object{pattern, repl})
	}
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
	// The case helpers re._compiler reads when it optimises a case-insensitive
	// literal prefix. iscased reports whether a code point has a case pairing and
	// tolower folds it, in the unicode and ascii variants the IGNORECASE and
	// ASCII flags select. They fold the same way the matcher does, so a prefix the
	// compiler simplifies stays consistent with the run.
	for name, fn := range map[string]func(int32) objects.Object{
		"unicode_iscased": func(ch int32) objects.Object { return boolObj(uniIscased(ch)) },
		"ascii_iscased":   func(ch int32) objects.Object { return boolObj(asciiIscased(ch)) },
		"unicode_tolower": func(ch int32) objects.Object { return objects.NewInt(int64(uniTolower(ch))) },
		"ascii_tolower":   func(ch int32) objects.Object { return objects.NewInt(int64(asciiTolower(ch))) },
	} {
		f := objects.NewFunc(name, 1, func(a []objects.Object) (objects.Object, error) {
			ch, ok := objects.AsInt(a[0])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "an integer is required")
			}
			return fn(int32(ch)), nil
		})
		if err := set(name, f); err != nil {
			return err
		}
	}

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

	// template(pattern, template): build the compiled replacement re._compiler
	// hands sub. The second argument is the flat literal-and-index list
	// _parser.parse_template produced, with every group index already checked
	// against the pattern, so the template just carries it for expansion.
	tmpl := objects.NewFunc("template", -1, func(a []objects.Object) (objects.Object, error) {
		if len(a) != 2 {
			return nil, objects.Raise(objects.TypeError,
				"template() takes exactly 2 arguments (%d given)", len(a))
		}
		items, err := materialize(a[1])
		if err != nil {
			return nil, objects.Raise(objects.TypeError, "template argument must be a list")
		}
		return objects.NewReTemplate(items), nil
	})
	if err := set("template", tmpl); err != nil {
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
		v, ok := objects.AsIntValue(e)
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

// boolObj wraps a Go bool as the interned True or False.
func boolObj(b bool) objects.Object {
	if b {
		return objects.True
	}
	return objects.False
}

// uniTolower folds a code point to lower case the Unicode way, matching the
// engine's Py_UNICODE_TOLOWER path so a compiler-simplified prefix stays in step
// with the matcher.
func uniTolower(ch int32) int32 { return int32(unicode.ToLower(rune(ch))) }

// asciiTolower folds only the ASCII letters, leaving every other code point be,
// the fold the ASCII flag selects.
func asciiTolower(ch int32) int32 {
	if ch >= 'A' && ch <= 'Z' {
		return ch + ('a' - 'A')
	}
	return ch
}

// uniIscased reports whether a code point has a case pairing, so folding it can
// change it: CPython's SRE_UNI_IS_CASED is true when the lower or the upper form
// differs from the character.
func uniIscased(ch int32) bool {
	r := rune(ch)
	return unicode.ToLower(r) != r || unicode.ToUpper(r) != r
}

// asciiIscased reports whether a code point is an ASCII letter, the only cased
// characters under the ASCII flag.
func asciiIscased(ch int32) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
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
