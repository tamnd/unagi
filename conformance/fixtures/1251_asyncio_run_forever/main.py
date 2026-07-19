import asyncio

log = []


def tick(tag):
    log.append(tag)


loop = asyncio.new_event_loop()


def stopper():
    log.append("stop")
    loop.stop()


# callbacks fire in scheduled order, then stop halts the loop
loop.call_soon(tick, "a")
loop.call_soon(tick, "b")
loop.call_soon(stopper)
loop.call_soon(tick, "after-stop")

print("running before", loop.is_running())
loop.run_forever()
print("running after", loop.is_running())
print("log", log)

# the callback queued after stop survives to the next run_forever
log.clear()
loop.call_soon(loop.stop)
loop.run_forever()
print("log2", log)

# timers order run_forever's wakeups by delay
log.clear()
loop.call_later(0.03, tick, "late")
loop.call_later(0.01, tick, "early")
loop.call_later(0.04, loop.stop)
loop.call_later(0.02, tick, "mid")
loop.run_forever()
print("log3", log)

loop.close()
print("closed", loop.is_closed())
