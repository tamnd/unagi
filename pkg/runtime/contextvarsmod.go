package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// contextvars is a built-in module: CPython carries a per-thread context stack
// in C, and the runtime keeps the current context on the Thread the call spine
// already threads (spec 2076 doc 10). This slice exposes ContextVar with
// get/set/reset, copy_context, Context with run, and Token for the reset
// receipt. Context's mapping surface (iteration, len, subscript) is a later
// slice.

func init() {
	moduleTable["contextvars"] = &moduleEntry{builtin: true, exec: initContextvars}
}

func initContextvars(m *objects.Module) error {
	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"ContextVar", objects.NewFuncKw("ContextVar", contextvarsContextVar)},
		{"copy_context", objects.NewFuncT("copy_context", 0, contextvarsCopyContext)},
		{"Context", objects.NewFunc("Context", 0, contextvarsContext)},
		{"Token", objects.ContextTokenClass()},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	return nil
}

// contextvarsContextVar implements contextvars.ContextVar(name, *, default).
// The name is a required positional string; default is keyword-only and, when
// absent, leaves the variable with no default so get raises without one.
func contextvarsContextVar(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "ContextVar() takes exactly 1 positional argument (%d given)", len(pos))
	}
	name, ok := objects.AsStr(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "context variable name must be a str")
	}
	hasDefault := false
	var def objects.Object
	for i, kn := range kwNames {
		switch kn {
		case "default":
			hasDefault = true
			def = kwVals[i]
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for ContextVar()", kn)
		}
	}
	return objects.NewContextVar(name, hasDefault, def), nil
}

// contextvarsCopyContext implements contextvars.copy_context(): a shallow copy
// of the running thread's current context.
func contextvarsCopyContext(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "copy_context() takes no arguments (%d given)", len(args))
	}
	return objects.CopyThreadContext(t), nil
}

// contextvarsContext implements contextvars.Context(): a fresh context with no
// variables set.
func contextvarsContext(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "Context() takes no arguments (%d given)", len(args))
	}
	return objects.NewEmptyContext(), nil
}
