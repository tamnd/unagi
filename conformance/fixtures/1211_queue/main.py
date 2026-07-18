import queue
import threading
from queue import Empty, Full

# queue.Queue over the goroutine-backed thread state. Every observable is
# deterministic: the fixture never prints a repr (which carries an address) and
# sequences every handoff with join, so the output is byte for byte stable
# against CPython.

q = queue.Queue()

# A fresh queue is empty and unbounded, so maxsize reads zero.
print("empty:", q.empty())
print("maxsize:", q.maxsize)

# Items come back in first-in first-out order, and qsize tracks the count.
for i in range(3):
    q.put(i)
print("qsize:", q.qsize())
print("get order:", [q.get() for _ in range(3)])
print("empty after drain:", q.empty())

# get_nowait on an empty queue raises queue.Empty.
try:
    q.get_nowait()
except Empty:
    print("get_nowait empty: Empty")

# A blocking get with a timeout gives up with queue.Empty.
try:
    q.get(timeout=0.01)
except Empty:
    print("get timeout: Empty")

# A bounded queue refuses a put past its capacity.
b = queue.Queue(2)
b.put(1)
b.put(2)
print("full:", b.full())
try:
    b.put_nowait(3)
except Full:
    print("put_nowait full: Full")
try:
    b.put(3, timeout=0.01)
except Full:
    print("put timeout: Full")

# A blocking put on a full queue waits for a get to make room.
b.get()
b.put(3)
print("bounded drain:", [b.get(), b.get()])

# A blocking get parks until another thread puts. The worker feeds one item, the
# main thread reads it back after the join, so the order is fixed.
handoff = queue.Queue()
seen = []


def producer():
    handoff.put("payload")


w = threading.Thread(target=producer)
w.start()
w.join()
print("handoff:", handoff.get())

# task_done and join balance a producer against a consumer. The main thread
# enqueues the work, a pool drains it, and join returns only once every item has
# been marked done. Results are sorted for a stable print.
work = queue.Queue()
done = []
done_lock = threading.Lock()
for n in range(10):
    work.put(n)


def consumer():
    while True:
        try:
            item = work.get_nowait()
        except Empty:
            return
        with done_lock:
            done.append(item)
        work.task_done()


pool = [threading.Thread(target=consumer) for _ in range(4)]
for t in pool:
    t.start()
work.join()
for t in pool:
    t.join()
print("consumed:", sorted(done))
print("all done:", work.qsize())

# task_done called more times than there were puts is a ValueError.
try:
    work.task_done()
except ValueError as e:
    print("over task_done:", e)

# A blocking call rejects a negative timeout; a non-blocking call ignores it.
try:
    queue.Queue().get(timeout=-1)
except ValueError as e:
    print("negative timeout:", e)
try:
    queue.Queue().get(block=False, timeout=-1)
except Empty:
    print("nonblocking ignores timeout: Empty")

# The exception classes carry the names CPython reports.
print("Empty name:", queue.Empty.__name__, queue.Empty.__module__)
print("Full name:", queue.Full.__name__, queue.Full.__module__)
print("subclass:", issubclass(queue.Empty, Exception), issubclass(queue.Full, Exception))
print("queue type:", type(q).__name__)
