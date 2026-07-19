import asyncio


class CM:
    def __init__(self, name):
        self.name = name

    async def __aenter__(self):
        print("enter", self.name)
        await asyncio.sleep(0)
        return self.name

    async def __aexit__(self, et, ev, tb):
        print("exit", self.name, et.__name__ if et else None)
        await asyncio.sleep(0)
        return False


class Suppress:
    async def __aenter__(self):
        return self

    async def __aexit__(self, et, ev, tb):
        await asyncio.sleep(0)
        return True


class SyncOnly:
    def __enter__(self):
        return self

    def __exit__(self, *a):
        return False


class NoEnter:
    async def __aexit__(self, *a):
        return False


async def basic():
    async with CM("a") as x:
        print("body", x)


async def propagates():
    try:
        async with CM("b") as x:
            print("body", x)
            raise ValueError("boom")
    except ValueError as e:
        print("caught", e)


async def suppresses():
    async with Suppress():
        print("in suppress")
        raise KeyError("k")
    print("after suppress")


async def nested():
    async with CM("outer"), CM("inner") as y:
        print("nested body", y)


async def parks():
    for i in range(3):
        async with CM(i):
            if i == 1:
                continue
            if i == 2:
                break
            print("kept", i)


async def protocol_errors():
    for cls in (SyncOnly, NoEnter):
        try:
            async with cls():
                pass
        except TypeError as e:
            print(repr(str(e)))


async def main():
    await basic()
    await propagates()
    await suppresses()
    await nested()
    await parks()
    await protocol_errors()


asyncio.run(main())
