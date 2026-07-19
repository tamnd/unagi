# expect: UNA-AIO-001
# A blocking sleep inside a coroutine stalls the whole event loop.
import asyncio
import time


async def handle():
    time.sleep(1)
