package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// sys.monitoring is PEP 669's low-overhead monitoring API: a program registers a
// tool id and per-event callbacks the interpreter fires from its bytecode eval
// loop. A compiled program runs native Go with no such loop, so a registered
// callback never fires, exactly like the sys.settrace and sys.setprofile hooks
// this tier already keeps inert. The state a program's result depends on is the
// bookkeeping half: which tool ids are claimed, their names, their event masks,
// and their callbacks; that round-trips honestly here so bdb, pdb and doctest,
// which read sys.monitoring.events at import time and drive the tool-id and
// event API, import and run their non-tracing paths. The debugging that would
// need a live trace simply observes nothing, the same honest gap as settrace.
//
// The whole surface is portable, so every build target registers it.

// monToolCount is the number of monitoring tool ids CPython 3.14 reserves, ids 0
// through 5. A tool id outside the range raises the same ValueError CPython does.
const monToolCount = 6

// monTool holds one tool id's honest bookkeeping. name is empty when the id is
// not in use. events is the global event mask; callbacks maps an event flag to
// its registered callable; local maps a code object to its per-code mask. None
// of it ever fires, so it is pure state a program reads back.
type monTool struct {
	name      string
	inUse     bool
	events    int64
	callbacks map[int64]objects.Object
	local     map[objects.Object]int64
}

// monState is the process-wide monitoring registry. A compiled program is
// single-threaded until M5 and the API is not hot, so one mutex over the small
// fixed table is enough.
type monState struct {
	mu    sync.Mutex
	tools [monToolCount]monTool
}

var sysMonitoring = &monState{}

// monValidateTool bounds-checks a tool id, raising the ValueError CPython raises
// for an id outside 0..5.
func monValidateTool(id int64) error {
	if id < 0 || id >= monToolCount {
		return objects.Raise(objects.ValueError, "invalid tool %d (must be between 0 and 5)", id)
	}
	return nil
}

// monToolArg reads the leading tool-id argument and bounds-checks it.
func monToolArg(args []objects.Object) (int64, error) {
	id, ok := objects.AsInt(args[0])
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required")
	}
	if err := monValidateTool(id); err != nil {
		return 0, err
	}
	return id, nil
}

func monUseToolID(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	name, ok := objects.AsStr(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "str expected, not %s", args[1].TypeName())
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	if sysMonitoring.tools[id].inUse {
		return nil, objects.Raise(objects.ValueError, "tool %d is already in use", id)
	}
	sysMonitoring.tools[id] = monTool{name: name, inUse: true}
	return objects.None, nil
}

func monFreeToolID(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	sysMonitoring.tools[id] = monTool{}
	return objects.None, nil
}

func monGetTool(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	if !sysMonitoring.tools[id].inUse {
		return objects.None, nil
	}
	return objects.NewStr(sysMonitoring.tools[id].name), nil
}

// monClearToolID clears a tool's events and callbacks but leaves the id claimed,
// matching CPython, which keeps the tool registered so it can re-arm events.
func monClearToolID(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	t := &sysMonitoring.tools[id]
	t.events = 0
	t.callbacks = nil
	t.local = nil
	return objects.None, nil
}

// monRegisterCallback stores the callback for one event and returns the one it
// replaced, or None. A None func removes the callback. The callback never fires,
// so this is a faithful set-and-return-previous with no side effect.
func monRegisterCallback(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	event, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	t := &sysMonitoring.tools[id]
	var prev objects.Object = objects.None
	if t.callbacks != nil {
		if p, ok := t.callbacks[event]; ok {
			prev = p
		}
	}
	if args[2] == objects.None {
		if t.callbacks != nil {
			delete(t.callbacks, event)
		}
		return prev, nil
	}
	if t.callbacks == nil {
		t.callbacks = map[int64]objects.Object{}
	}
	t.callbacks[event] = args[2]
	return prev, nil
}

func monSetEvents(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	mask, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	if !sysMonitoring.tools[id].inUse {
		return nil, objects.Raise(objects.ValueError, "tool %d is not in use", id)
	}
	sysMonitoring.tools[id].events = mask
	return objects.None, nil
}

func monGetEvents(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	return objects.NewInt(sysMonitoring.tools[id].events), nil
}

func monSetLocalEvents(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	mask, ok := objects.AsInt(args[2])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	if !sysMonitoring.tools[id].inUse {
		return nil, objects.Raise(objects.ValueError, "tool %d is not in use", id)
	}
	t := &sysMonitoring.tools[id]
	if t.local == nil {
		t.local = map[objects.Object]int64{}
	}
	t.local[args[1]] = mask
	return objects.None, nil
}

func monGetLocalEvents(args []objects.Object) (objects.Object, error) {
	id, err := monToolArg(args)
	if err != nil {
		return nil, err
	}
	sysMonitoring.mu.Lock()
	defer sysMonitoring.mu.Unlock()
	t := &sysMonitoring.tools[id]
	if t.local != nil {
		if m, ok := t.local[args[1]]; ok {
			return objects.NewInt(m), nil
		}
	}
	return objects.NewInt(0), nil
}

// monRestartEvents re-arms the deferred events CPython disabled through a DISABLE
// return. No callback ever fires here, so nothing is deferred and this is a
// no-op that returns None, all a caller observes.
func monRestartEvents(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// monEventsNamespace builds sys.monitoring.events: the PEP 669 event flags as a
// types.SimpleNamespace, each a power-of-two bit so the masks OR together the way
// bdb builds GLOBAL_EVENTS and LOCAL_EVENTS. The values match CPython 3.14 so a
// program that stores or compares a mask reads the same bits back.
func monEventsNamespace() objects.Object {
	names := []string{
		"NO_EVENTS", "PY_START", "PY_RESUME", "PY_RETURN", "PY_YIELD", "CALL",
		"LINE", "INSTRUCTION", "JUMP", "BRANCH_LEFT", "BRANCH_RIGHT",
		"STOP_ITERATION", "RAISE", "EXCEPTION_HANDLED", "PY_UNWIND", "PY_THROW",
		"RERAISE", "C_RETURN", "C_RAISE", "BRANCH",
	}
	vals := []int64{
		0, 1, 2, 4, 8, 16,
		32, 64, 128, 256, 512,
		1024, 2048, 4096, 8192, 16384,
		32768, 65536, 131072, 262144,
	}
	objs := make([]objects.Object, len(vals))
	for i, v := range vals {
		objs[i] = objects.NewInt(v)
	}
	return objects.NewSimpleNamespace(names, objs)
}

// buildSysMonitoring builds the sys.monitoring namespace: the tool-id constants,
// the DISABLE and MISSING sentinels, the events sub-namespace, and the tool and
// event API bound to the process-wide registry.
func buildSysMonitoring() objects.Object {
	fn := func(name string, arity int, f func([]objects.Object) (objects.Object, error)) objects.Object {
		return objects.NewFunc(name, arity, f)
	}
	names := []string{
		"DEBUGGER_ID", "COVERAGE_ID", "PROFILER_ID", "OPTIMIZER_ID",
		"DISABLE", "MISSING", "events",
		"use_tool_id", "free_tool_id", "get_tool", "clear_tool_id",
		"register_callback", "set_events", "get_events",
		"set_local_events", "get_local_events", "restart_events",
	}
	vals := []objects.Object{
		objects.NewInt(0), objects.NewInt(1), objects.NewInt(2), objects.NewInt(5),
		// DISABLE and MISSING are unique sentinels compared by identity; a
		// callback returns DISABLE to turn its event off. They never fire here,
		// so two distinct empty namespaces carry the identity a program needs.
		objects.NewSimpleNamespace(nil, nil),
		objects.NewSimpleNamespace(nil, nil),
		monEventsNamespace(),
		fn("use_tool_id", 2, monUseToolID),
		fn("free_tool_id", 1, monFreeToolID),
		fn("get_tool", 1, monGetTool),
		fn("clear_tool_id", 1, monClearToolID),
		fn("register_callback", 3, monRegisterCallback),
		fn("set_events", 2, monSetEvents),
		fn("get_events", 1, monGetEvents),
		fn("set_local_events", 3, monSetLocalEvents),
		fn("get_local_events", 2, monGetLocalEvents),
		fn("restart_events", 0, monRestartEvents),
	}
	return objects.NewSimpleNamespace(names, vals)
}
