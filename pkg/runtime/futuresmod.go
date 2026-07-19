package runtime

import (
	"math"
	goruntime "runtime"
	"time"

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
		{"wait", objects.NewFuncKw("wait", futuresWait)},
		{"as_completed", objects.NewFuncKw("as_completed", futuresAsCompleted)},
		{"FIRST_COMPLETED", objects.NewStr("FIRST_COMPLETED")},
		{"FIRST_EXCEPTION", objects.NewStr("FIRST_EXCEPTION")},
		{"ALL_COMPLETED", objects.NewStr("ALL_COMPLETED")},
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

	e := objects.NewExecutor(maxWorkers, prefix)
	if in, ok := vals["initializer"]; ok && in != objects.None {
		if !objects.Callable(in) {
			return nil, objects.Raise(objects.TypeError, "initializer must be a callable")
		}
		initargs := objects.NewTuple(nil)
		if ia, ok := vals["initargs"]; ok && ia != objects.None {
			initargs = ia
		}
		e.SetInitializer(in, initargs)
	}

	return e, nil
}

// futuresWait is concurrent.futures.wait(fs, timeout=None, return_when=ALL_COMPLETED).
// timeout and return_when may be given positionally or by keyword. The first
// positional argument is the required future iterable.
func futuresWait(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("wait", []string{"fs", "timeout", "return_when"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	fs, ok := vals["fs"]
	if !ok {
		return nil, objects.Raise(objects.TypeError, "wait() missing 1 required positional argument: 'fs'")
	}
	hasTimeout, timeout, err := futuresTimeout(vals["timeout"])
	if err != nil {
		return nil, err
	}
	returnWhen := objects.Object(objects.NewStr("ALL_COMPLETED"))
	if rw, ok := vals["return_when"]; ok {
		returnWhen = rw
	}
	return objects.Wait(fs, hasTimeout, timeout, returnWhen)
}

// futuresAsCompleted is concurrent.futures.as_completed(fs, timeout=None). It
// returns the iterator that yields the futures in completion order.
func futuresAsCompleted(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("as_completed", []string{"fs", "timeout"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	fs, ok := vals["fs"]
	if !ok {
		return nil, objects.Raise(objects.TypeError, "as_completed() missing 1 required positional argument: 'fs'")
	}
	hasTimeout, timeout, err := futuresTimeout(vals["timeout"])
	if err != nil {
		return nil, err
	}
	return objects.AsCompleted(fs, hasTimeout, timeout)
}

// bindArgs maps positional and keyword arguments onto a parameter list, raising
// the TypeErrors CPython raises for too many positionals, an unknown keyword, or
// a keyword that repeats a value already given positionally.
func bindArgs(fn string, params []string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (map[string]objects.Object, error) {
	vals := map[string]objects.Object{}
	if len(pos) > len(params) {
		return nil, objects.Raise(objects.TypeError, "%s() takes from 1 to %d positional arguments but %d were given", fn, len(params), len(pos))
	}
	for i, v := range pos {
		vals[params[i]] = v
	}
	known := map[string]bool{}
	for _, p := range params {
		known[p] = true
	}
	for i, k := range kwNames {
		if !known[k] {
			return nil, objects.Raise(objects.TypeError, "%s() got an unexpected keyword argument '%s'", fn, k)
		}
		if _, dup := vals[k]; dup {
			return nil, objects.Raise(objects.TypeError, "%s() got multiple values for argument '%s'", fn, k)
		}
		vals[k] = kwVals[i]
	}
	return vals, nil
}

// futuresTimeout reads a timeout argument shared by wait and as_completed. None
// or a missing argument means no deadline; a number sets one, with a non-positive
// value meaning an immediate check. A non-numeric timeout is the TypeError the
// future's own timeout parsing raises.
func futuresTimeout(v objects.Object) (bool, time.Duration, error) {
	if v == nil || v == objects.None {
		return false, 0, nil
	}
	f, ok := objects.AsFloat(v)
	if !ok {
		return false, 0, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as a float", v.TypeName())
	}
	d := max(time.Duration(f*float64(time.Second)), 0)
	return true, d, nil
}
