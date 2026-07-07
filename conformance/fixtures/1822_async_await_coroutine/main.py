# async def builds a coroutine, which runs on the same frame a generator uses.
# await drives another awaitable inline. With no event loop yet, a coroutine that
# only awaits other coroutines runs to completion the first time it is driven, so
# send(None) walks the whole await chain and the StopIteration a finished
# coroutine raises carries the return value.


def run(coro):
    # Drive a coroutine to completion with no event loop: send None once and read
    # the value off the StopIteration a finished coroutine raises.
    try:
        coro.send(None)
    except StopIteration as e:
        return e.value
    raise RuntimeError("coroutine suspended with no event loop")


async def inc(x):
    return x + 1


async def add(a, b):
    x = await inc(a)
    y = await inc(b)
    return x + y


async def total():
    return await add(3, 4) + await inc(10)


c = total()
print("type:", type(c).__name__)
print("repr ok:", repr(c).startswith("<coroutine object total at 0x"))
print("total:", run(c))

# A coroutine is not iterable, and a plain value cannot be awaited.
d = inc(1)
try:
    iter(d)
except TypeError as e:
    print("iter:", e)
print("driven:", run(d))


async def bad():
    return await 5


b = bad()
try:
    run(b)
except TypeError as e:
    print("await int:", e)
