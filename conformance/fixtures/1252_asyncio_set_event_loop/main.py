import asyncio


# no loop has been set and none is running yet
try:
    asyncio.get_event_loop()
except RuntimeError as e:
    print("unset", e)

loop = asyncio.new_event_loop()
asyncio.set_event_loop(loop)
print("same", asyncio.get_event_loop() is loop)
print("running", loop.is_running())

# clearing the slot brings the error back
asyncio.set_event_loop(None)
try:
    asyncio.get_event_loop()
except RuntimeError as e:
    print("cleared", e)

# a non-loop argument is a TypeError naming the offending type
try:
    asyncio.set_event_loop(42)
except TypeError as e:
    print("badtype", e)

loop.close()


# a running loop wins over any set loop
async def main():
    print("inside", asyncio.get_event_loop() is asyncio.get_running_loop())


asyncio.run(main())

# the run left no current loop behind
try:
    asyncio.get_event_loop()
except RuntimeError as e:
    print("after run", e)
