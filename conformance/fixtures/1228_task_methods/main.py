import asyncio


async def worker(x):
    await asyncio.sleep(0.001)
    return x * 2


async def boom():
    await asyncio.sleep(0.001)
    raise ValueError("nope")


async def main():
    t1 = asyncio.create_task(worker(5))
    t2 = asyncio.create_task(worker(7), name="doubler")
    print("t1 name", t1.get_name())
    print("t2 name", t2.get_name())
    print("t1 done before", t1.done())
    print("t1 loop is loop", t1.get_loop() is asyncio.get_running_loop())
    r1 = await t1
    r2 = await t2
    print("results", r1, r2)
    print("t1 done after", t1.done())
    print("t1 result", t1.result())
    print("t2 result", t2.result())
    print("t1 cancelled", t1.cancelled())
    print("t1 exception", t1.exception())
    t1.set_name("renamed")
    print("t1 renamed", t1.get_name())

    tb = asyncio.create_task(boom())
    try:
        await tb
    except ValueError as e:
        print("caught", e)
    print("tb done", tb.done())
    print("tb exception", tb.exception())
    print("tb cancelled", tb.cancelled())


asyncio.run(main())
