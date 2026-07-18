import threading

# threading.Semaphore and BoundedSemaphore over the goroutine-backed thread
# state. Every observable is deterministic: the fixture never prints a repr
# (which carries an address) and sequences every handoff with join, so the
# output is byte for byte stable against CPython.

sem = threading.Semaphore(2)

# Two acquires drain a value-2 semaphore, and a third non-blocking acquire misses.
print("acquire 1:", sem.acquire())
print("acquire 2:", sem.acquire())
print("acquire 3 nonblocking:", sem.acquire(blocking=False))

# A timed acquire on the drained semaphore returns False.
print("acquire timeout:", sem.acquire(timeout=0.01))

# release lifts the count, and a following acquire takes it.
sem.release()
print("acquire after release:", sem.acquire(blocking=False))

# A non-blocking acquire may not carry a timeout.
try:
    sem.acquire(blocking=False, timeout=1.0)
except ValueError as e:
    print("nonblocking timeout:", e)

# release(n) below one is a ValueError.
try:
    sem.release(0)
except ValueError as e:
    print("release zero:", e)

# The context manager acquires on entry and releases on exit. A fresh value-1
# semaphore is free inside the block and free again after.
cm = threading.Semaphore(1)
with cm:
    print("cm inside nonblocking spare:", cm.acquire(blocking=False))
    cm.release()
print("cm after nonblocking:", cm.acquire(blocking=False))

# A worker blocks on an empty semaphore until the main thread releases it.
gate = threading.Semaphore(0)
result = []


def worker():
    gate.acquire()
    result.append("released")


w = threading.Thread(target=worker)
w.start()
gate.release()
w.join()
print("worker:", result[0])

# BoundedSemaphore refuses a release past its initial value.
b = threading.BoundedSemaphore(1)
b.acquire()
b.release()
try:
    b.release()
except ValueError as e:
    print("bounded over-release:", e)
