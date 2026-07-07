# time is a built-in module here: the clocks and sleep are C in CPython, so the
# runtime provides them in Go. Only the shapes and monotonicity hold across
# machines, so those are what the golden checks, never an absolute reading.
import time

t0 = time.monotonic()
t1 = time.monotonic()
print("mono nondec:", t1 >= t0)
print("perf nonneg:", time.perf_counter() >= 0.0)
print("proc nonneg:", time.process_time() >= 0.0)

print("time is float:", isinstance(time.time(), float))
print("time after 2020:", time.time() > 1_600_000_000)
print("time_ns is int:", isinstance(time.time_ns(), int))
print("time_ns big:", time.time_ns() > 1_600_000_000_000_000_000)
print("mono_ns is int:", isinstance(time.monotonic_ns(), int))

print("sleep0:", time.sleep(0))

try:
    time.sleep("x")
except TypeError as e:
    print("TE:", e)

try:
    time.sleep(-1)
except ValueError as e:
    print("VE:", e)
