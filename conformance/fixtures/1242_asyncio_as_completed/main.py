import asyncio
from asyncio import CancelledError


async def val(x, delay):
    await asyncio.sleep(delay)
    return x


async def main():
    # Plain iteration yields awaitables that resolve in completion order.
    t1 = asyncio.create_task(val(1, 0.03))
    t2 = asyncio.create_task(val(2, 0.01))
    t3 = asyncio.create_task(val(3, 0.02))
    order = []
    for fut in asyncio.as_completed([t1, t2, t3]):
        order.append(await fut)
    print("order", order)

    # A bare coroutine is allowed and wraps into a task.
    order = []
    for fut in asyncio.as_completed([val(7, 0.02), val(8, 0.01)]):
        order.append(await fut)
    print("coros", order)

    # Async iteration yields the underlying futures, already done.
    t1 = asyncio.create_task(val(10, 0.02))
    t2 = asyncio.create_task(val(20, 0.01))
    order = []
    async for fut in asyncio.as_completed([t1, t2]):
        order.append(fut.result())
    print("async", order)

    # An exception surfaces when the resolved awaitable is awaited.
    async def boom():
        await asyncio.sleep(0.01)
        raise ValueError("nope")

    t = asyncio.create_task(boom())
    for fut in asyncio.as_completed([t]):
        try:
            await fut
        except ValueError as e:
            print("raised", str(e))

    # A timeout raises TimeoutError while awaitables are still pending.
    slow = asyncio.create_task(val(99, 0.5))
    try:
        for fut in asyncio.as_completed([slow], timeout=0.02):
            await fut
    except TimeoutError:
        print("timed out")
    slow.cancel()
    try:
        await slow
    except CancelledError:
        pass

    # An empty set of awaitables yields nothing.
    got = [await f for f in asyncio.as_completed([])]
    print("empty", got)

    print("type", type(asyncio.as_completed([])).__name__)


asyncio.run(main())
