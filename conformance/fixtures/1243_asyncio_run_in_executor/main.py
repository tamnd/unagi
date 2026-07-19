import asyncio


def square(x):
    return x * x


def boom():
    raise ValueError("nope")


def greet(name, punct="!"):
    return "hi " + name + punct


async def main():
    loop = asyncio.get_running_loop()

    # A single run_in_executor call runs the function on the default thread pool
    # and awaiting the returned future hands back its result.
    r = await loop.run_in_executor(None, square, 6)
    print("one", r)

    # Several run concurrently on the pool, wrapped in coroutines gather can
    # order: the results line up with the inputs regardless of which worker
    # finishes first.
    async def sq(x):
        return await loop.run_in_executor(None, square, x)

    results = await asyncio.gather(*[sq(i) for i in range(5)])
    print("gather", results)

    # An exception raised in the worker re-raises out of the awaited future.
    try:
        await loop.run_in_executor(None, boom)
    except ValueError as e:
        print("raised", e)

    # to_thread is the convenience wrapper: positional and keyword arguments are
    # forwarded to the call.
    print("to_thread", await asyncio.to_thread(greet, "sam"))
    print("to_thread kw", await asyncio.to_thread(greet, "kim", punct="?"))


asyncio.run(main())
