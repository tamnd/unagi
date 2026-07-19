import asyncio
from asyncio import BrokenBarrierError


async def main():
    # constructor rejects fewer than one party
    try:
        asyncio.Barrier(0)
    except ValueError as e:
        print("ctor:", e)

    b = asyncio.Barrier(3)
    print("parties", b.parties, "n_waiting", b.n_waiting, "broken", b.broken)

    # three tasks trip the barrier and each gets a unique index in range(3)
    indices = []

    async def party():
        idx = await b.wait()
        indices.append(idx)

    await asyncio.gather(party(), party(), party())
    print("indices", sorted(indices))
    print("after trip n_waiting", b.n_waiting, "broken", b.broken)

    # the barrier is reusable: async with awaits wait() under the hood
    hits = []

    async def entered(name):
        async with b:
            hits.append(name)

    await asyncio.gather(entered("a"), entered("b"), entered("c"))
    print("hits", sorted(hits))

    # abort breaks a barrier that has a task parked mid-fill
    b2 = asyncio.Barrier(2)

    async def waiter():
        try:
            await b2.wait()
            print("waiter passed")
        except BrokenBarrierError as e:
            print("aborted waiter:", e)

    t = asyncio.ensure_future(waiter())
    await asyncio.sleep(0)
    print("parked n_waiting", b2.n_waiting)
    await b2.abort()
    await t
    print("broken after abort", b2.broken)

    # a wait() on an already-broken barrier fails at once
    try:
        await b2.wait()
    except BrokenBarrierError as e:
        print("wait on broken:", e)

    # reset returns a broken-then-emptied barrier to service
    b3 = asyncio.Barrier(2)
    await b3.abort()
    await b3.reset()
    print("reset broken", b3.broken)

    async def pair(tag):
        idx = await b3.wait()
        return tag, idx

    res = await asyncio.gather(pair("x"), pair("y"))
    print("reset indices", sorted(i for _, i in res))


asyncio.run(main())
print("ok")
