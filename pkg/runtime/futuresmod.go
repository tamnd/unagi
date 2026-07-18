package runtime

import (
	"math"
	goruntime "runtime"

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
		{"ThreadPoolExecutor", objects.NewFuncKw("ThreadPoolExecutor", futuresNewThreadPool)},
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

// futuresNewThreadPool is concurrent.futures.ThreadPoolExecutor(max_workers=None,
// thread_name_prefix=”, initializer=None, initargs=()). A None max_workers takes
// CPython's default, min(32, cpu_count + 4); any positive number is accepted, an
// integer or a float, since CPython only checks it is greater than zero. A worker
// cap of zero or below raises ValueError, and a non-numeric cap raises the same
// TypeError CPython's `max_workers <= 0` comparison does. initializer is a later
// slice, so a non-None one is refused the way the Thread group argument is.
func futuresNewThreadPool(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	params := []string{"max_workers", "thread_name_prefix", "initializer", "initargs"}
	vals := map[string]objects.Object{}
	if len(pos) > len(params) {
		return nil, objects.Raise(objects.TypeError, "__init__() takes from 1 to %d positional arguments but %d were given", len(params)+1, len(pos)+1)
	}
	for i, v := range pos {
		vals[params[i]] = v
	}
	known := map[string]bool{"max_workers": true, "thread_name_prefix": true, "initializer": true, "initargs": true}
	for i, k := range kwNames {
		if !known[k] {
			return nil, objects.Raise(objects.TypeError, "__init__() got an unexpected keyword argument '%s'", k)
		}
		if _, dup := vals[k]; dup {
			return nil, objects.Raise(objects.TypeError, "__init__() got multiple values for argument '%s'", k)
		}
		vals[k] = kwVals[i]
	}

	maxWorkers := min(32, goruntime.NumCPU()+4)
	if mw, ok := vals["max_workers"]; ok && mw != objects.None {
		f, ok := objects.AsFloat(mw)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'<=' not supported between instances of '%s' and 'int'", mw.TypeName())
		}
		if f <= 0 {
			return nil, objects.Raise(objects.ValueError, "max_workers must be greater than 0")
		}
		// CPython keeps the number as given and spawns a worker while the live
		// count is below it, so a fractional cap allows the next whole worker up;
		// ceil reproduces that count without ever rounding a positive cap to zero.
		maxWorkers = int(math.Ceil(f))
	}

	prefix := ""
	if p, ok := vals["thread_name_prefix"]; ok && p != objects.None {
		s, ok := objects.AsStr(p)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "thread_name_prefix must be a string or None")
		}
		prefix = s
	}

	if in, ok := vals["initializer"]; ok && in != objects.None {
		return nil, objects.Raise("AssertionError", "initializer argument must be None for now")
	}

	return objects.NewExecutor(maxWorkers, prefix), nil
}
