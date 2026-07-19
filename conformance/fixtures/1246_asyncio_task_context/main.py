import asyncio
import contextvars

request_id = contextvars.ContextVar("request_id", default="none")


async def worker(tag):
    # Each task inherits a copy of the context current when it was created, so it
    # sees the value the parent set before creating it, then its own set stays
    # confined to this task.
    seen = request_id.get()
    request_id.set(tag)
    await asyncio.sleep(0)
    return seen, request_id.get()


async def main():
    request_id.set("root")
    # Sibling tasks each copy the root context; neither sees the other's set.
    a = asyncio.create_task(worker("a"))
    b = asyncio.create_task(worker("b"))
    ra = await a
    rb = await b
    print("a", ra)
    print("b", rb)
    # The parent's own value is untouched by the tasks.
    print("parent", request_id.get())

    # A task set persists across its own awaits but a fresh nested task copies
    # the current (already mutated) context.
    async def outer():
        request_id.set("outer")
        inner = asyncio.create_task(inner_task())
        r = await inner
        return request_id.get(), r

    async def inner_task():
        before = request_id.get()
        request_id.set("inner")
        return before, request_id.get()

    print("nested", await asyncio.create_task(outer()))
    print("parent after nested", request_id.get())

    # gather wraps coroutines as tasks, so each still gets its own context copy.
    results = await asyncio.gather(worker("g1"), worker("g2"))
    print("gather", results)
    print("parent after gather", request_id.get())


asyncio.run(main())
