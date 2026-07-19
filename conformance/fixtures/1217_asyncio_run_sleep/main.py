import asyncio

# asyncio.run drives a coroutine to completion and returns its value. The
# coroutine awaits a child coroutine and a real timer sleep along the way.
async def child(n):
    await asyncio.sleep(0)
    return n * 10

async def main():
    print("start")
    r = await child(3)
    print("child returns", r)
    await asyncio.sleep(0.01)
    print("after sleep")
    total = 0
    for i in range(3):
        total += await child(i)
    print("loop total", total)
    return 42

print("run returns", asyncio.run(main()))

# sleep hands back its result argument when it wakes.
async def with_result():
    return await asyncio.sleep(0, "done")

print("sleep result", asyncio.run(with_result()))

# get_running_loop outside a loop raises RuntimeError.
try:
    asyncio.get_running_loop()
except RuntimeError as e:
    print("no loop:", e)

# get_running_loop inside a loop returns the running loop object.
async def loop_check():
    loop = asyncio.get_running_loop()
    return loop is not None

print("running loop present", asyncio.run(loop_check()))

# A non-coroutine argument is a TypeError.
try:
    asyncio.run(123)
except TypeError as e:
    print("bad run:", e)

# A nested run inside a running loop raises RuntimeError. The inner coroutine is
# closed so it is not reported as never awaited.
async def nester():
    c = child(1)
    try:
        asyncio.run(c)
    except RuntimeError as e:
        print("nested:", e)
    finally:
        c.close()

asyncio.run(nester())

# An exception raised in the coroutine propagates out of run.
async def boom():
    raise ValueError("kaboom")

try:
    asyncio.run(boom())
except ValueError as e:
    print("boom:", e)
