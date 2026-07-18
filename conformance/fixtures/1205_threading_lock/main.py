import threading

# threading.Lock and RLock over the goroutine-backed thread state. The fixture
# keeps every observable deterministic: it never prints a repr (which carries an
# address) and never races on a timer, so the output is byte for byte stable
# against CPython.

lock = threading.Lock()
print("fresh locked:", lock.locked())
print("acquire:", lock.acquire())
print("held locked:", lock.locked())
print("non-blocking while held:", lock.acquire(blocking=False))
print("timeout=0 while held:", lock.acquire(timeout=0))
lock.release()
print("released locked:", lock.locked())

# Releasing a free lock is a RuntimeError with CPython's exact message.
try:
    lock.release()
except RuntimeError as e:
    print("double release:", e)

# A non-blocking call may not carry a timeout.
try:
    lock.acquire(blocking=False, timeout=1)
except ValueError as e:
    print("nonblocking timeout:", e)

# A negative timeout other than -1 is rejected.
try:
    lock.acquire(timeout=-5)
except ValueError as e:
    print("bad timeout:", e)

# The context manager acquires on the way in and releases on the way out.
with lock:
    print("inside with, locked:", lock.locked())
print("after with, locked:", lock.locked())

# A child cannot take a lock the main thread holds, and can once it is released.
lock.acquire()
took = []


def worker():
    took.append(lock.acquire(blocking=False))


t = threading.Thread(target=worker)
t.start()
t.join()
print("child took held lock:", took[0])
lock.release()

# RLock is reentrant for its owner: the same thread acquires it repeatedly.
rlock = threading.RLock()
print("rlock owned before:", rlock._is_owned())
rlock.acquire()
rlock.acquire()
print("rlock owned after:", rlock._is_owned())
print("rlock locked:", rlock.locked())
rlock.release()
rlock.release()
print("rlock owned released:", rlock._is_owned())

# Releasing an RLock this thread does not own is CPython's exact RuntimeError.
try:
    rlock.release()
except RuntimeError as e:
    print("rlock unowned release:", e)

# The RLock context manager acquires and releases for the running thread.
with rlock:
    print("rlock in with owned:", rlock._is_owned())
print("rlock after with owned:", rlock._is_owned())
