import asyncio


async def main():
    sem = asyncio.Semaphore(2)
    print("locked0", sem.locked())

    order = []

    async def work(n):
        async with sem:
            order.append(("enter", n))
            await asyncio.sleep(0.01 * n)
            order.append(("exit", n))

    tasks = [asyncio.create_task(work(n)) for n in (1, 2, 3, 4)]
    await asyncio.sleep(0)
    print("locked after 2 acquired", sem.locked())
    await asyncio.gather(*tasks)
    # The enter and exit sequences are each fixed by the graduated sleeps and the
    # semaphore's FIFO wakeup, but their interleaving depends on loop timing under
    # load, so assert the two orders separately rather than the fragile merge.
    print("enters", [n for k, n in order if k == "enter"])
    print("exits", [n for k, n in order if k == "exit"])
    print("locked end", sem.locked())

    s = asyncio.Semaphore(1)
    await s.acquire()
    print("locked1", s.locked())
    s.release()
    print("locked after release", s.locked())

    b = asyncio.BoundedSemaphore(1)
    await b.acquire()
    b.release()
    try:
        b.release()
    except ValueError as e:
        print("bounded err", e)

    try:
        asyncio.Semaphore(-1)
    except ValueError as e:
        print("negative err", e)


asyncio.run(main())
