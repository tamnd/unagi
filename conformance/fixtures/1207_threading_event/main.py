import threading

# threading.Event over the goroutine-backed thread state. Every observable is
# deterministic: the fixture never prints a repr (which carries an address) and
# sequences every handoff with join, so the output is byte for byte stable
# against CPython.

e = threading.Event()

# A fresh event is unset.
print("set at start:", e.is_set())

# set raises the flag, and a set event answers wait at once.
e.set()
print("set after set:", e.is_set())
print("wait when set:", e.wait())
print("wait timeout when set:", e.wait(0.01))

# clear lowers the flag, and a wait with a timeout and no set returns False.
e.clear()
print("set after clear:", e.is_set())
print("wait timeout when clear:", e.wait(0.01))

# A worker blocks on the event until the main thread sets it, then reports.
result = []


def worker():
    e.wait()
    result.append("released")


e.clear()
t = threading.Thread(target=worker)
t.start()
e.set()
t.join()
print("worker:", result[0])

# One set releases many waiters. Each records its tag once woken; the fixture
# sorts the tags so the order the scheduler wakes them in does not leak.
gate = threading.Event()
seen = []
seen_lock = threading.Lock()
workers = []


def waiter(tag):
    gate.wait()
    with seen_lock:
        seen.append(tag)


for i in range(3):
    w = threading.Thread(target=waiter, args=("w%d" % i,))
    w.start()
    workers.append(w)

gate.set()
for w in workers:
    w.join()
print("woken:", sorted(seen))
