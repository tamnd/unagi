import asyncio


async def handle(reader, writer):
    # Echo one line back, then finish the write side and close.
    data = await reader.readline()
    writer.write(data)
    await writer.drain()
    writer.write_eof()
    await writer.drain()
    writer.close()
    await writer.wait_closed()


async def main():
    # Bind to an ephemeral port; the port varies per run, so it is never printed.
    server = await asyncio.start_server(handle, "127.0.0.1", 0)
    async with server:
        print("serving", server.is_serving())
        host, port = server.sockets[0].getsockname()

        reader, writer = await asyncio.open_connection(host, port)
        writer.write(b"ping\n")
        await writer.drain()

        line = await reader.readline()
        print("line", line.decode().strip())

        rest = await reader.read()
        print("rest", repr(rest))

        writer.close()
        await writer.wait_closed()

    print("serving", server.is_serving())


asyncio.run(main())
print("ok")
