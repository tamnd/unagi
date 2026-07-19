import asyncio
import threading

loop = asyncio.new_event_loop()


def run_loop():
    asyncio.set_event_loop(loop)
    loop.run_forever()


worker = threading.Thread(target=run_loop)
worker.start()


async def double(x):
    await asyncio.sleep(0.01)
    return x * 2


async def boom():
    await asyncio.sleep(0.01)
    raise ValueError("nope")


# submit work from the main thread to the loop running on the worker thread
fut = asyncio.run_coroutine_threadsafe(double(21), loop)
print("double", fut.result())
print("done", fut.done())

# a second submission reuses the same running loop
fut2 = asyncio.run_coroutine_threadsafe(double(100), loop)
print("double2", fut2.result())

# an exception raised in the coroutine crosses the thread boundary
efut = asyncio.run_coroutine_threadsafe(boom(), loop)
try:
    efut.result()
except ValueError as e:
    print("caught", e)

# call_soon_threadsafe schedules a plain callback that stops the loop
loop.call_soon_threadsafe(loop.stop)
worker.join()
print("running", loop.is_running())
loop.close()
print("closed", loop.is_closed())
