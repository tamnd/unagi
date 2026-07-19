import asyncio


async def main():
    loop = asyncio.get_running_loop()

    # a fresh loop reports no custom handler
    print("default is", loop.get_exception_handler())

    seen = []

    def handler(caught_loop, context):
        seen.append(context["message"])
        exc = context.get("exception")
        print("handler:", context["message"], "loop", caught_loop is loop, "exc", repr(exc))

    # set one, read it back, and route two contexts through it
    loop.set_exception_handler(handler)
    print("installed", loop.get_exception_handler() is handler)
    loop.call_exception_handler({"message": "boom", "exception": ValueError("bad")})
    loop.call_exception_handler({"message": "quiet"})
    print("seen", seen)

    # None restores the default
    loop.set_exception_handler(None)
    print("restored", loop.get_exception_handler())

    # the default handler logs the message and every other key, sorted, to stderr
    loop.call_exception_handler({"message": "to stderr", "code": 7, "attempt": 2})
    loop.call_exception_handler({"where": "no message key"})

    # a non-callable that is not None is a TypeError
    try:
        loop.set_exception_handler(42)
    except TypeError as e:
        print("typeerror", e)


asyncio.run(main())
print("done")
