import asyncio


class Counter:
    def __init__(self, n):
        self.n = n
        self.i = 0

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self.i >= self.n:
            raise StopAsyncIteration
        self.i += 1
        await asyncio.sleep(0)
        return self.i


async def double(x):
    await asyncio.sleep(0)
    return x * 2


async def basic():
    gen = (x async for x in Counter(3))
    print("type", type(gen).__name__)
    out = []
    async for v in gen:
        out.append(v)
    print("basic", out)


async def in_comprehension():
    print("comp", [x async for x in (y async for y in Counter(4))])


async def await_in_element():
    print("await elt", [v async for v in (await double(x) for x in range(4))])


async def sync_outer_async_body():
    # No async-for clause, but an await in the element still makes it an
    # async generator built on its own frame.
    print("sync outer", [v async for v in (await double(x) for x in [10, 20, 30])])


async def with_condition():
    got = [v async for v in (x async for x in Counter(6) if x % 2 == 0)]
    print("cond", got)


async def multi_clause():
    pairs = [
        p
        async for p in ((a, b) async for a in Counter(2) for b in range(2))
    ]
    print("multi", pairs)


def make_agen(n):
    # An async genexp is legal in a sync def; only iterating it needs an
    # async context.
    return (x * x async for x in Counter(n))


async def from_sync_def():
    out = []
    async for v in make_agen(3):
        out.append(v)
    print("from sync", out)


async def manual_anext():
    it = (x async for x in Counter(2))
    print("manual", await it.__anext__(), await it.__anext__())
    try:
        await it.__anext__()
    except StopAsyncIteration:
        print("manual stop")


async def gather_over_agen():
    async def collect(tag):
        return [tag + x async for x in Counter(3)]

    results = await asyncio.gather(collect(100), collect(200))
    print("gather", results)


module_gen = (x + 1 async for x in Counter(2))


async def module_level():
    out = []
    async for v in module_gen:
        out.append(v)
    print("module", out)


async def main():
    await basic()
    await in_comprehension()
    await await_in_element()
    await sync_outer_async_body()
    await with_condition()
    await multi_clause()
    await from_sync_def()
    await manual_anext()
    await gather_over_agen()
    await module_level()


asyncio.run(main())
