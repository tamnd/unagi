package runtime

import "github.com/tamnd/unagi/pkg/objects"

// CPython 3.14 deprecates bitwise inversion of a bool: `~True` still returns the
// bitwise inversion of the underlying int (-2), but warns that this is rarely
// what a `~` on a bool is meant to do. The message is verbatim from CPython so a
// caller filtering or matching on it — unittest.assertWarns, a warnings filter —
// sees exactly the text it does there. Removed in 3.16.
const invertBoolDeprecationMsg = "Bitwise inversion '~' on bool is deprecated and will be removed in Python 3.16. This returns the bitwise inversion of the underlying int object and is usually not what you expect from negating a bool. Use the 'not' operator for boolean negation or ~int(x) if you really want the bitwise inversion of the underlying int."

func init() {
	objects.InvertBoolWarnHook = invertBoolWarn
}

// invertBoolWarn raises the ~bool DeprecationWarning through the public warnings
// module, so it flows through the same filters and catch_warnings capture as any
// other warning. stacklevel=2 steps past this hook frame to point at the code
// that wrote the `~`. The main thread stands in for the current one, matching the
// t-less Invert entry point; a warning promoted to an error by the active filter
// comes back as the error that aborts the operation.
//
// The warning is best-effort: unagi bundles only the modules a program imports,
// so a script that never touches warnings has no warnings module to route
// through. There the deprecation is silently skipped — the ~ still yields the
// right int — while any program that observes warnings (unittest.assertWarns, an
// explicit filter, the test suite) has imported it and sees the warning fire.
func invertBoolWarn() error {
	w, err := ImportModule("warnings")
	if err != nil {
		if isModuleNotFound(err) {
			return nil
		}
		return err
	}
	fn, err := objects.LoadAttr(w, "warn")
	if err != nil {
		return err
	}
	cat, ok := objects.ExcClass("DeprecationWarning")
	if !ok {
		return objects.Raise(objects.RuntimeError, "DeprecationWarning class unavailable")
	}
	_, err = objects.CallKwT(objects.MainThread(), fn,
		[]objects.Object{objects.NewStr(invertBoolDeprecationMsg), cat},
		[]string{"stacklevel"}, []objects.Object{objects.NewInt(2)})
	return err
}

// isModuleNotFound reports whether err is a ModuleNotFoundError (or its
// ImportError base), the signal that the warnings module was never bundled into
// this program because nothing imported it.
func isModuleNotFound(err error) bool {
	exc, ok := err.(*objects.Exception)
	return ok && (exc.Kind == objects.ModuleNotFoundError || exc.Kind == objects.ImportError)
}
