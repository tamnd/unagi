import threading

# threading.Condition over the goroutine-backed thread state. Every observable is
# deterministic: the fixture never prints a repr (which carries an address) and
# sequences every handoff with join, so the output is byte for byte stable
# against CPython.

cond = threading.Condition()

# A fresh condition is not owned, and its context manager takes the lock.
print("owned before:", cond._is_owned())
with cond:
    print("owned inside:", cond._is_owned())
print("owned after:", cond._is_owned())

# wait requires the lock, or it is a RuntimeError with CPython's exact message.
try:
    cond.wait()
except RuntimeError as e:
    print("wait unowned:", e)

# notify requires the lock too.
try:
    cond.notify()
except RuntimeError as e:
    print("notify unowned:", e)

# A wait with a timeout and no notify returns False.
with cond:
    print("wait timeout:", cond.wait(0.01))

# A single producer hands one item to a single consumer through wait_for. The
# consumer either parks and is woken, or finds the item already there; either way
# it reads 42.
box = []
result = []


def consumer():
    with cond:
        cond.wait_for(lambda: len(box) > 0)
        result.append(box[0])


t = threading.Thread(target=consumer)
t.start()
with cond:
    box.append(42)
    cond.notify()
t.join()
print("consumer got:", result[0])

# notify_all wakes several waiters. Each worker records its name once released;
# the fixture sorts the names so the order the scheduler wakes them in does not
# leak.
gate = [False]
seen = []
seen_lock = threading.Lock()
workers = []


def waiter(tag):
    with cond:
        cond.wait_for(lambda: gate[0])
    with seen_lock:
        seen.append(tag)


for i in range(3):
    w = threading.Thread(target=waiter, args=("w%d" % i,))
    w.start()
    workers.append(w)

with cond:
    gate[0] = True
    cond.notify_all()
for w in workers:
    w.join()
print("woken:", sorted(seen))

# A Condition can wrap an explicit Lock, and _is_owned tracks that lock's state.
lock = threading.Lock()
c2 = threading.Condition(lock)
print("lock-backed owned before:", c2._is_owned())
with c2:
    print("lock-backed owned inside:", c2._is_owned())
print("lock-backed owned after:", c2._is_owned())
