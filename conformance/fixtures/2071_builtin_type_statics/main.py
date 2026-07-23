# The static and class methods a builtin type object carries, read off the type
# rather than an instance: str.maketrans, bytes.maketrans and bytes.fromhex, plus
# int.from_bytes / int.to_bytes. base64 reaches bytes.maketrans and int.from_bytes
# at import to build its translation tables and pack its bit groups, so with these
# and str.format keyword fields it imports and round-trips end to end. Every value
# here is pure ASCII or a fixed integer, identical on every host.

# str.maketrans builds a dict keyed by code point in its three forms.
print(str.maketrans("abc", "xyz"))
print(str.maketrans("ab", "xy", "z"))
print(str.maketrans({"a": "X", 98: "Y", 99: None}))
print("abcz".translate(str.maketrans("ab", "xy", "z")))

# bytes.maketrans returns a 256-byte table bytes.translate consumes; the identity
# table is byte i at position i, with from remapped to to.
t = bytes.maketrans(b"+/", b"-_")
print(type(t).__name__, len(t), t[43], t[47])
print(b"a+b/c".translate(t))
print(b"abcdef".translate(None, delete=b"bd"))
print(b"abcdef".translate(bytes.maketrans(b"a", b"A"), b"c"))

# bytes.fromhex / bytearray.fromhex skip ASCII whitespace at byte boundaries.
print(bytes.fromhex("48 65 6c6c6f"))
print(bytearray.fromhex("00ff").__class__.__name__, bytes(bytearray.fromhex("00ff")))

# int.from_bytes and int.to_bytes pack and unpack in either byte order, signed or
# not, and read byteorder positionally or by keyword.
print(int.from_bytes(b"\x00\x10", "big"), int.from_bytes(b"\x00\x10", "little"))
print(int.from_bytes(b"\xff", "big", signed=True), int.from_bytes(b"\x01\x02"))
print(int.from_bytes([1, 2], byteorder="big"))
print((1024).to_bytes(2, "big"), (1024).to_bytes(2, "little"))
print((-1).to_bytes(2, "big", signed=True), (5).to_bytes())
print((5).to_bytes(length=2, byteorder="big"))

# str.format resolves named fields from keyword arguments.
print("{greeting}, {who}!".format(greeting="hi", who="world"))
print("{0} {name} {1}".format("a", "b", name="mid"))

# base64 imports and round-trips its whole surface now.
import base64

data = b"Hello, World! 123"
print(base64.b64encode(data), base64.b64decode(base64.b64encode(data)) == data)
print(base64.urlsafe_b64encode(b"\xfb\xef"))
print(base64.b32encode(data), base64.b16encode(data))
print(base64.b85encode(data), base64.a85encode(data), base64.z85encode(data))
print(base64.b64decode(base64.b64encode(data)) == data)


def err(f):
    try:
        print("OK", f())
    except Exception as e:
        print(type(e).__name__, e)


err(lambda: bytes.maketrans(b"ab", b"abc"))
err(lambda: str.maketrans("ab", "abc"))
err(lambda: str.maketrans("ab"))
err(lambda: bytes.fromhex("abc"))
err(lambda: bytes.fromhex("4g"))
err(lambda: b"x".translate(b"short"))
err(lambda: int.from_bytes(5, "big"))
err(lambda: int.from_bytes(b"\x01", "weird"))
err(lambda: (256).to_bytes(1, "big"))
err(lambda: (-1).to_bytes(2, "big"))
err(lambda: (-129).to_bytes(1, "big", signed=True))
