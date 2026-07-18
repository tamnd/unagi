import queue
import threading
from queue import Empty, Full

# queue.LifoQueue and queue.PriorityQueue subclass Queue and change only the
# container: a stack and a heap. The fixture never prints a repr (which carries an
# address) and sequences every handoff, so the output is byte for byte stable
# against CPython.

# A LifoQueue returns the most recently put item.
lifo = queue.LifoQueue()
print("lifo empty:", lifo.empty())
for i in range(4):
    lifo.put(i)
print("lifo qsize:", lifo.qsize())
print("lifo order:", [lifo.get() for _ in range(4)])
print("lifo empty after drain:", lifo.empty())

# A bounded LifoQueue refuses a put past capacity, same as the FIFO.
bl = queue.LifoQueue(2)
bl.put(1)
bl.put(2)
print("lifo full:", bl.full())
try:
    bl.put_nowait(3)
except Full:
    print("lifo put_nowait full: Full")

# A PriorityQueue returns items smallest first, whatever the insertion order.
pq = queue.PriorityQueue()
for v in [5, 1, 4, 1, 3, 2, 9, 0]:
    pq.put(v)
print("priority order:", [pq.get() for _ in range(8)])

# Tuple priorities: the first element orders, the payload rides along.
tasks = queue.PriorityQueue()
tasks.put((3, "clean up"))
tasks.put((1, "urgent"))
tasks.put((2, "normal"))
print("priority tuples:", [tasks.get()[1] for _ in range(3)])

# get_nowait on an empty PriorityQueue raises queue.Empty.
try:
    queue.PriorityQueue().get_nowait()
except Empty:
    print("priority get_nowait empty: Empty")

# Putting an item the heap cannot order against the rest raises TypeError, and the
# rejected put leaves nothing behind to join on.
mixed = queue.PriorityQueue()
mixed.put(1)
try:
    mixed.put("two")
except TypeError:
    print("priority unorderable: TypeError")
print("priority after reject:", mixed.qsize())

# task_done and join balance a producer against a pool draining the heap in
# priority order. The results are already sorted by the heap, so the print is
# stable without an extra sort.
work = queue.PriorityQueue()
for n in [7, 3, 8, 1, 5]:
    work.put(n)
drained = []
drain_lock = threading.Lock()


def consumer():
    while True:
        try:
            item = work.get_nowait()
        except Empty:
            return
        with drain_lock:
            drained.append(item)
        work.task_done()


pool = [threading.Thread(target=consumer) for _ in range(3)]
for t in pool:
    t.start()
work.join()
for t in pool:
    t.join()
print("priority drained:", sorted(drained))
print("priority all done:", work.qsize())

# The subclasses report their own type names.
print("lifo type:", type(lifo).__name__)
print("priority type:", type(pq).__name__)
