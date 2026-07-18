import threading

# threading.Barrier over the goroutine-backed thread state. Every observable is
# deterministic: arrival indices are collected under a lock and sorted, so the
# scheduler's order does not leak, and every handoff is sequenced with join.

results = []
results_lock = threading.Lock()
action_runs = []


def on_trip():
    action_runs.append(1)


barrier = threading.Barrier(3, action=on_trip)


def worker():
    idx = barrier.wait()
    with results_lock:
        results.append(idx)


threads = [threading.Thread(target=worker) for _ in range(3)]
for t in threads:
    t.start()
for t in threads:
    t.join()
print("indices:", sorted(results))
print("action runs:", len(action_runs))
print("parties:", barrier.parties)
print("n_waiting idle:", barrier.n_waiting)
print("broken:", barrier.broken)

# The barrier re-arms for a second full cycle.
results.clear()
threads = [threading.Thread(target=worker) for _ in range(3)]
for t in threads:
    t.start()
for t in threads:
    t.join()
print("second cycle indices:", sorted(results))

# A lone waiter with a timeout breaks the barrier. BrokenBarrierError is a
# RuntimeError subclass, so the base name catches it and type(e) names it.
tb = threading.Barrier(2)
try:
    tb.wait(timeout=0.01)
except RuntimeError as e:
    print("timeout broke:", type(e).__name__, tb.broken)

# abort wakes a parked waiter, and BrokenBarrierError is a RuntimeError subclass.
ab = threading.Barrier(2)
caught = []


def lone():
    try:
        ab.wait()
    except RuntimeError as e:
        caught.append(type(e).__name__)


w = threading.Thread(target=lone)
w.start()
while ab.n_waiting != 1:
    pass
ab.abort()
w.join()
print("abort caught:", caught[0])

# reset clears the broken state.
ab.reset()
print("after reset broken:", ab.broken)
