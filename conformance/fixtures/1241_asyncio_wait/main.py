import asyncio
from asyncio import FIRST_COMPLETED, FIRST_EXCEPTION, ALL_COMPLETED


async def val(x, delay):
    await asyncio.sleep(delay)
    return x


async def boom(delay):
    await asyncio.sleep(delay)
    raise ValueError("boom")


async def main():
    # ALL_COMPLETED is the default: every task finishes before wait returns.
    t1 = asyncio.create_task(val(1, 0.01))
    t2 = asyncio.create_task(val(2, 0.02))
    done, pending = await asyncio.wait([t1, t2])
    print("all", sorted(t.result() for t in done), len(pending))

    # FIRST_COMPLETED returns as soon as one task is done.
    t1 = asyncio.create_task(val(1, 0.01))
    t2 = asyncio.create_task(val(2, 0.1))
    done, pending = await asyncio.wait([t1, t2], return_when=FIRST_COMPLETED)
    print("first_completed", len(done), len(pending))
    print("winner", next(iter(done)).result())
    await asyncio.wait([t1, t2])

    # FIRST_EXCEPTION returns as soon as a task raises.
    t1 = asyncio.create_task(val(1, 0.1))
    t2 = asyncio.create_task(boom(0.01))
    done, pending = await asyncio.wait([t1, t2], return_when=FIRST_EXCEPTION)
    print("first_exception", len(done), len(pending))
    for t in done:
        print("raised", type(t.exception()).__name__)
    await asyncio.wait([t1, t2])

    # A timeout returns whatever is done and leaves the rest pending, uncancelled.
    t1 = asyncio.create_task(val(1, 0.01))
    t2 = asyncio.create_task(val(2, 0.5))
    done, pending = await asyncio.wait([t1, t2], timeout=0.05)
    print("timeout", len(done), len(pending), "cancelled", t2.cancelled())
    await asyncio.wait([t1, t2])

    # Duplicate arguments collapse to one watched future.
    t1 = asyncio.create_task(val(9, 0.01))
    done, pending = await asyncio.wait([t1, t1])
    print("dedup", len(done), len(pending))

    # A plain Future is a valid argument.
    fut = asyncio.get_running_loop().create_future()
    fut.set_result(7)
    done, pending = await asyncio.wait([fut])
    print("future", next(iter(done)).result())

    # An empty set of awaitables is a ValueError.
    try:
        await asyncio.wait([])
    except ValueError as e:
        print("empty", str(e))

    # Bare coroutines are forbidden.
    c = val(0, 0)
    try:
        await asyncio.wait([c])
    except TypeError as e:
        print("coro", str(e))
    finally:
        c.close()

    # An invalid return_when is a ValueError.
    t1 = asyncio.create_task(val(1, 0.01))
    try:
        await asyncio.wait([t1], return_when="NOPE")
    except ValueError as e:
        print("badrw", str(e))
    await asyncio.wait([t1])

    # A single future in place of a list is a TypeError.
    t1 = asyncio.create_task(val(1, 0.01))
    try:
        await asyncio.wait(t1)
    except TypeError as e:
        print("futarg", str(e))
    await t1

    print("ALL_COMPLETED", ALL_COMPLETED)


asyncio.run(main())
