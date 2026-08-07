def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A classmethod read off a builtin type constructor carries __self__ (the type it
# is bound to) and a qualified __qualname__, while __name__ stays the bare method
# name. The maketrans staticmethods report __self__ = None instead of the type.
print("== a builtin classmethod is bound to its type ==")
for label, cm, owner in [
    ("int.from_bytes", int.from_bytes, int),
    ("bool.from_bytes", bool.from_bytes, bool),
    ("float.fromhex", float.fromhex, float),
    ("float.from_number", float.from_number, float),
    ("complex.from_number", complex.from_number, complex),
    ("bytes.fromhex", bytes.fromhex, bytes),
    ("bytearray.fromhex", bytearray.fromhex, bytearray),
    ("dict.fromkeys", dict.fromkeys, dict),
]:
    print(
        label,
        "| __self__:", cm.__self__,
        "| is owner:", cm.__self__ is owner,
        "| __name__:", cm.__name__,
        "| __qualname__:", cm.__qualname__,
    )

print("== a maketrans staticmethod reports __self__ = None ==")
for label, cm in [("str.maketrans", str.maketrans), ("bytes.maketrans", bytes.maketrans)]:
    print(label, "| __self__:", cm.__self__, "| __name__:", cm.__name__, "| __qualname__:", cm.__qualname__)

print("== a classmethod still calls ==")
print("int.from_bytes(b'\\x01\\x02', 'big'):", int.from_bytes(b"\x01\x02", "big"))
print("bool.from_bytes(b'\\x00', 'big'):", bool.from_bytes(b"\x00", "big"))
print("float.fromhex('0x1p4'):", float.fromhex("0x1p4"))
print("float.from_number(3):", float.from_number(3))
print("complex.from_number(2):", complex.from_number(2))
print("bytes.fromhex('dead'):", bytes.fromhex("dead"))
print("dict.fromkeys(['a', 'b']):", dict.fromkeys(["a", "b"]))
print("str.maketrans('a', 'b'):", str.maketrans("a", "b"))

print("== the type itself is unstamped ==")
print("int.__name__:", int.__name__, "| int.__qualname__:", int.__qualname__)
print("len.__name__:", len.__name__, "| len.__qualname__:", len.__qualname__)
show("int.__self__", lambda: int.__self__)
