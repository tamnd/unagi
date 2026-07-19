import asyncio

log = []


async def worker(tag, delay):
    await asyncio.sleep(delay)
    log.append(tag)
    return tag.upper()


async def main():
    # two coroutines ordered by their sleep, scheduled on this loop
    t1 = asyncio.create_task(worker("a", 0.02))
    t2 = asyncio.create_task(worker("b", 0.01))
    results = await asyncio.gather(t1, t2)
    return results


loop = asyncio.new_event_loop()
print("running before", loop.is_running())
print("closed before", loop.is_closed())

first = loop.run_until_complete(main())
print("gather", first)
print("log", log)

# the loop drives a second entry, run_until_complete is reusable until close
second = loop.run_until_complete(worker("c", 0.0))
print("second", second)
print("log2", log)

# a bare coroutine result too
print("plain", loop.run_until_complete(worker("d", 0.0)))

print("running after", loop.is_running())
loop.close()
print("closed after", loop.is_closed())

# closing twice is a no-op
loop.close()
print("closed twice", loop.is_closed())

# an exception from the driven coroutine propagates out of run_until_complete
async def boom():
    raise ValueError("kaboom")


loop2 = asyncio.new_event_loop()
try:
    loop2.run_until_complete(boom())
except ValueError as exc:
    print("ValueError", exc)
loop2.close()
