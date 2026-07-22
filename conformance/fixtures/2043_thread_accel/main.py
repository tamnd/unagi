# The _thread accelerator exposes the low-level primitives the pure stdlib
# imports by underscore name: the identity call, the two lock constructors, the
# error alias, stack_size, and start_new_thread. reprlib and functools reach for
# get_ident and RLock here, so this is the floor they sit on. A lock hands the
# spawned thread's result back to the main thread so the output is deterministic.
import _thread

print("ident is int:", isinstance(_thread.get_ident(), int))
print("ident positive:", _thread.get_ident() > 0)

lock = _thread.allocate_lock()
print("acquire:", lock.acquire())
print("locked:", lock.locked())
lock.release()
print("locked after release:", lock.locked())

rlock = _thread.RLock()
rlock.acquire()
rlock.acquire()
rlock.release()
rlock.release()
print("rlock reentered")

print("error is RuntimeError:", _thread.error is RuntimeError)
print("stack_size:", _thread.stack_size())

result = []
handoff = _thread.allocate_lock()
handoff.acquire()


def worker(a, b):
    result.append(a + b)
    handoff.release()


_thread.start_new_thread(worker, (40, 2))
handoff.acquire()
print("worker computed:", result[0])

try:
    _thread.start_new_thread(worker, [1, 2])
except TypeError as e:
    print("non-tuple args:", e)
