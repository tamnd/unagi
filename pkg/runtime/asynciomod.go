package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// asyncio is a built-in module: CPython implements it across the asyncio package
// over a selector-driven loop, and the runtime provides the same surface in Go
// over the frame machinery of pkg/objects (spec 2076 doc 10 §6). This slice
// exposes run and sleep plus the two loop accessors; tasks, gather, and the
// user-facing Future are later slices.

func init() {
	moduleTable["asyncio"] = &moduleEntry{builtin: true, exec: initAsyncio}
}

func initAsyncio(m *objects.Module) error {
	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"run", objects.NewFuncKw("run", asyncioRun)},
		{"sleep", objects.NewFuncKw("sleep", asyncioSleep)},
		{"create_task", objects.NewFuncKw("create_task", asyncioCreateTask)},
		{"gather", objects.NewFuncKw("gather", asyncioGather)},
		{"get_running_loop", objects.NewFunc("get_running_loop", 0, asyncioGetRunningLoop)},
		{"get_event_loop", objects.NewFunc("get_event_loop", 0, asyncioGetEventLoop)},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	return nil
}

// asyncioRun is asyncio.run(main, *, debug=None). It drives the coroutine to
// completion on a fresh loop and returns its result. The debug flag is accepted
// but not yet acted on; any other keyword is the TypeError CPython raises.
func asyncioRun(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "run() takes 1 positional argument but %d were given", len(pos))
	}
	for _, k := range kwNames {
		if k != "debug" && k != "loop_factory" {
			return nil, objects.Raise(objects.TypeError, "run() got an unexpected keyword argument '%s'", k)
		}
	}
	return objects.AsyncioRun(pos[0])
}

// asyncioSleep is asyncio.sleep(delay, result=None). delay is a number of
// seconds; a non-numeric delay is the TypeError CPython's arithmetic on it
// raises. result defaults to None and is returned when the sleep completes.
func asyncioSleep(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("sleep", []string{"delay", "result"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	delayObj, ok := vals["delay"]
	if !ok {
		return nil, objects.Raise(objects.TypeError, "sleep() missing 1 required positional argument: 'delay'")
	}
	delay, ok := objects.AsFloat(delayObj)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "unsupported operand type(s) for +: 'float' and '%s'", delayObj.TypeName())
	}
	result := objects.Object(objects.None)
	if r, ok := vals["result"]; ok {
		result = r
	}
	return objects.AsyncioSleep(delay, result), nil
}

// asyncioCreateTask is asyncio.create_task(coro, *, name=None, context=None). It
// schedules the coroutine on the running loop and returns the Task at once. The
// name and context keywords are accepted for signature compatibility but not yet
// acted on; any other keyword is the TypeError CPython raises.
func asyncioCreateTask(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "create_task() takes 1 positional argument but %d were given", len(pos))
	}
	for _, k := range kwNames {
		if k != "name" && k != "context" {
			return nil, objects.Raise(objects.TypeError, "create_task() got an unexpected keyword argument '%s'", k)
		}
	}
	return objects.AsyncioCreateTask(pos[0])
}

// asyncioGather is asyncio.gather(*aws, return_exceptions=False). The awaitables
// are positional; return_exceptions is the one keyword and defaults to false.
func asyncioGather(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	returnExceptions := false
	for i, k := range kwNames {
		if k != "return_exceptions" {
			return nil, objects.Raise(objects.TypeError, "gather() got an unexpected keyword argument '%s'", k)
		}
		returnExceptions = objects.Truth(kwVals[i])
	}
	return objects.AsyncioGather(pos, returnExceptions)
}

// asyncioGetRunningLoop is asyncio.get_running_loop(). It returns the loop bound
// to the running task, or raises RuntimeError when called with no loop running.
func asyncioGetRunningLoop(args []objects.Object) (objects.Object, error) {
	l := objects.AsyncioRunningLoop()
	if l == nil {
		return nil, objects.Raise(objects.RuntimeError, "no running event loop")
	}
	return l, nil
}

// asyncioGetEventLoop is asyncio.get_event_loop(). Inside a running loop it
// returns that loop, matching CPython 3.14, where calling it outside a running
// loop is the deprecated path; this slice raises the same RuntimeError as
// get_running_loop until loop policies land.
func asyncioGetEventLoop(args []objects.Object) (objects.Object, error) {
	l := objects.AsyncioRunningLoop()
	if l == nil {
		return nil, objects.Raise(objects.RuntimeError, "no running event loop")
	}
	return l, nil
}
