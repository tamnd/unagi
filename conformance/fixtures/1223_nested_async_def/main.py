import asyncio


async def coroutine_capture(base):
    async def add(x):
        await asyncio.sleep(0)
        return base + x

    print("coro call", await add(5))
    print("coro comp", [await add(i) for i in range(3)])


async def async_gen_capture(base):
    async def agen(n):
        for i in range(n):
            await asyncio.sleep(0)
            yield base + i

    got = []
    async for v in agen(3):
        got.append(v)
    print("agen for", got)
    print("agen comp", [x async for x in agen(4)])

    it = agen(2)
    print("agen manual", await it.__anext__(), await it.__anext__())
    try:
        await it.__anext__()
    except StopAsyncIteration:
        print("agen stop")


async def deep_nesting():
    async def a():
        async def b():
            async def c():
                await asyncio.sleep(0)
                return 1

            return await c() + 10

        return await b() + 100

    print("deep", await a())


async def gather_nested():
    async def worker(i):
        await asyncio.sleep(0)
        return i * i

    results = await asyncio.gather(*(worker(i) for i in range(5)))
    print("gather", results)


async def nested_uses_outer_local():
    total = 0

    async def bump(x):
        nonlocal total
        await asyncio.sleep(0)
        total += x
        return total

    await bump(1)
    await bump(2)
    await bump(3)
    print("nonlocal", total)


async def returns_coroutine_factory():
    def make(base):
        async def inner(x):
            await asyncio.sleep(0)
            return base + x

        return inner

    f = make(1000)
    print("factory", await f(1), await f(2))


async def main():
    await coroutine_capture(10)
    await async_gen_capture(20)
    await deep_nesting()
    await gather_nested()
    await nested_uses_outer_local()
    await returns_coroutine_factory()


asyncio.run(main())
