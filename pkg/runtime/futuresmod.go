package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// concurrent.futures is a built-in package: CPython splits its Future and
// executor machinery across concurrent/futures/_base.py and the pool modules,
// but the surface a program imports is the Future class and the three exception
// names, which the runtime provides in Go under the same dotted import. The
// package walks as two entries, concurrent then concurrent.futures, so a plain
// `import concurrent.futures` and a `from concurrent.futures import Future`
// both resolve through the ancestor-first import chain. ThreadPoolExecutor and
// the wait and as_completed module functions are later slices.

func init() {
	moduleTable["concurrent"] = &moduleEntry{builtin: true, pkg: true, exec: initConcurrent}
	moduleTable["concurrent.futures"] = &moduleEntry{builtin: true, pkg: true, exec: initFutures}
}

// initConcurrent runs the concurrent package body. The package is a namespace
// holding the futures submodule, so its own body binds nothing; the submodule
// binds itself on the parent when it imports.
func initConcurrent(m *objects.Module) error {
	return nil
}

func initFutures(m *objects.Module) error {
	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"Future", objects.NewFuncKw("Future", futuresNewFuture)},
		{"CancelledError", objects.CancelledErrorClass()},
		{"InvalidStateError", objects.InvalidStateErrorClass()},
		{"TimeoutError", objects.ExcClass2("TimeoutError")},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	return nil
}

// futuresNewFuture is concurrent.futures.Future(): a fresh pending future. The
// constructor takes no arguments, so any argument is a TypeError the way
// CPython's Future.__init__ rejects extra positional or keyword arguments.
func futuresNewFuture(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 0 {
		return nil, objects.Raise(objects.TypeError, "Future.__init__() takes 1 positional argument but %d were given", len(pos)+1)
	}
	if len(kwNames) != 0 {
		return nil, objects.Raise(objects.TypeError, "Future.__init__() got an unexpected keyword argument '%s'", kwNames[0])
	}
	return objects.NewFuture(), nil
}
