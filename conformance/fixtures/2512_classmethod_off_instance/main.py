def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# CPython inherits a builtin classmethod or staticmethod onto an instance, so
# reading one off a value hands back the same callable the type-level read does.
# A classmethod reports its type through __self__, a bool binding bool and a
# bytearray binding bytearray; a staticmethod reports None. __qualname__ reads
# type.name while __name__ stays bare, and the callable still runs.
print("== a classmethod read off an instance binds the type ==")
print("(255).from_bytes.__self__:", (255).from_bytes.__self__)
print("(255).from_bytes.__qualname__:", (255).from_bytes.__qualname__)
print("(255).from_bytes.__name__:", (255).from_bytes.__name__)
print("True.from_bytes.__self__:", True.from_bytes.__self__)
print("True.from_bytes.__qualname__:", True.from_bytes.__qualname__)
print("(1.5).fromhex.__self__:", (1.5).fromhex.__self__)
print("(1.5).fromhex.__qualname__:", (1.5).fromhex.__qualname__)
print("(1.5).from_number.__self__:", (1.5).from_number.__self__)
print("(1.5).__getformat__.__self__:", (1.5).__getformat__.__self__)
print("(3+4j).from_number.__self__:", (3 + 4j).from_number.__self__)
print("(3+4j).from_number.__qualname__:", (3 + 4j).from_number.__qualname__)
print("b'x'.fromhex.__self__:", b"x".fromhex.__self__)
print("b'x'.fromhex.__qualname__:", b"x".fromhex.__qualname__)
print("bytearray(b'x').fromhex.__self__:", bytearray(b"x").fromhex.__self__)
print("bytearray(b'x').fromhex.__qualname__:", bytearray(b"x").fromhex.__qualname__)

print("== a staticmethod read off an instance reports None ==")
print("'abc'.maketrans.__self__:", "abc".maketrans.__self__)
print("'abc'.maketrans.__qualname__:", "abc".maketrans.__qualname__)
print("'abc'.maketrans.__name__:", "abc".maketrans.__name__)
print("b'x'.maketrans.__self__:", b"x".maketrans.__self__)
print("b'x'.maketrans.__qualname__:", b"x".maketrans.__qualname__)

print("== the inherited callable still runs off an instance ==")
print("(255).from_bytes(b'\\x02', 'big'):", (255).from_bytes(b"\x02", "big"))
print("(255).from_bytes(b'\\x02', byteorder='big'):", (255).from_bytes(b"\x02", byteorder="big"))
print("True.from_bytes(b'\\x00', 'big'):", True.from_bytes(b"\x00", "big"))
print("True.from_bytes(b'\\x01', 'big'):", True.from_bytes(b"\x01", "big"))
print("(1.5).fromhex('0x1.8p0'):", (1.5).fromhex("0x1.8p0"))
print("(3+4j).from_number(2):", (3 + 4j).from_number(2))
print("b'x'.fromhex('41 42'):", b"x".fromhex("41 42"))
print("bytearray(b'x').fromhex('41'):", bytearray(b"x").fromhex("41"))
print("'abc'.maketrans('a', 'b'):", "abc".maketrans("a", "b"))
print("b'x'.maketrans(b'a', b'b'):", b"x".maketrans(b"a", b"b"))

print("== a read off an instance still calls as a plain value ==")
g = (255).from_bytes
print("g = (255).from_bytes; g(b'\\x03', 'big'):", g(b"\x03", "big"))
