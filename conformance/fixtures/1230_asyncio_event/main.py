import asyncio


async def main():
    ev = asyncio.Event()
    print("set0", ev.is_set())

    log = []

    async def waiter(n):
        log.append(("wait", n))
        r = await ev.wait()
        log.append(("woke", n, r))

    t1 = asyncio.create_task(waiter(1))
    t2 = asyncio.create_task(waiter(2))
    await asyncio.sleep(0.001)
    print("before set", ev.is_set())
    ev.set()
    print("after set", ev.is_set())
    await asyncio.gather(t1, t2)
    print("log", log)

    r = await ev.wait()
    print("immediate", r)
    ev.clear()
    print("cleared", ev.is_set())


asyncio.run(main())
