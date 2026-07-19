package vet

import "github.com/tamnd/unagi/pkg/frontend"

// checkAsyncOrphanTask reports UNA-AIO-002, a fire-and-forget task. The shape is
// asyncio.create_task called as a bare statement, its result kept nowhere:
//
//	async def main():
//	    asyncio.create_task(worker())   # nothing holds the returned task
//	    await serve()
//
// The event loop keeps only a weak reference to a task, so a task no one else
// holds a reference to can be garbage-collected before it finishes, and it then
// stops mid-run. Worse, an exception raised inside such a task is delivered to
// the loop's handler of last resort rather than to any awaiter, so it vanishes
// from the visible call stack. The fix is to keep the task: bind it to a
// variable you later await, add it to a set you hold for the loop's lifetime,
// or create it under an asyncio.TaskGroup, which owns and awaits its children.
//
// The check fires on a bare asyncio.create_task (or a create_task imported from
// asyncio) whose value is dropped. A create_task on some other receiver, such as
// a TaskGroup's tg.create_task, is left alone, since the group holds the task.
func checkAsyncOrphanTask(mod *frontend.Module) []Finding {
	var out []Finding
	eachStmt(mod.Body, func(s frontend.Stmt) {
		es, ok := s.(*frontend.ExprStmt)
		if !ok {
			return
		}
		call, ok := es.X.(*frontend.Call)
		if !ok || !isOrphanCreateTask(call) {
			return
		}
		out = append(out, Finding{
			Code: "UNA-AIO-002",
			Pos:  call.Span(),
			Msg: "the task from asyncio.create_task is discarded; the loop holds only a weak reference, so an unheld task can be " +
				"garbage-collected mid-run and its exceptions vanish; keep it in a variable you await or a set you hold, or use asyncio.TaskGroup",
		})
	})
	return out
}

// isOrphanCreateTask reports whether a call is asyncio.create_task or a bare
// create_task imported from asyncio. A create_task on any other receiver is not
// matched, since a TaskGroup's create_task hands the task to the group and is
// not orphaned.
func isOrphanCreateTask(call *frontend.Call) bool {
	switch fn := call.Fn.(type) {
	case *frontend.Name:
		return fn.Id == "create_task"
	case *frontend.Attribute:
		if fn.Name != "create_task" {
			return false
		}
		recv, ok := fn.X.(*frontend.Name)
		return ok && recv.Id == "asyncio"
	}
	return false
}
