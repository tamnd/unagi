import asyncio


def square(x):
    return x * x


async def setter(fut, val):
    await asyncio.sleep(0.01)
    fut.set_result(val)


async def boom_fut(fut):
    await asyncio.sleep(0.01)
    fut.set_exception(ValueError("bad"))


async def main():
    loop = asyncio.get_running_loop()

    # gather accepts run_in_executor futures directly, no coroutine wrapper.
    results = await asyncio.gather(*[loop.run_in_executor(None, square, i) for i in range(4)])
    print("exec", results)

    # gather over a plain future resolved by another task, mixed with a coroutine.
    fut = loop.create_future()
    asyncio.create_task(setter(fut, 99))

    async def twice(x):
        return x * 2

    print("mixed", await asyncio.gather(fut, twice(5)))

    # A future that resolves with an exception, captured under return_exceptions.
    bad = loop.create_future()
    asyncio.create_task(boom_fut(bad))
    out = await asyncio.gather(bad, return_exceptions=True)
    print("caught", type(out[0]).__name__, out[0])


asyncio.run(main())
