import asyncio


async def notify_ordering():
    cond = asyncio.Condition()
    order = []

    async def waiter(name):
        async with cond:
            order.append("wait " + name)
            await cond.wait()
            order.append("woke " + name)

    w1 = asyncio.create_task(waiter("a"))
    w2 = asyncio.create_task(waiter("b"))
    await asyncio.sleep(0)
    await asyncio.sleep(0)
    async with cond:
        order.append("notify one")
        cond.notify(1)
    await asyncio.sleep(0)
    await asyncio.sleep(0)
    async with cond:
        order.append("notify all")
        cond.notify_all()
    await w1
    await w2
    print(order)


async def wait_for_predicate():
    cond = asyncio.Condition()
    state = {"ready": False}

    async def consumer():
        async with cond:
            await cond.wait_for(lambda: state["ready"])
            return "got it"

    task = asyncio.create_task(consumer())
    await asyncio.sleep(0)
    async with cond:
        state["ready"] = True
        cond.notify_all()
    print(await task)


async def errors_and_states():
    cond = asyncio.Condition()
    print("locked start", cond.locked())
    async with cond:
        print("locked in", cond.locked())
    print("locked out", cond.locked())
    try:
        await cond.wait()
    except RuntimeError as e:
        print("wait err", e)
    try:
        cond.notify()
    except RuntimeError as e:
        print("notify err", e)


async def main():
    await notify_ordering()
    await wait_for_predicate()
    await errors_and_states()


asyncio.run(main())
