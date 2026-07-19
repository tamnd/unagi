import asyncio
from asyncio import QueueEmpty, QueueFull


async def nowait():
    q = asyncio.Queue()
    print("empty0", q.empty(), "size0", q.qsize(), "full0", q.full())
    q.put_nowait("a")
    q.put_nowait("b")
    print("size2", q.qsize(), "empty2", q.empty())
    print("get1", q.get_nowait())
    print("get2", q.get_nowait())
    print("empty end", q.empty())
    try:
        q.get_nowait()
    except QueueEmpty:
        print("empty err")


async def bounded():
    q = asyncio.Queue(maxsize=1)
    q.put_nowait(1)
    print("bfull", q.full())
    try:
        q.put_nowait(2)
    except QueueFull:
        print("full err")
    print("bget", q.get_nowait(), "bfull2", q.full())


async def producer(q):
    for i in range(3):
        await q.put(i)
    print("produced")


async def consumer(q, out):
    for _ in range(3):
        item = await q.get()
        out.append(item)
        q.task_done()


async def pipeline():
    q = asyncio.Queue()
    out = []
    prod = asyncio.create_task(producer(q))
    cons = asyncio.create_task(consumer(q, out))
    await q.join()
    await prod
    await cons
    print("consumed", out)


async def blocking_put():
    q = asyncio.Queue(maxsize=1)
    order = []

    async def slow_put():
        await q.put("x")
        order.append("put x")
        await q.put("y")
        order.append("put y")

    async def drain():
        await asyncio.sleep(0)
        order.append("get " + str(await q.get()))
        await asyncio.sleep(0)
        order.append("get " + str(await q.get()))

    await asyncio.gather(slow_put(), drain())
    print("order", order)


async def main():
    await nowait()
    await bounded()
    await pipeline()
    await blocking_put()


asyncio.run(main())
