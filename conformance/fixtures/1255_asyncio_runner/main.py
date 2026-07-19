import asyncio


async def add(a, b):
    await asyncio.sleep(0)
    return a + b


async def boom():
    await asyncio.sleep(0)
    raise ValueError("boom")


# basic run, result returned; loop reused across runs and equal to get_loop
with asyncio.Runner() as runner:
    print("run", runner.run(add(3, 4)))
    loop1 = runner.get_loop()
    print("second", runner.run(add(10, 20)))
    print("same loop", runner.get_loop() is loop1)
    print("running now", loop1.is_running())

# the loop is closed once the context manager exits
print("closed after with", loop1.is_closed())

# debug flag plumbed through the constructor
with asyncio.Runner(debug=True) as r2:
    print("debug on", r2.get_loop().get_debug())
with asyncio.Runner() as r3:
    print("debug default", r3.get_loop().get_debug())

# run() with a non-awaitable
r4 = asyncio.Runner()
try:
    r4.run(1234)
except TypeError as e:
    print("type error", e)
r4.close()

# close before the loop was ever built is a no-op
r5 = asyncio.Runner()
r5.close()
print("close before init ok")

# run() and get_loop() after close; the running-loop and closed checks fire
# before the argument is type-checked, so a bare int reaches them
r6 = asyncio.Runner()
r6.get_loop()
r6.close()
r6.close()
try:
    r6.run(1)
except RuntimeError as e:
    print("run closed", e)
try:
    r6.get_loop()
except RuntimeError as e:
    print("get_loop closed", e)


# run() cannot nest inside a running loop
async def nested():
    inner = asyncio.Runner()
    try:
        inner.run(1)
    except RuntimeError as e:
        print("nested", e)
    finally:
        inner.close()


asyncio.run(nested())

# an exception raised inside the coroutine propagates out of run()
r7 = asyncio.Runner()
try:
    r7.run(boom())
except ValueError as e:
    print("propagated", e)
r7.close()
print("done")
