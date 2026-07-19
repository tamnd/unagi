import asyncio


async def asend_roundtrip():
    async def gen():
        x = yield 1
        print("got", x)
        y = yield x + 1
        print("got", y)
        yield y + 1

    g = gen()
    print("a", await g.asend(None))
    print("b", await g.asend(5))
    print("c", await g.asend(10))


async def anext_and_stop():
    async def gen():
        yield 1
        yield 2

    g = gen()
    print("n1", await g.__anext__())
    print("n2", await g.__anext__())
    try:
        await g.__anext__()
    except StopAsyncIteration:
        print("stopped")


async def athrow_caught():
    async def gen():
        try:
            yield 1
        except ValueError as e:
            print("caught", e)
            yield 99
        yield 3

    g = gen()
    print("t1", await g.__anext__())
    print("t2", await g.athrow(ValueError("boom")))
    print("t3", await g.__anext__())


async def athrow_uncaught():
    async def gen():
        yield 1

    g = gen()
    await g.__anext__()
    try:
        await g.athrow(KeyError("missing"))
    except KeyError as e:
        print("propagated", e)


async def aclose_clean():
    async def gen():
        yield 1
        yield 2

    g = gen()
    print("first", await g.__anext__())
    await g.aclose()
    print("aclosed")
    # aclose is idempotent and iterating a closed gen is exhaustion.
    await g.aclose()
    try:
        await g.__anext__()
    except StopAsyncIteration:
        print("closed exhausted")


async def aclose_runs_finally():
    log = []

    async def gen():
        try:
            yield 1
            yield 2
        finally:
            await asyncio.sleep(0)
            log.append("finally")

    g = gen()
    print("f1", await g.__anext__())
    await g.aclose()
    print("finally log", log)


async def aclose_ignored():
    async def gen():
        try:
            yield 1
        except GeneratorExit:
            yield 99

    g = gen()
    await g.__anext__()
    try:
        await g.aclose()
    except RuntimeError as e:
        print("ignored", e)


async def aclose_never_started():
    async def gen():
        yield 1

    g = gen()
    await g.aclose()
    print("never started closed")


async def async_for_drains():
    async def gen(n):
        for i in range(n):
            await asyncio.sleep(0)
            yield i * i

    out = []
    async for v in gen(5):
        out.append(v)
    print("for", out)


async def main():
    await asend_roundtrip()
    await anext_and_stop()
    await athrow_caught()
    await athrow_uncaught()
    await aclose_clean()
    await aclose_runs_finally()
    await aclose_ignored()
    await aclose_never_started()
    await async_for_drains()


asyncio.run(main())
