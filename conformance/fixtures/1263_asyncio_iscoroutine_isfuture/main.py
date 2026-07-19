import asyncio


async def child():
    return 1


async def main():
    coro = child()

    # a coroutine object is a coroutine but not a future
    print("coro iscoroutine", asyncio.iscoroutine(coro))
    print("coro isfuture", asyncio.isfuture(coro))

    # a task is a future but not a coroutine
    t = asyncio.ensure_future(coro)
    print("task iscoroutine", asyncio.iscoroutine(t))
    print("task isfuture", asyncio.isfuture(t))

    # a bare Future is a future
    fut = asyncio.get_running_loop().create_future()
    print("future iscoroutine", asyncio.iscoroutine(fut))
    print("future isfuture", asyncio.isfuture(fut))

    # ordinary values are neither
    for v in (1, "x", None, [1, 2]):
        print("plain", asyncio.iscoroutine(v), asyncio.isfuture(v))

    fut.set_result(0)
    await t


asyncio.run(main())
print("ok")
