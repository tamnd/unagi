import asyncio


async def main():
    lock = asyncio.Lock()
    print("locked0", lock.locked())
    await lock.acquire()
    print("locked1", lock.locked())

    order = []

    async def contender(n):
        async with lock:
            order.append(n)
            await asyncio.sleep(0.001)

    t1 = asyncio.create_task(contender(1))
    t2 = asyncio.create_task(contender(2))
    await asyncio.sleep(0.001)
    print("held while contended", lock.locked())
    lock.release()
    print("free right after release", lock.locked())
    await asyncio.gather(t1, t2)
    print("order", order)
    print("locked end", lock.locked())

    try:
        lock.release()
    except RuntimeError as e:
        print("release err", e)

    fresh = asyncio.Lock()
    async with fresh:
        print("in ctx", fresh.locked())
    print("after ctx", fresh.locked())


asyncio.run(main())
