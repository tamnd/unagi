import asyncio


async def work(n):
    await asyncio.sleep(0.005)
    return n * 2


async def main():
    # a coroutine becomes a Task
    t = asyncio.ensure_future(work(21))
    print("is task", type(t).__name__)
    print("result", await t)

    # a task passes through unchanged
    t2 = asyncio.create_task(work(3))
    print("task same", asyncio.ensure_future(t2) is t2)
    print("t2", await t2)

    # a future passes through unchanged
    fut = asyncio.get_running_loop().create_future()
    print("future same", asyncio.ensure_future(fut) is fut)
    fut.set_result("ok")
    print("fut", await fut)

    # ensure_future feeds gather a mix of coroutine and task
    a = asyncio.ensure_future(work(5))
    b = asyncio.ensure_future(work(6))
    print("gathered", await asyncio.gather(a, b))


asyncio.run(main())
