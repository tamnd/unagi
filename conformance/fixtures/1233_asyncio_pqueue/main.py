import asyncio


async def lifo():
    q = asyncio.LifoQueue()
    for i in range(3):
        await q.put(i)
    out = []
    while not q.empty():
        out.append(q.get_nowait())
    print("lifo", out)


async def priority():
    q = asyncio.PriorityQueue()
    for v in (3, 1, 4, 1, 5, 9, 2, 6):
        q.put_nowait(v)
    out = []
    while not q.empty():
        out.append(await q.get())
    print("priority", out)


async def priority_tuples():
    q = asyncio.PriorityQueue()
    q.put_nowait((2, "b"))
    q.put_nowait((1, "a"))
    q.put_nowait((2, "a"))
    q.put_nowait((0, "z"))
    out = []
    while not q.empty():
        out.append(q.get_nowait())
    print("tuples", out)


async def lifo_join():
    q = asyncio.LifoQueue()
    order = []

    async def worker():
        for _ in range(4):
            item = await q.get()
            order.append(item)
            q.task_done()

    for i in range(4):
        q.put_nowait(i)
    task = asyncio.create_task(worker())
    await q.join()
    print("lifo drained", order)


async def main():
    await lifo()
    await priority()
    await priority_tuples()
    await lifo_join()


asyncio.run(main())
