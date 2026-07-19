import asyncio
from asyncio import CancelledError, InvalidStateError

# A Future is a result box one task resolves and another awaits. The setter runs
# concurrently as a task and fills the future; awaiting it hands back the value.
async def setter(fut, value):
    await asyncio.sleep(0.005)
    fut.set_result(value)

async def await_future():
    fut = asyncio.Future()
    print("pending repr", repr(fut))
    print("done before", fut.done())
    asyncio.create_task(setter(fut, 42))
    result = await fut
    print("awaited", result)
    print("done after", fut.done(), "cancelled", fut.cancelled())
    print("finished repr", repr(fut))
    print("result()", fut.result())
    print("exception()", fut.exception())

asyncio.run(await_future())

# result and set_result guard the future's state: reading a result before one is
# set is InvalidStateError, and resolving a done future twice is too.
async def state_guards():
    fut = asyncio.Future()
    try:
        fut.result()
    except InvalidStateError as e:
        print("result unset", e)
    try:
        fut.exception()
    except InvalidStateError as e:
        print("exception unset", e)
    fut.set_result(1)
    try:
        fut.set_result(2)
    except InvalidStateError as e:
        print("set twice", e)

asyncio.run(state_guards())

# Cancelling a pending future resolves it with CancelledError, so awaiting it
# re-raises. CancelledError is a BaseException, so a bare except Exception does
# not swallow it.
async def cancelling():
    fut = asyncio.Future()
    print("cancel pending", fut.cancel())
    print("cancelled", fut.cancelled(), "done", fut.done())
    print("cancel again", fut.cancel())
    print("cancelled repr", repr(fut))
    try:
        await fut
    except CancelledError:
        print("await saw CancelledError")
    print("is BaseException", issubclass(asyncio.CancelledError, BaseException))
    print("is Exception", issubclass(asyncio.CancelledError, Exception))

asyncio.run(cancelling())

# set_exception resolves the future with an exception; awaiting it re-raises, and
# exception() reads it back without raising.
async def with_exception():
    fut = asyncio.Future()
    fut.set_exception(ValueError("boom"))
    print("exc repr", repr(fut))
    print("exception()", repr(fut.exception()))
    try:
        await fut
    except ValueError as e:
        print("await raised", e)

asyncio.run(with_exception())

# A done callback runs with the future as its argument once it is resolved, on
# the loop, after the resolving task yields.
async def done_callbacks():
    fut = asyncio.Future()
    seen = []
    fut.add_done_callback(lambda f: seen.append(f.result()))
    asyncio.create_task(setter(fut, 7))
    await fut
    await asyncio.sleep(0)
    print("callback saw", seen)

asyncio.run(done_callbacks())
