# An async generator runs on the same frame a coroutine uses, so with no event
# loop a step runs to its next yield the first time it is driven. Driving
# ag.__anext__() or ag.asend(v) with send(None) advances one step: it raises
# StopIteration carrying the yielded value, or StopAsyncIteration once the body
# has returned.


def step(aw):
    try:
        aw.send(None)
    except StopIteration as e:
        return ("yield", e.value)
    except StopAsyncIteration:
        return ("done", None)
    raise RuntimeError("async step suspended with no event loop")


async def counter():
    x = yield 1
    y = yield x + 10
    yield y + 100


ag = counter()
print("type:", type(ag).__name__)
print("repr ok:", repr(ag).startswith("<async_generator object counter at 0x"))
print("aiter is self:", ag.__aiter__() is ag)

first = ag.__anext__()
print("anext type:", type(first).__name__)
print(step(first))
print(step(ag.asend(5)))
print(step(ag.asend(7)))
print(step(ag.__anext__()))

# A just-started async generator cannot receive a sent value at a yield that
# has not happened yet.
fresh = counter()
try:
    fresh.asend(9).send(None)
except TypeError as e:
    print("asend-start:", e)

# An async generator is driven with async for, not the iterator protocol.
try:
    iter(counter())
except TypeError as e:
    print("iter:", e)
