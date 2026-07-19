import asyncio
from asyncio import CancelledError


async def slow(label):
    try:
        await asyncio.sleep(10)
    except CancelledError:
        print(label, "cancelled")
        raise
    return "done"


async def main():
    # completes within the deadline
    async with asyncio.timeout(1.0):
        await asyncio.sleep(0.01)
    print("within ok")

    # deadline fires, CancelledError becomes TimeoutError
    try:
        async with asyncio.timeout(0.02):
            await slow("first")
    except TimeoutError:
        print("timed out")

    # None disables the timeout
    async with asyncio.timeout(None) as cm:
        print("none when", cm.when())
        await asyncio.sleep(0.01)
    print("none ok")

    # expired() reports the transition
    cm = asyncio.timeout(0.02)
    print("expired created", cm.expired())
    try:
        async with cm:
            print("expired active", cm.expired())
            await asyncio.sleep(10)
    except TimeoutError:
        pass
    print("expired after", cm.expired())

    # timeout_at with an absolute deadline
    loop = asyncio.get_running_loop()
    try:
        async with asyncio.timeout_at(loop.time() + 0.02):
            await asyncio.sleep(10)
    except TimeoutError:
        print("timeout_at fired")

    # reschedule extends the deadline so the body finishes
    async with asyncio.timeout(0.02) as cm:
        cm.reschedule(loop.time() + 1.0)
        await asyncio.sleep(0.01)
    print("rescheduled ok", cm.expired())

    # a real error inside is not swallowed
    try:
        async with asyncio.timeout(1.0):
            raise ValueError("boom")
    except ValueError as e:
        print("inner", e)

    # type name and TimeoutError identity
    print("type", type(asyncio.timeout(1.0)).__name__)
    print("timeout is builtin", asyncio.TimeoutError is TimeoutError)


asyncio.run(main())
