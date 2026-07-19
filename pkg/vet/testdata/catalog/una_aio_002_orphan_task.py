# expect: UNA-AIO-002
# The task is discarded; the loop may collect it mid-run and lose its errors.
import asyncio


async def main():
    asyncio.create_task(worker())
    await serve()
