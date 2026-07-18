import threading

# threading.local over the goroutine-backed thread state. Each thread sees its
# own attribute store: a value the main thread sets is invisible to a worker
# until that worker sets its own, so every worker reads the same fresh miss. All
# observables are made deterministic by collecting under a lock and sorting.

data = threading.local()

# The main thread stores and reads back its own value.
data.x = "main"
print("main x:", data.x)

results = []
lock = threading.Lock()


def worker(n):
    # This worker set nothing, so main's x is invisible to it.
    seen_before = hasattr(data, "x")
    data.x = n
    with lock:
        results.append((n, seen_before, data.x))


threads = [threading.Thread(target=worker, args=(i,)) for i in range(5)]
for t in threads:
    t.start()
for t in threads:
    t.join()

for n, seen, val in sorted(results):
    print("worker", n, seen, val)

# None of the workers disturbed the main thread's own value.
print("main x after:", data.x)

# The attribute builtins reach the same per-thread store as the dotted syntax.
print("getattr missing:", getattr(data, "y", "default"))
setattr(data, "y", 99)
print("getattr y:", getattr(data, "y"))
print("hasattr y:", hasattr(data, "y"))

delattr(data, "y")
print("hasattr y after del:", hasattr(data, "y"))

# A missing attribute raises AttributeError spelled with the qualified type name.
try:
    data.missing
except AttributeError as e:
    print("attr error:", e)

# Deleting a missing attribute raises the same AttributeError.
try:
    del data.missing
except AttributeError as e:
    print("del error:", e)

# The base local accepts no constructor arguments.
try:
    threading.local(1)
except TypeError as e:
    print("init error:", e)

# type(data).__name__ is the bare _local CPython reports.
print("type name:", type(data).__name__)
