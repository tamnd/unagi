"""The _queue C extension backs the public queue module: queue.py does
`from _queue import Empty, SimpleQueue` and only falls back to a pure-Python
SimpleQueue when that import fails. This exercises _queue directly -- the
unbounded FIFO SimpleQueue and the Empty exception -- and confirms CPython's
cross-module identity, that queue.Empty is _queue.Empty and
queue.SimpleQueue is _queue.SimpleQueue. Every output here is deterministic and
platform-invariant."""

import _queue
import queue

q = _queue.SimpleQueue()
for v in (1, 2, 3):
    q.put(v)
print("qsize:", q.qsize(), "empty:", q.empty())

print("get:", q.get(), q.get())
print("get_nowait:", q.get_nowait())
print("empty now:", q.empty())

# A drained SimpleQueue raises _queue.Empty on a non-blocking get.
try:
    q.get_nowait()
except _queue.Empty:
    print("Empty raised on drained queue")

# queue.py re-exports both names from _queue, so they are the very same objects.
print("Empty identity:", queue.Empty is _queue.Empty)
print("SimpleQueue identity:", queue.SimpleQueue is _queue.SimpleQueue)
print("Empty is subclass of Exception:", issubclass(_queue.Empty, Exception))

# Empty carries its C-module name.
print("Empty module:", _queue.Empty.__module__)
print("Empty qualname:", _queue.Empty.__qualname__)
