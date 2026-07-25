import socket

# Numeric getaddrinfo is deterministic across platforms: the host is a literal
# address, so no DNS runs and the family/type/proto are fixed.
for r in socket.getaddrinfo("127.0.0.1", 80, socket.AF_INET, socket.SOCK_STREAM):
    print("gai stream:", r)

# AI_PASSIVE with a None host and a datagram type yields the wildcard address
# and the udp protocol, exercising the IntEnum-member coercion of the type arg.
for r in socket.getaddrinfo(None, 0, socket.AF_INET, socket.SOCK_DGRAM, 0, socket.AI_PASSIVE):
    print("gai passive:", r)

# Numeric name lookups pass straight through.
print("gethostbyname:", socket.gethostbyname("127.0.0.1"))

# Service and protocol tables share a common subset on every platform.
print("getservbyname http:", socket.getservbyname("http"))
print("getservbyname http tcp:", socket.getservbyname("http", "tcp"))
print("getprotobyname tcp:", socket.getprotobyname("tcp"))
print("getprotobyname udp:", socket.getprotobyname("udp"))

# Numeric getnameinfo reverses the numeric getaddrinfo.
ni = socket.getnameinfo(("127.0.0.1", 80), socket.NI_NUMERICHOST | socket.NI_NUMERICSERV)
print("getnameinfo:", ni)

# The flag constants are ints (their values differ by platform, so only the
# types and truthiness are asserted, not the numbers).
print("AI_PASSIVE is int:", isinstance(socket.AI_PASSIVE, int))
print("NI_NUMERICHOST is int:", isinstance(socket.NI_NUMERICHOST, int))

print("done")
