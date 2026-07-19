import asyncio
from asyncio import CancelledError
from asyncio import TimeoutError as ATimeoutError


async def quick():
    await asyncio.sleep(0.01)
    return "quick"


async def slow():
    try:
        await asyncio.sleep(10)
    except CancelledError:
        print("slow cancelled")
        raise
    return "slow"


async def boom():
    await asyncio.sleep(0.01)
    raise ValueError("kaboom")


async def main():
    got = await asyncio.wait_for(quick(), timeout=1.0)
    print("within", got)

    try:
        await asyncio.wait_for(slow(), timeout=0.02)
    except ATimeoutError:
        print("timed out")

    got = await asyncio.wait_for(quick(), timeout=None)
    print("no timeout", got)

    try:
        await asyncio.wait_for(boom(), timeout=1.0)
    except ValueError as e:
        print("inner error", e)


asyncio.run(main())
print("TimeoutError is builtin", asyncio.TimeoutError is TimeoutError)
