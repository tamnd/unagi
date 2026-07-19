import asyncio
from asyncio import QueueShutDown


async def main():
    # a graceful shutdown lets get drain the items already queued, then raises
    q = asyncio.Queue()
    q.put_nowait("a")
    q.put_nowait("b")
    q.shutdown()
    print("after shutdown empty", q.empty(), "qsize", q.qsize())
    print("get", await q.get())
    print("get", await q.get())
    try:
        await q.get()
    except QueueShutDown:
        print("get raised QueueShutDown")

    # put on a shut-down queue raises at once, blocking or not
    try:
        await q.put("c")
    except QueueShutDown:
        print("put raised QueueShutDown")
    try:
        q.put_nowait("d")
    except QueueShutDown:
        print("put_nowait raised QueueShutDown")

    # immediate shutdown drops the pending items and unblocks join
    q2 = asyncio.Queue()
    q2.put_nowait(1)
    q2.put_nowait(2)
    print("q2 qsize before", q2.qsize())
    q2.shutdown(immediate=True)
    print("q2 empty after immediate", q2.empty())
    await q2.join()
    print("q2 join returned")
    try:
        q2.get_nowait()
    except QueueShutDown:
        print("q2 get_nowait raised QueueShutDown")

    # a getter parked on an empty queue is woken by shutdown and raises
    q3 = asyncio.Queue()

    async def consumer():
        try:
            await q3.get()
            print("consumer got item")
        except QueueShutDown:
            print("consumer saw QueueShutDown")

    t = asyncio.ensure_future(consumer())
    await asyncio.sleep(0)
    q3.shutdown()
    await t

    # a putter parked on a full queue is woken by shutdown and raises
    q4 = asyncio.Queue(1)
    q4.put_nowait("x")

    async def producer():
        try:
            await q4.put("y")
            print("producer put item")
        except QueueShutDown:
            print("producer saw QueueShutDown")

    t2 = asyncio.ensure_future(producer())
    await asyncio.sleep(0)
    q4.shutdown()
    await t2

    print("subclass of Exception", issubclass(QueueShutDown, Exception))


asyncio.run(main())
print("ok")
