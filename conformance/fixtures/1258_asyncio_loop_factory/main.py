import asyncio


async def main(tag):
    await asyncio.sleep(0)
    print(tag, asyncio.get_running_loop() is not None)
    return tag


# loop_factory on asyncio.run builds the loop through the given callable
print("run", asyncio.run(main("run"), loop_factory=asyncio.new_event_loop))

# loop_factory on a Runner: the loop it built is the one get_loop reports, and the
# factory is called once and reused across runs
made = []


def factory():
    loop = asyncio.new_event_loop()
    made.append(loop)
    return loop


with asyncio.Runner(loop_factory=factory) as r:
    print("first", r.run(main("first")))
    print("second", r.run(main("second")))
    print("used factory loop", r.get_loop() is made[0])
    print("factory calls", len(made))
print("done")
