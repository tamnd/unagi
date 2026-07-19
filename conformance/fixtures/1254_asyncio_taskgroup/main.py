import asyncio


async def worker(name, delay, val):
    await asyncio.sleep(delay)
    print("done", name)
    return val


async def boom(delay):
    await asyncio.sleep(delay)
    raise ValueError("boom")


async def all_succeed():
    async with asyncio.TaskGroup() as tg:
        t1 = tg.create_task(worker("a", 0.02, 10))
        t2 = tg.create_task(worker("b", 0.01, 20))
    print("results", t1.result(), t2.result())


async def one_fails():
    sibling = None
    try:
        async with asyncio.TaskGroup() as tg:
            sibling = tg.create_task(worker("c", 0.10, 3))
            tg.create_task(boom(0.01))
    except* ValueError as eg:
        print("caught", len(eg.exceptions), str(eg.exceptions[0]))
    print("sibling cancelled", sibling.cancelled())


async def body_raises():
    child = None
    try:
        async with asyncio.TaskGroup() as tg:
            child = tg.create_task(worker("d", 0.10, 4))
            raise KeyError("body")
    except* KeyError as eg:
        print("body group", len(eg.exceptions), str(eg.exceptions[0]))
    print("child cancelled", child.cancelled())


async def empty_group():
    async with asyncio.TaskGroup() as tg:
        pass
    print("empty ok")


async def named():
    async with asyncio.TaskGroup() as tg:
        t = tg.create_task(worker("e", 0.01, 1), name="w-e")
        print("name", t.get_name())


async def after_exit():
    async with asyncio.TaskGroup() as tg:
        tg.create_task(worker("f", 0.01, 1))
    try:
        tg.create_task(worker("g", 0.01, 2))
    except RuntimeError as exc:
        print("after exit", exc)


async def main():
    await empty_group()
    await all_succeed()
    await one_fails()
    await body_raises()
    await named()
    await after_exit()


asyncio.run(main())
