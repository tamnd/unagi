import asyncio


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


class NoAiter:
    pass


class NoAnext:
    def __aiter__(self):
        return self


async def sleepy_gen(n):
    for i in range(n):
        await asyncio.sleep(0)
        yield i * 10


async def plain_gen():
    yield "a"
    yield "b"
    yield "c"


async def over_class():
    async for x in ARange(3):
        print("class", x)
    else:
        print("class else")


async def over_empty():
    async for x in ARange(0):
        print("unreachable", x)
    else:
        print("empty else")


async def over_sleepy_gen():
    total = 0
    async for v in sleepy_gen(4):
        total += v
    print("sleepy total", total)


async def over_plain_gen():
    async for v in plain_gen():
        print("plain", v)


async def with_break():
    async for x in ARange(5):
        if x == 3:
            break
        print("break", x)
    else:
        print("unreachable else")


async def with_continue():
    async for x in ARange(5):
        if x % 2 == 0:
            continue
        print("continue", x)


async def nested():
    async for a in ARange(2):
        async for b in ARange(2):
            print("nested", a, b)


async def manual_anext():
    g = sleepy_gen(2)
    while True:
        try:
            v = await g.__anext__()
        except StopAsyncIteration:
            break
        print("manual", v)


async def returns_from_loop():
    async for x in ARange(4):
        if x == 2:
            return x
    return -1


async def protocol_errors():
    for cls in (NoAiter, NoAnext):
        try:
            async for x in cls():
                print(x)
        except TypeError as e:
            print("typeerror", str(e))


async def main():
    await over_class()
    await over_empty()
    await over_sleepy_gen()
    await over_plain_gen()
    await with_break()
    await with_continue()
    await nested()
    await manual_anext()
    print("returned", await returns_from_loop())
    await protocol_errors()


asyncio.run(main())
