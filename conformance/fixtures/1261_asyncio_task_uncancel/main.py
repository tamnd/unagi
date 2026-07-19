import asyncio
from asyncio import CancelledError


async def child():
    try:
        await asyncio.sleep(10)
    except CancelledError:
        pass


async def main():
    t = asyncio.ensure_future(child())
    await asyncio.sleep(0)

    # a fresh task has no cancellation requested
    print("fresh", t.cancelling())

    # each cancel request bumps the count
    print("cancel1", t.cancel())
    print("cancelling", t.cancelling())
    print("cancel2", t.cancel())
    print("cancelling", t.cancelling())

    # uncancel walks it back down, floored at zero
    print("uncancel", t.uncancel())
    print("uncancel", t.uncancel())
    print("uncancel", t.uncancel())

    await t

    # a done task refuses further cancels and leaves the count alone
    print("done cancel", t.cancel())
    print("done cancelling", t.cancelling())


asyncio.run(main())
print("ok")
