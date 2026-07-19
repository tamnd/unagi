import asyncio


# an async generator left suspended at a yield is acloseed at loop teardown, so
# its finally still runs, the firstiter/finalizer contract asyncio.run honors
async def ticker():
    try:
        n = 0
        while True:
            yield n
            n += 1
    finally:
        print("ticker closed")


async def use_ticker():
    async for v in ticker():
        print("tick", v)
        if v == 2:
            break
    print("after ticker")


asyncio.run(use_ticker())


# an async generator run to exhaustion needs no shutdown finalize; its finally has
# already run, so shutdown does not touch it again
async def once():
    try:
        yield "only"
    finally:
        print("once closed")


async def use_once():
    async for v in once():
        print("value", v)
    print("after once")


asyncio.run(use_once())


# explicit loop.shutdown_asyncgens on a Runner closes a generator suspended after a
# partial manual drive
async def pair():
    try:
        yield 100
        yield 200
    finally:
        print("pair closed")


async def drive_pair():
    ait = pair()
    print("first", await ait.__anext__())
    await asyncio.get_running_loop().shutdown_asyncgens()
    print("after explicit shutdown")


asyncio.run(drive_pair())


# a Runner reused across runs finalizes each run's generators at close
with asyncio.Runner() as runner:
    async def leaky(tag):
        try:
            yield tag
        finally:
            print("leaky closed", tag)

    async def start(tag):
        ait = leaky(tag)
        print("got", await ait.__anext__())

    runner.run(start("a"))

print("done")
