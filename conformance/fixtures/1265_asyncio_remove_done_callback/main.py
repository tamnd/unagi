import asyncio


async def main():
    loop = asyncio.get_running_loop()

    fired = []

    def cb1(fut):
        fired.append("cb1")

    def cb2(fut):
        fired.append("cb2")

    fut = loop.create_future()
    # cb1 registered twice, cb2 once
    fut.add_done_callback(cb1)
    fut.add_done_callback(cb1)
    fut.add_done_callback(cb2)

    # both cb1 registrations drop together
    print("removed", fut.remove_done_callback(cb1))
    # nothing left to remove for cb1
    print("removed again", fut.remove_done_callback(cb1))

    fut.set_result(0)
    await asyncio.sleep(0)
    # only cb2 survived to fire
    print("fired", fired)

    # a Task exposes the same method
    async def child():
        return 1

    t = asyncio.ensure_future(child())

    def tcb(task):
        pass

    t.add_done_callback(tcb)
    print("task removed", t.remove_done_callback(tcb))
    print("task removed again", t.remove_done_callback(tcb))

    # calling with no argument is a TypeError on both
    try:
        fut.remove_done_callback()
    except TypeError as e:
        print("fut arity", e)
    try:
        t.remove_done_callback()
    except TypeError as e:
        print("task arity", e)
    await t


asyncio.run(main())
print("ok")
