import asyncio


async def resolve(fut):
    await asyncio.sleep(0)
    fut.set_result("resolved")


async def main():
    loop = asyncio.get_event_loop()
    print("running", loop.is_running())
    print("closed", loop.is_closed())
    print("debug", loop.get_debug())
    t1 = loop.time()
    print("time is float", isinstance(t1, float))
    await asyncio.sleep(0)
    t2 = loop.time()
    print("time monotonic", t2 >= t1)

    fut = loop.create_future()
    print("future pending", not fut.done())
    asyncio.create_task(resolve(fut))
    print("future result", await fut)

    fut2 = loop.create_future()
    print("same loop", fut2.get_loop() is loop)


asyncio.run(main())
