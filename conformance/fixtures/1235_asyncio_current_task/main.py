import asyncio


async def worker(name):
    task = asyncio.current_task()
    print("worker", name, "sees", task.get_name())
    await asyncio.sleep(0)
    return name


async def main():
    main_task = asyncio.current_task()
    print("main task", main_task.get_name())

    workers = [asyncio.create_task(worker(f"w{i}"), name=f"w{i}") for i in range(3)]
    pending = asyncio.all_tasks()
    print("all count while pending", len(pending))
    print("all names while pending", sorted(t.get_name() for t in pending))

    results = await asyncio.gather(*workers)
    print("results", results)

    remaining = asyncio.all_tasks()
    print("all names after gather", sorted(t.get_name() for t in remaining))
    print("current in main still", asyncio.current_task().get_name())


try:
    asyncio.current_task()
except RuntimeError as e:
    print("current no loop", e)
try:
    asyncio.all_tasks()
except RuntimeError as e:
    print("all no loop", e)
asyncio.run(main())
