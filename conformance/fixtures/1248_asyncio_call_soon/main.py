import asyncio
import contextvars

log = []
cv = contextvars.ContextVar("cv", default="root")

def note(tag):
    log.append(tag)

async def main():
    loop = asyncio.get_running_loop()

    # call_soon fires on the next iteration, in FIFO order
    loop.call_soon(note, "soon-1")
    loop.call_soon(note, "soon-2")
    await asyncio.sleep(0)
    print("after soon", log)

    # cancel before it runs
    h = loop.call_soon(note, "cancelled")
    print("handle type", type(h).__name__)
    print("cancelled() before", h.cancelled())
    h.cancel()
    print("cancelled() after", h.cancelled())
    await asyncio.sleep(0)
    print("after cancel", log)

    # call_later orders by delay regardless of scheduling order
    log.clear()
    loop.call_later(0.03, note, "late")
    loop.call_later(0.01, note, "early")
    th = loop.call_later(0.02, note, "middle")
    print("timer type", type(th).__name__)
    print("when is float", isinstance(th.when(), float))
    await asyncio.sleep(0.05)
    print("after later", log)

    # call_at with the loop clock
    log.clear()
    loop.call_at(loop.time() + 0.01, note, "at")
    await asyncio.sleep(0.03)
    print("after at", log)

    # callback runs under a copy of the context, no leak
    cv.set("main")
    def mutate():
        cv.set("in-callback")
    loop.call_soon(mutate)
    await asyncio.sleep(0)
    print("cv after callback", cv.get())

    # explicit context
    ctx = contextvars.copy_context()
    ctx.run(lambda: cv.set("explicit"))
    seen = []
    loop.call_soon(lambda: seen.append(cv.get()), context=ctx)
    await asyncio.sleep(0)
    print("explicit ctx", seen)

asyncio.run(main())
print("done")
