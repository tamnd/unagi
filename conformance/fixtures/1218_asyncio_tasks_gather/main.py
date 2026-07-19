import asyncio

# create_task schedules a coroutine to run concurrently and hands back a Task at
# once. Awaiting the task yields its return value.
async def worker(name, delay, value):
    await asyncio.sleep(delay)
    return value

async def one_task():
    task = asyncio.create_task(worker("w", 0.005, 7))
    result = await task
    print("task result", result)

asyncio.run(one_task())

# Two tasks run concurrently and finish in timer order, not creation order: the
# shorter sleep records first even though it was created second.
async def record(order, name, delay):
    await asyncio.sleep(delay)
    order.append(name)

async def concurrent():
    order = []
    slow = asyncio.create_task(record(order, "slow", 0.02))
    fast = asyncio.create_task(record(order, "fast", 0.005))
    await slow
    await fast
    print("finish order", order)

asyncio.run(concurrent())

# gather runs several awaitables concurrently and returns their results in
# argument order, regardless of the order they finish.
async def gathering():
    results = await asyncio.gather(
        worker("a", 0.02, 1),
        worker("b", 0, 2),
        worker("c", 0.01, 3),
    )
    print("gather results", results)

asyncio.run(gathering())

# gather over an empty argument list resolves to an empty list at once.
async def gather_empty():
    print("gather empty", await asyncio.gather())

asyncio.run(gather_empty())

# With return_exceptions off the first awaitable to raise propagates out of the
# gather and the whole run.
async def failing(delay):
    await asyncio.sleep(delay)
    raise ValueError("boom")

async def gather_raises():
    try:
        await asyncio.gather(worker("a", 0.02, 1), failing(0))
    except ValueError as e:
        print("gather raised", e)

asyncio.run(gather_raises())

# With return_exceptions on each exception takes its slot in the result list
# beside the ordinary results.
async def gather_collects():
    results = await asyncio.gather(
        worker("a", 0, 1),
        failing(0),
        worker("c", 0.005, 3),
        return_exceptions=True,
    )
    print("first", results[0])
    print("second is ValueError", isinstance(results[1], ValueError), results[1])
    print("third", results[2])

asyncio.run(gather_collects())

# create_task off a running loop raises RuntimeError. The coroutine is closed so
# it is not reported as never awaited.
c = worker("x", 0, 0)
try:
    asyncio.create_task(c)
except RuntimeError as e:
    print("no loop:", e)
finally:
    c.close()
