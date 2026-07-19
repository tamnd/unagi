from queue import Queue, LifoQueue, PriorityQueue, ShutDown, Empty


# Graceful shutdown: queued items stay drainable, but new puts and a get past the
# last item raise ShutDown. The queue is not emptied, so qsize keeps its count.
q = Queue()
q.put("a")
q.put("b")
q.shutdown()
print("graceful qsize", q.qsize(), "empty", q.empty())
print("get a", q.get())
print("get b", q.get())
try:
    q.get_nowait()
except ShutDown:
    print("get past end raised ShutDown")
try:
    q.put("c")
except ShutDown:
    print("put raised ShutDown")
try:
    q.put_nowait("d")
except ShutDown:
    print("put_nowait raised ShutDown")

# A get with items left still returns them even after shutdown, and only the
# empty get raises. get on a bounded queue past capacity never blocks here since
# we drain before refilling.
b = Queue(maxsize=2)
b.put(1)
b.shutdown()
print("bounded get", b.get())
try:
    b.put(2)
except ShutDown:
    print("bounded put raised ShutDown")

# Immediate shutdown drops the pending items and balances the unfinished count so
# join returns at once; a following get raises ShutDown on the emptied queue.
q2 = Queue()
q2.put(10)
q2.put(20)
print("q2 before", q2.qsize(), "unfinished join would block")
q2.shutdown(immediate=True)
print("q2 after immediate qsize", q2.qsize(), "empty", q2.empty())
q2.join()
print("q2 join returned")
try:
    q2.get_nowait()
except ShutDown:
    print("q2 get raised ShutDown")

# immediate passed positionally reads the same way.
q3 = Queue()
q3.put(1)
q3.shutdown(True)
print("q3 after positional immediate qsize", q3.qsize())

# LifoQueue and PriorityQueue share the shutdown machinery: graceful drain keeps
# each discipline, and the empty get then raises ShutDown.
lifo = LifoQueue()
lifo.put(1)
lifo.put(2)
lifo.shutdown()
print("lifo drain", lifo.get(), lifo.get())
try:
    lifo.get_nowait()
except ShutDown:
    print("lifo empty raised ShutDown")

pq = PriorityQueue()
pq.put(3)
pq.put(1)
pq.put(2)
pq.shutdown()
print("pq drain", pq.get(), pq.get(), pq.get())
try:
    pq.get_nowait()
except ShutDown:
    print("pq empty raised ShutDown")

# task_done and join still work up to the shutdown point on a graceful queue.
tq = Queue()
tq.put("x")
tq.shutdown()
print("tq get", tq.get())
tq.task_done()
tq.join()
print("tq join returned after task_done")

# Empty is unaffected on a queue that was never shut down.
plain = Queue()
try:
    plain.get_nowait()
except Empty:
    print("plain get_nowait raised Empty")

print("ShutDown is Exception subclass", issubclass(ShutDown, Exception))
print("ok")
