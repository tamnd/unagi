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
		{"wait_for", objects.NewFuncKw("wait_for", asyncioWaitFor)},
		{"Future", objects.NewFuncKw("Future", asyncioFuture)},
		{"Lock", objects.NewFuncKw("Lock", asyncioLock)},
		{"Event", objects.NewFuncKw("Event", asyncioEvent)},
		{"Condition", objects.NewFuncKw("Condition", asyncioCondition)},
		{"Semaphore", objects.NewFuncKw("Semaphore", asyncioSemaphore)},
		{"BoundedSemaphore", objects.NewFuncKw("BoundedSemaphore", asyncioBoundedSemaphore)},
		{"Queue", objects.NewFuncKw("Queue", asyncioQueue)},
		{"LifoQueue", objects.NewFuncKw("LifoQueue", asyncioLifoQueue)},
		{"PriorityQueue", objects.NewFuncKw("PriorityQueue", asyncioPriorityQueue)},
		{"QueueEmpty", objects.AsyncioQueueEmptyClass()},
		{"QueueFull", objects.AsyncioQueueFullClass()},
		{"current_task", objects.NewFuncKw("current_task", asyncioCurrentTask)},
		{"all_tasks", objects.NewFuncKw("all_tasks", asyncioAllTasks)},
		{"get_running_loop", objects.NewFunc("get_running_loop", 0, asyncioGetRunningLoop)},
		{"get_event_loop", objects.NewFunc("get_event_loop", 0, asyncioGetEventLoop)},
		{"CancelledError", objects.AsyncioCancelledErrorClass()},
		{"InvalidStateError", objects.AsyncioInvalidStateErrorClass()},
		{"TimeoutError", objects.ExcClass2("TimeoutError")},
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
	name := ""
	for i, k := range kwNames {
		switch k {
		case "name":
			// A name of None keeps the auto-numbered default; any other value is
			// stringified, matching CPython's set_name.
			if kwVals[i] != objects.None {
				name = objects.Str(kwVals[i])
			}
		case "context":
		default:
			return nil, objects.Raise(objects.TypeError, "create_task() got an unexpected keyword argument '%s'", k)
		}
	}
	return objects.AsyncioCreateTask(pos[0], name)
}

// asyncioFuture is asyncio.Future(*, loop=None). It builds a pending Future bound
// to the running loop. The loop keyword is accepted for signature compatibility
// but ignored, since this slice runs one loop; any other keyword and any
// positional argument are the errors CPython raises.
func asyncioFuture(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 0 {
		return nil, objects.Raise(objects.TypeError, "Future() takes no positional arguments")
	}
	for _, k := range kwNames {
		if k != "loop" {
			return nil, objects.Raise(objects.TypeError, "Future() got an unexpected keyword argument '%s'", k)
		}
	}
	return objects.AsyncioNewFuture()
}

// asyncioLock is asyncio.Lock(). CPython 3.14 dropped the loop keyword, so the
// constructor takes no arguments; any positional or keyword argument is the
// error CPython raises.
func asyncioLock(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 0 {
		return nil, objects.Raise(objects.TypeError, "Lock() takes no positional arguments")
	}
	for _, k := range kwNames {
		return nil, objects.Raise(objects.TypeError, "Lock() got an unexpected keyword argument '%s'", k)
	}
	return objects.AsyncioNewLock(), nil
}

// asyncioEvent is asyncio.Event(). CPython 3.14 dropped the loop keyword, so the
// constructor takes no arguments; any positional or keyword argument is the
// error CPython raises.
func asyncioEvent(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 0 {
		return nil, objects.Raise(objects.TypeError, "Event() takes no positional arguments")
	}
	for _, k := range kwNames {
		return nil, objects.Raise(objects.TypeError, "Event() got an unexpected keyword argument '%s'", k)
	}
	return objects.AsyncioNewEvent(), nil
}

// asyncioCondition is asyncio.Condition(lock=None). With no lock it builds a fresh
// one; a supplied asyncio.Lock is shared. Any other lock value is the TypeError
// CPython raises.
func asyncioCondition(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("Condition", []string{"lock"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	var lock objects.Object
	if v, ok := vals["lock"]; ok {
		lock = v
	}
	return objects.AsyncioNewCondition(lock)
}

// asyncioSemaphore is asyncio.Semaphore(value=1). value is the permit count and
// defaults to one; a negative value is the ValueError the constructor raises.
func asyncioSemaphore(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return newAsyncioSemaphore("Semaphore", false, pos, kwNames, kwVals)
}

// asyncioBoundedSemaphore is asyncio.BoundedSemaphore(value=1), a semaphore that
// refuses to release more permits than it started with.
func asyncioBoundedSemaphore(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return newAsyncioSemaphore("BoundedSemaphore", true, pos, kwNames, kwVals)
}

// newAsyncioSemaphore binds the shared value argument of both semaphore
// constructors and builds the counter.
func newAsyncioSemaphore(who string, bounded bool, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	value := 1
	vals, err := bindArgs(who, []string{"value"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	if v, ok := vals["value"]; ok {
		n, ok := objects.AsInt(v)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", v.TypeName())
		}
		value = int(n)
	}
	return objects.AsyncioNewSemaphore(value, bounded)
}

// asyncioQueue is asyncio.Queue(maxsize=0). maxsize bounds the queue; zero or
// less is unbounded. A non-integer maxsize is the TypeError CPython raises.
func asyncioQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	maxsize, err := asyncioQueueMaxsize("Queue", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.AsyncioNewQueue(maxsize), nil
}

// asyncioLifoQueue is asyncio.LifoQueue(maxsize=0), whose get returns the most
// recently put item.
func asyncioLifoQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	maxsize, err := asyncioQueueMaxsize("LifoQueue", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.AsyncioNewLifoQueue(maxsize), nil
}

// asyncioPriorityQueue is asyncio.PriorityQueue(maxsize=0), whose get returns the
// smallest item under Python's <.
func asyncioPriorityQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	maxsize, err := asyncioQueueMaxsize("PriorityQueue", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.AsyncioNewPriorityQueue(maxsize), nil
}

// asyncioQueueMaxsize binds the shared maxsize argument of the three queue
// constructors. maxsize bounds the queue; zero or less is unbounded. A non-integer
// maxsize is the TypeError CPython raises.
func asyncioQueueMaxsize(who string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (int, error) {
	maxsize := 0
	vals, err := bindArgs(who, []string{"maxsize"}, pos, kwNames, kwVals)
	if err != nil {
		return 0, err
	}
	if v, ok := vals["maxsize"]; ok {
		n, ok := objects.AsInt(v)
		if !ok {
			return 0, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", v.TypeName())
		}
		maxsize = int(n)
	}
	return maxsize, nil
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

// asyncioWaitFor is asyncio.wait_for(aw, timeout). It awaits aw, raising
// TimeoutError if timeout seconds pass first. A timeout of None waits forever.
func asyncioWaitFor(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("wait_for", []string{"fut", "timeout"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	aw, ok := vals["fut"]
	if !ok {
		return nil, objects.Raise(objects.TypeError, "wait_for() missing 1 required positional argument: 'fut'")
	}
	timeout := objects.Object(objects.None)
	if v, ok := vals["timeout"]; ok {
		timeout = v
	}
	return objects.AsyncioWaitFor(aw, timeout), nil
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

// asyncioCurrentTask is asyncio.current_task(loop=None). It returns the currently
// running task, or None when no task is running on the loop.
func asyncioCurrentTask(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("current_task", []string{"loop"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.AsyncioCurrentTask(vals["loop"])
}

// asyncioAllTasks is asyncio.all_tasks(loop=None). It returns the set of the
// loop's not-yet-done tasks, raising RuntimeError with no running loop.
func asyncioAllTasks(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	vals, err := bindArgs("all_tasks", []string{"loop"}, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.AsyncioAllTasks(vals["loop"])
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
