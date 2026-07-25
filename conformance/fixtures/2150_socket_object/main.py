import socket

# A TCP loopback round trip over the real socket object: bind an ephemeral
# port, connect, accept, and pass bytes both ways. The port is not printed
# because it is assigned by the kernel and varies per run.
srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("127.0.0.1", 0))
srv.listen(1)
host, port = srv.getsockname()
print("bound host:", host)

cli = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
cli.connect((host, port))
conn, addr = srv.accept()
print("peer host:", addr[0])

cli.sendall(b"hello unagi")
data = conn.recv(64)
print("server got:", data.decode())

conn.sendall(b"ack")
back = cli.recv(64)
print("client got:", back.decode())

# The family, type and proto attributes come off the C socket as IntEnum
# members and compare equal to the module constants.
print("family:", cli.family == socket.AF_INET)
print("type:", cli.type == socket.SOCK_STREAM)
print("proto:", cli.proto)

# fileno is a live descriptor while open and -1 after close and detach.
print("fileno open:", cli.fileno() >= 0)

# gettimeout defaults to None (blocking); settimeout round-trips.
print("timeout default:", cli.gettimeout())
cli.settimeout(5.0)
print("timeout set:", cli.gettimeout())
cli.setblocking(True)
print("blocking:", cli.getblocking())

conn.close()
cli.close()
srv.close()
print("closed fileno:", cli.fileno())

# A closed socket raises OSError on an operation that needs the descriptor.
try:
    cli.recv(1)
except OSError:
    print("recv after close: OSError")

# SocketType is an alias of the socket class.
print("SocketType is socket:", socket.SocketType is socket.socket)
print("done")
