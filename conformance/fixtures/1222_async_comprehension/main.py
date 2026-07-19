import asyncio


async def arange(n):
    for i in range(n):
        await asyncio.sleep(0)
        yield i


async def adouble(x):
    await asyncio.sleep(0)
    return x * 2


class ARange:
    def __init__(self, n):
        self.n = n
        self.i = 0

    def __aiter__(self):
        return self

    async def __anext__(self):
        await asyncio.sleep(0)
        if self.i >= self.n:
            raise StopAsyncIteration
        v = self.i
        self.i += 1
        return v


async def lists():
    print("list", [x async for x in arange(4)])
    print("list class", [x async for x in ARange(3)])
    print("list cond", [x async for x in arange(6) if x % 2 == 0])
    print("list two conds", [x async for x in arange(10) if x % 2 == 0 if x > 3])
    print("list empty", [x async for x in arange(0)])


async def sets():
    print("set", sorted({x % 3 async for x in arange(9)}))


async def dicts():
    print("dict", {x: x * x async for x in arange(5)})
    print("dict cond", {x: x + 1 async for x in arange(6) if x % 2})


async def await_in_element():
    print("await elt", [await adouble(x) async for x in arange(4)])
    print("await cond", [x async for x in arange(6) if await adouble(x) < 6])
    print("await dict", {x: await adouble(x) async for x in arange(3)})


async def multi_clause():
    print("async async", [(a, b) async for a in arange(2) async for b in arange(2)])
    print("async sync", [x + y async for x in arange(2) for y in range(3)])
    print("sync async", [x + y for x in range(2) async for y in arange(3)])


async def nested():
    print("nested", [[y async for y in arange(a)] async for a in arange(4)])


async def sync_comp_in_async():
    # A comprehension with neither an async clause nor an await stays an
    # ordinary inlined loop even inside an async def.
    print("sync", [x * x for x in range(4)])


async def main():
    await lists()
    await sets()
    await dicts()
    await await_in_element()
    await multi_clause()
    await nested()
    await sync_comp_in_async()


asyncio.run(main())
