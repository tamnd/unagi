import tracemalloc

# The tracer state round-trips.
print("tracing init:", tracemalloc.is_tracing())
print("limit init:", tracemalloc.get_traceback_limit())

tracemalloc.start(5)
print("tracing after start(5):", tracemalloc.is_tracing())
print("limit after start(5):", tracemalloc.get_traceback_limit())

# The limit is retained across a stop and reset by the next start.
tracemalloc.stop()
print("tracing after stop:", tracemalloc.is_tracing())
print("limit after stop:", tracemalloc.get_traceback_limit())
tracemalloc.start()
print("limit after start():", tracemalloc.get_traceback_limit())
tracemalloc.stop()

# When not tracing, get_traced_memory is (0, 0) and take_snapshot refuses.
print("traced_mem not tracing:", tracemalloc.get_traced_memory())
try:
    tracemalloc.take_snapshot()
except RuntimeError:
    print("take_snapshot not tracing: RuntimeError")

# clear_traces and reset_peak return None.
print("clear_traces:", tracemalloc.clear_traces())
print("reset_peak:", tracemalloc.reset_peak())

# start() rejects an out-of-range frame count.
try:
    tracemalloc.start(0)
except ValueError:
    print("start(0): ValueError")

# get_object_traceback of an untraced object is None.
print("obj traceback:", tracemalloc.get_object_traceback(object()))
