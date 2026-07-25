# sys.monitoring (PEP 669) was missing, so bdb/pdb/doctest failed at import on
# `E = sys.monitoring.events`. A compiled program has no bytecode eval loop, so a
# registered monitoring callback never fires, the same inert-but-honest gap as
# sys.settrace. The bookkeeping half (tool ids, names, event masks, callbacks)
# round-trips, which is all these modules read back.

import sys
import bdb

m = sys.monitoring
E = m.events

# The event flags are the PEP 669 powers of two, so masks OR together.
print(E.NO_EVENTS, E.PY_START, E.PY_RETURN, E.CALL, E.LINE, E.RAISE, E.BRANCH)
print(E.PY_START | E.PY_RESUME | E.PY_THROW | E.PY_UNWIND | E.RAISE)

# The reserved tool ids.
print(m.DEBUGGER_ID, m.COVERAGE_ID, m.PROFILER_ID, m.OPTIMIZER_ID)

# DISABLE and MISSING are distinct sentinels.
print(m.DISABLE is m.MISSING)


def show(label, f):
    try:
        print(label, "->", f())
    except Exception as e:
        print(label, "ERR", type(e).__name__, e)


# Claiming, reading, and freeing a tool id round-trips; a double claim and an
# out-of-range id raise the CPython ValueErrors.
show("use", lambda: m.use_tool_id(0, "dbg"))
print("get_tool", m.get_tool(0))
show("use_again", lambda: m.use_tool_id(0, "other"))
show("out_of_range", lambda: m.use_tool_id(6, "x"))

# Events default to 0, set on an in-use tool, and error on a free one.
print("events_default", m.get_events(0))
m.set_events(0, E.LINE | E.CALL)
print("events_after", m.get_events(0))
show("set_free", lambda: m.set_events(3, E.LINE))

# register_callback returns the callback it replaced; a None func removes it.
print("first_prev", m.register_callback(0, E.LINE, lambda *a: None))
print("second_prev_callable", callable(m.register_callback(0, E.LINE, None)))

# clear leaves the id claimed but drops its events; free releases it entirely.
m.clear_tool_id(0)
print("cleared_events", m.get_events(0), "still_claimed", m.get_tool(0))
m.free_tool_id(0)
print("after_free", m.get_tool(0))
print("restart", m.restart_events())

# bdb imports and constructs on both backends; the default settrace backend
# leaves the monitoring tracer unused.
b = bdb.Bdb()
print("default", b.backend, b.monitoring_tracer is None)
mb = bdb.Bdb(backend="monitoring")
print("monitoring", mb.backend, mb.monitoring_tracer is not None)
print(bdb._MonitoringTracer.GLOBAL_EVENTS, bdb._MonitoringTracer.LOCAL_EVENTS)

print("done")
