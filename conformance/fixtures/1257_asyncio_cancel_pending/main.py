import asyncio
from asyncio import CancelledError


# a fire-and-forget task left suspended is cancelled at run teardown, so its except
# and finally run before the loop closes, the _cancel_all_tasks step Runner.close
# takes first
async def forever(tag):
    try:
        while True:
            await asyncio.sleep(1)
    except CancelledError:
        print("cancelled", tag)
        raise
    finally:
        print("finally", tag)


async def spawn_one():
    asyncio.create_task(forever("a"))
    await asyncio.sleep(0)
    print("spawn_one done")


asyncio.run(spawn_one())
print("after spawn_one")


# a task that finishes on its own before teardown is not cancelled
async def quick(tag):
    print("quick", tag)
    return tag


async def spawn_quick():
    t = asyncio.create_task(quick("b"))
    result = await t
    print("awaited", result)


asyncio.run(spawn_quick())
print("after spawn_quick")


# a task that catches its cancellation and returns cleanly is drained without
# stranding, and gather with return_exceptions swallows the outcome
async def swallow(tag):
    try:
        await asyncio.sleep(1)
    except CancelledError:
        print("swallowed", tag)
        return


async def spawn_swallow():
    asyncio.create_task(swallow("c"))
    await asyncio.sleep(0)
    print("spawn_swallow done")


asyncio.run(spawn_swallow())
print("after spawn_swallow")


# the same teardown runs through an explicit Runner reused across a run
with asyncio.Runner() as runner:
    async def leaky(tag):
        try:
            await asyncio.sleep(1)
        finally:
            print("leaky finally", tag)

    async def start(tag):
        asyncio.create_task(leaky(tag))
        await asyncio.sleep(0)
        print("start", tag)

    runner.run(start("d"))

print("done")
