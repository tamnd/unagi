import asyncio
from asyncio import CancelledError


async def worker(label, n):
    for _ in range(n):
        await asyncio.sleep(0.005)
    return label + "-done"


async def boom():
    await asyncio.sleep(0.005)
    raise ValueError("kaboom")


async def main():
    # plain: shield forwards the inner result
    r = await asyncio.shield(worker("a", 2))
    print("plain", r)

    # cancelling the shield leaves the inner task running to completion
    inner = asyncio.create_task(worker("b", 3))
    outer = asyncio.shield(inner)
    await asyncio.sleep(0.001)
    outer.cancel()
    try:
        await outer
    except CancelledError:
        print("outer cancelled")
    print("inner cancelled", inner.cancelled())
    print("inner result", await inner)

    # an inner exception propagates through the shield
    try:
        await asyncio.shield(boom())
    except ValueError as e:
        print("inner error", e)

    # cancelling the inner task cancels the shield
    inner2 = asyncio.create_task(worker("c", 5))
    outer2 = asyncio.shield(inner2)
    await asyncio.sleep(0.001)
    inner2.cancel()
    try:
        await outer2
    except CancelledError:
        print("shield saw inner cancel")


asyncio.run(main())
