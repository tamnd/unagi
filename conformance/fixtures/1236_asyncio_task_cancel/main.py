import asyncio
from asyncio import CancelledError


async def propagate():
    try:
        await asyncio.sleep(10)
        return "done"
    except CancelledError:
        print("propagate saw cancel")
        raise


async def swallow():
    try:
        await asyncio.sleep(10)
    except CancelledError:
        print("swallow caught")
        return "recovered"


async def with_msg():
    try:
        await asyncio.sleep(10)
    except CancelledError as e:
        print("inner args", e.args)
        raise


async def main():
    t1 = asyncio.create_task(propagate())
    await asyncio.sleep(0)
    print("cancel returned", t1.cancel())
    try:
        await t1
    except CancelledError:
        print("await raised CancelledError")
    print("cancelled()", t1.cancelled())
    print("done()", t1.done())
    print("second cancel", t1.cancel())

    t2 = asyncio.create_task(swallow())
    await asyncio.sleep(0)
    t2.cancel()
    got = await t2
    print("swallow result", got, "cancelled", t2.cancelled())

    t3 = asyncio.create_task(with_msg())
    await asyncio.sleep(0)
    t3.cancel("stop now")
    try:
        await t3
    except CancelledError as e:
        print("outer args", e.args)


asyncio.run(main())
