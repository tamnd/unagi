import asyncio


class Doubler:
    def __init__(self, value):
        self.value = value

    def __await__(self):
        yield
        return self.value * 2


class Sleeper:
    def __init__(self, label):
        self.label = label

    def __await__(self):
        result = yield from asyncio.sleep(0, self.label).__await__()
        return result + "!"


class Chain:
    def __await__(self):
        a = yield from Doubler(3).__await__()
        b = yield from Doubler(a).__await__()
        return b


class FutWaiter:
    def __init__(self, fut):
        self.fut = fut

    def __await__(self):
        r = yield from self.fut.__await__()
        return r.upper()


async def coro(n):
    await asyncio.sleep(0)
    return n + 100


async def resolve(fut, value):
    await asyncio.sleep(0)
    fut.set_result(value)


async def main():
    print("doubler", await Doubler(21))
    print("sleeper", await Sleeper("slept"))
    print("chain", await Chain())

    # A coroutine drives through its own __await__.
    c = coro(5)
    print("coro await", await c)

    # A Future awaited through its explicit __await__ iterator.
    fut = asyncio.Future()
    asyncio.create_task(resolve(fut, "fut-done"))
    print("future", await FutWaiter(fut))

    # gather keeps working alongside the delegating awaitables.
    print("gather", await asyncio.gather(coro(1), coro(2)))


asyncio.run(main())
