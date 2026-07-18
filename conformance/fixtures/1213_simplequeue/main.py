import queue
import threading
from queue import Empty

# queue.SimpleQueue is the unbounded C queue: put never blocks, and there is no
# task_done, join, maxsize, or full. The fixture never prints a repr (which carries
# an address) and sequences its one handoff, so the output is byte for byte stable
# against CPython.

sq = queue.SimpleQueue()
print("empty:", sq.empty())

# Items come back first-in first-out, and qsize tracks the count.
for i in range(4):
    sq.put(i)
print("qsize:", sq.qsize())
print("order:", [sq.get() for _ in range(4)])
print("empty after drain:", sq.empty())

# put never blocks: a SimpleQueue is unbounded, so a burst of puts with no reader is
# fine, and put_nowait is just put with no room to run out of.
for i in range(100):
    sq.put_nowait(i)
print("burst qsize:", sq.qsize())
print("burst first:", sq.get(), "last of 100 after drain:", [sq.get() for _ in range(99)][-1])

# get_nowait on an empty queue raises queue.Empty.
try:
    sq.get_nowait()
except Empty:
    print("get_nowait empty: Empty")

# A blocking get with a timeout gives up with queue.Empty.
try:
    sq.get(timeout=0.01)
except Empty:
    print("get timeout: Empty")

# A blocking call rejects a negative timeout the way Queue.get does.
try:
    sq.get(timeout=-1)
except ValueError as e:
    print("negative timeout:", e)

# A blocking get parks until another thread puts. The worker feeds one item, the
# main thread reads it back after the join, so the order is fixed.
handoff = queue.SimpleQueue()


def producer():
    handoff.put("payload")


w = threading.Thread(target=producer)
w.start()
w.join()
print("handoff:", handoff.get())

# The type reports its own name.
print("type:", type(sq).__name__)
