package objects

import "strconv"

// asyncio.TaskGroup is the structured-concurrency context manager (PEP 654 era,
// asyncio/taskgroups.py). Tasks created with tg.create_task run concurrently and
// the async with block does not leave until every one has finished. If any child
// raises, the group cancels the siblings and the parent, then re-raises the
// collected errors as an ExceptionGroup once all tasks have settled. Like the
// other asyncio primitives it lives on the loop goroutine, so its fields need no
// lock; the frame machinery serialises every create_task, aexit step, and the
// done callbacks that drive it.
//
// The parent-external-cancellation interaction CPython tracks with Task.uncancel
// counting is not modelled here: a group whose own parent task is cancelled from
// outside while inside the block is a later slice. Every path this slice drives,
// all children succeeding, one or more failing, and the sibling cancellation that
// follows a failure, matches CPython.
type asyncioTaskGroup struct {
	loop *eventLoop
	// parent is the task running the async with block, the one the group cancels
	// when a child fails so the failure surfaces at the block instead of being
	// swallowed.
	parent  *asyncTask
	entered bool
	// exiting is set once __aexit__ begins, so a create_task that races the exit
	// after the last task finished is the "is finished" RuntimeError.
	exiting bool
	// aborting is set once the group has started cancelling its children, so a
	// later child failure does not cancel the parent a second time and the aexit
	// loop knows a CancelledError it sees is its own doing, not an external one.
	aborting bool
	// parentCancelRequested guards the single parent cancel a first failure
	// triggers, matching CPython's _parent_cancel_requested.
	parentCancelRequested bool
	tasks                 map[*asyncTask]struct{}
	errors                []*Exception
	// baseError holds the first BaseException-only child (SystemExit and the
	// like), which is re-raised bare rather than wrapped in the ExceptionGroup.
	baseError *Exception
	// onCompleted is the future the exit awaits: the last child to finish resolves
	// it, waking the parent to re-check the task set.
	onCompleted *asyncFuture
}

func (tg *asyncioTaskGroup) TypeName() string { return "TaskGroup" }

// AsyncioNewTaskGroup builds asyncio.TaskGroup(), an un-entered group. The loop
// and parent task are bound at __aenter__, when a running loop and a current task
// exist, matching CPython, which reads both from the entered block.
func AsyncioNewTaskGroup() Object {
	return &asyncioTaskGroup{tasks: map[*asyncTask]struct{}{}}
}

// asyncioTaskGroupRepr renders the group the way CPython does: the tag lists a
// live task count, a collected error count, and the cancelling or entered state,
// so the RuntimeError messages that embed the repr stay byte-identical.
func asyncioTaskGroupRepr(tg *asyncioTaskGroup) string {
	s := "<TaskGroup"
	if n := len(tg.tasks); n > 0 {
		s += " tasks=" + strconv.Itoa(n)
	}
	if n := len(tg.errors); n > 0 {
		s += " errors=" + strconv.Itoa(n)
	}
	if tg.aborting {
		s += " cancelling"
	} else if tg.entered {
		s += " entered"
	}
	return s + ">"
}

// asyncioTaskGroupMethod dispatches the group's positional surface. create_task is
// the only method, and with no keywords it schedules the coroutine under the
// group's default naming.
func asyncioTaskGroupMethod(tg *asyncioTaskGroup, name string, args []Object) (Object, error) {
	if name == "create_task" {
		if len(args) != 1 {
			return nil, Raise(TypeError, "create_task() takes 2 positional arguments but %d were given", len(args)+1)
		}
		return tg.createTask(args[0], "")
	}
	return nil, noAttr(tg, name)
}

// asyncioTaskGroupMethodKw dispatches create_task with its keywords. name sets the
// task name, context is accepted for signature compatibility, and any other
// keyword is the TypeError CPython raises.
func asyncioTaskGroupMethodKw(tg *asyncioTaskGroup, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name != "create_task" {
		return nil, Raise(TypeError, "TaskGroup.%s() takes no keyword arguments", name)
	}
	if len(pos) != 1 {
		return nil, Raise(TypeError, "create_task() takes 2 positional arguments but %d were given", len(pos)+1)
	}
	taskName := ""
	for i, k := range kwNames {
		switch k {
		case "name":
			if kwVals[i] != None {
				taskName = Str(kwVals[i])
			}
		case "context":
		default:
			return nil, Raise(TypeError, "create_task() got an unexpected keyword argument '%s'", k)
		}
	}
	return tg.createTask(pos[0], taskName)
}

// createTask schedules coro as a task in the group and hooks its completion to
// onTaskDone. Creating a task before entry, after the group is finished, or while
// it is shutting down is the RuntimeError CPython raises with the group's repr.
func (tg *asyncioTaskGroup) createTask(coro Object, name string) (Object, error) {
	if !tg.entered {
		return nil, Raise(RuntimeError, "TaskGroup %s has not been entered", asyncioTaskGroupRepr(tg))
	}
	if tg.exiting && len(tg.tasks) == 0 {
		return nil, Raise(RuntimeError, "TaskGroup %s is finished", asyncioTaskGroupRepr(tg))
	}
	if tg.aborting {
		return nil, Raise(RuntimeError, "TaskGroup %s is shutting down", asyncioTaskGroupRepr(tg))
	}
	tk, err := scheduleTask(coro, tg.loop, name)
	if err != nil {
		return nil, err
	}
	tg.tasks[tk] = struct{}{}
	tk.doneFut.addDoneCallback(func() { tg.onTaskDone(tk) })
	return tk, nil
}

// onTaskDone runs when a child task settles. It drops the task, wakes the exit
// once the set empties, and, for a child that raised rather than was cancelled,
// records the error and starts the abort that cancels the siblings and parent so
// the failure surfaces at the block.
func (tg *asyncioTaskGroup) onTaskDone(tk *asyncTask) {
	delete(tg.tasks, tk)
	if tg.onCompleted != nil && len(tg.tasks) == 0 && !tg.onCompleted.doneP() {
		tg.onCompleted.setResult(True)
	}
	tk.doneFut.mu.Lock()
	cancelled, exc := tk.doneFut.cancelled, tk.doneFut.exc
	tk.doneFut.mu.Unlock()
	if cancelled || exc == nil {
		return
	}
	e, ok := errorObject(exc).(*Exception)
	if !ok {
		return
	}
	tg.errors = append(tg.errors, e)
	if tg.baseError == nil && !derivesFromException(e) {
		tg.baseError = e
	}
	// A parent that already finished has nowhere to receive the cancel; CPython
	// logs it through the loop's exception handler, which this slice does not
	// surface, so the error still rides out in the group.
	if tg.parent.doneFut.doneP() {
		return
	}
	if !tg.aborting && !tg.parentCancelRequested {
		tg.abort()
		tg.parentCancelRequested = true
		tg.parent.cancel(None)
	}
}

// abort cancels every live child. It is idempotent through the aborting flag, so
// a second failure that arrives while cancellation is under way does not restart
// it.
func (tg *asyncioTaskGroup) abort() {
	tg.aborting = true
	for tk := range tg.tasks {
		if !tk.doneFut.doneP() {
			tk.cancel(None)
		}
	}
}

// aenter binds the group to the running loop and its parent task and marks it
// entered, the value `async with asyncio.TaskGroup() as tg` binds. Re-entering a
// group, or entering one outside a task, is the RuntimeError CPython raises.
func (tg *asyncioTaskGroup) aenter(th *Thread) (Object, error) {
	body := func(y Yielder) (Object, error) {
		if tg.entered {
			return nil, Raise(RuntimeError, "TaskGroup %s has already been entered", asyncioTaskGroupRepr(tg))
		}
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		tg.loop = loop
		tg.parent = loop.current
		if tg.parent == nil {
			return nil, Raise(RuntimeError, "TaskGroup %s cannot determine the parent task", asyncioTaskGroupRepr(tg))
		}
		tg.entered = true
		return tg, nil
	}
	return &generatorObject{qual: "TaskGroup.__aenter__", body: fromTop(body), ret: None, isCoro: true}, nil
}

// aexit awaits every child, then decides the block's outcome. It cancels the
// siblings when the body itself raised, waits out the task set, and re-raises the
// collected child errors as an ExceptionGroup, or a bare base error, or the
// CancelledError that propagated in when nothing else was collected, mirroring
// CPython's _aexit.
func (tg *asyncioTaskGroup) aexit(th *Thread, args []Object) (Object, error) {
	body := func(y Yielder) (Object, error) {
		tg.exiting = true
		etNotNone := len(args) >= 1 && args[0] != None
		var bodyExc *Exception
		etIsCancelled := false
		if len(args) >= 2 {
			if pe, ok := args[1].(*Exception); ok {
				bodyExc = pe
				etIsCancelled = isCancelledError(pe)
			}
		}
		if bodyExc != nil && tg.baseError == nil && !derivesFromException(bodyExc) {
			tg.baseError = bodyExc
		}
		var propagateCancel *Exception
		if etIsCancelled {
			propagateCancel = bodyExc
		}
		if etNotNone && !tg.aborting {
			tg.abort()
		}
		for len(tg.tasks) > 0 {
			if tg.onCompleted == nil {
				tg.onCompleted = &asyncFuture{loop: tg.loop}
			}
			_, err := y.YieldFrom(&futureAwait{f: tg.onCompleted})
			tg.onCompleted = nil
			if err != nil {
				if !isCancelledError(err) {
					return nil, err
				}
				// A CancelledError here is the parent being cancelled. When the group
				// did not start it, it is an external cancel to propagate; when the
				// group did (aborting is already set), it is the abort landing and is
				// absorbed so the loop keeps draining the children.
				if !tg.aborting {
					if pe, ok := err.(*Exception); ok {
						propagateCancel = pe
					}
					tg.abort()
				}
			}
		}
		if tg.baseError != nil {
			return nil, tg.baseError
		}
		if propagateCancel != nil && len(tg.errors) == 0 {
			return nil, propagateCancel
		}
		if etNotNone && !etIsCancelled && bodyExc != nil {
			tg.errors = append(tg.errors, bodyExc)
		}
		if len(tg.errors) > 0 {
			items := make([]Object, len(tg.errors))
			for i, e := range tg.errors {
				items[i] = e
			}
			tg.errors = nil
			grp, gerr := NewExcGroup("BaseExceptionGroup", []Object{NewStr("unhandled errors in a TaskGroup"), NewList(items)})
			if gerr != nil {
				return nil, gerr
			}
			// raise ... from None: the group replaces the block's context rather than
			// chaining onto it, so the traceback does not print the child errors twice.
			g := grp.(*Exception)
			g.SuppressContext = true
			return nil, g
		}
		return None, nil
	}
	return &generatorObject{qual: "TaskGroup.__aexit__", body: fromTop(body), ret: None, isCoro: true}, nil
}
