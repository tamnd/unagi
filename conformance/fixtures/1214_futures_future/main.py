import threading
from concurrent.futures import Future, CancelledError, InvalidStateError, TimeoutError

# A fresh future is pending: not running, not done, not cancelled.
f = Future()
print("typename:", type(f).__name__)
print("fresh:", f.running(), f.done(), f.cancelled())

# Non-blocking result on a pending future gives up with TimeoutError.
try:
    f.result(timeout=0)
except TimeoutError:
    print("pending result -> TimeoutError")

# Callbacks registered before the finish fire in order once it finishes, each
# with the future as its only argument.
order = []
f.add_done_callback(lambda fut: order.append(("a", fut is f)))
f.add_done_callback(lambda fut: order.append(("b", fut.result())))
f.set_result(42)
print("finished:", f.running(), f.done(), f.cancelled())
print("result:", f.result(), "exception:", f.exception())
print("callbacks:", order)

# A callback added after the finish fires immediately.
f.add_done_callback(lambda fut: order.append(("c", fut.done())))
print("late callback:", order[-1])

# Setting a result twice raises InvalidStateError; its text carries an address,
# so only the type is stable to print.
try:
    f.set_result(7)
except InvalidStateError as e:
    print("double set:", type(e).__name__)

# A finished-with-exception future re-raises on result and hands the exception
# back on exception.
g = Future()
g.set_exception(ValueError("boom"))
print("exc done:", g.done())
try:
    g.result()
except ValueError as e:
    print("exc result raises:", e)
print("exc via exception():", repr(g.exception()))

# Cancelling a pending future makes it cancelled and done; result then raises
# CancelledError.
c = Future()
print("cancel pending:", c.cancel(), c.cancelled(), c.done())
try:
    c.result()
except CancelledError:
    print("cancelled result -> CancelledError")

# A running or finished future refuses to cancel.
r = Future()
print("set_running:", r.set_running_or_notify_cancel(), "running:", r.running())
print("cancel running:", r.cancel())
r.set_result(1)
print("cancel finished:", r.cancel())

# A cancelled future's set_running_or_notify_cancel reports False.
c2 = Future()
c2.cancel()
print("set_running cancelled:", c2.set_running_or_notify_cancel())

# Cross-thread handoff: a worker sets the result while the main thread blocks in
# result().
h = Future()
h.set_running_or_notify_cancel()

def worker():
    h.set_result("from worker")

th = threading.Thread(target=worker)
th.start()
print("blocking result:", h.result())
th.join()

# The two leaf exceptions share the package Error base through CancelledError.
print("CancelledError is Exception:", issubclass(CancelledError, Exception))
print("InvalidStateError is Exception:", issubclass(InvalidStateError, Exception))
print("CancelledError qualname:", CancelledError.__qualname__)
print("CancelledError module:", CancelledError.__module__)
