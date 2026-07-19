import asyncio


async def handle(reader, writer):
    # Consume the client's lines through the async-iterator protocol until the
    # client half-closes, upper-casing each line back.
    async for line in reader:
        writer.write(line.upper())
    await writer.drain()
    writer.write_eof()
    await writer.drain()
    writer.close()
    await writer.wait_closed()


async def main():
    server = await asyncio.start_server(handle, "127.0.0.1", 0)
    async with server:
        host, port = server.sockets[0].getsockname()

        reader, writer = await asyncio.open_connection(host, port)
        writer.writelines([b"one\n", b"two\n", b"three\n"])
        await writer.drain()
        writer.write_eof()

        lines = []
        async for line in reader:
            lines.append(line.decode().strip())
        print("lines", lines)

        writer.close()
        await writer.wait_closed()

    print("serving", server.is_serving())


asyncio.run(main())
print("ok")
