class MyInt(int):
    pass


class MyFloat(float):
    pass


class MyStr(str):
    pass


class MyBytes(bytes):
    pass


def show(label, e):
    try:
        v = e()
        print(label, repr(v), type(v).__name__)
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# A value subclass inherits its base's constructing classmethods and rebuilds the
# subclass, so from_bytes on an int subclass yields the subclass, not a bare int,
# and fromhex on a float or bytes subclass does the same.
show("int-from_bytes", lambda: MyInt.from_bytes(b"\x01\x02", "big"))
show("int-from_bytes-signed", lambda: MyInt.from_bytes(b"\xff", "big", signed=True))
show("int-from_bytes-little", lambda: MyInt.from_bytes(b"\x01\x02", byteorder="little"))
show("float-fromhex", lambda: MyFloat.fromhex("0x1.8p3"))
show("float-fromhex-neg", lambda: MyFloat.fromhex("-0x1p4"))
show("bytes-fromhex", lambda: MyBytes.fromhex("6162"))
show("bytes-fromhex-space", lambda: MyBytes.fromhex("61 62"))

# The result really is the subclass and keeps its inherited behavior, so the int
# value and its methods read through.
v = MyInt.from_bytes(b"\x01\x00", "big")
show("subclass-value", lambda: int(v))
show("subclass-bit_length", lambda: v.bit_length())

# A bad argument still raises the base classmethod's own error.
show("float-fromhex-err", lambda: MyFloat.fromhex("nothex"))
show("int-from_bytes-badorder", lambda: MyInt.from_bytes(b"\x01", "sideways"))

# A non-constructing classmethod inherits unchanged, returning its native type
# rather than the subclass: maketrans yields a translation table and __getformat__
# a str.
show("str-maketrans", lambda: type(MyStr.maketrans("ab", "cd")).__name__)
show("bytes-maketrans", lambda: type(MyBytes.maketrans(b"ab", b"cd")).__name__)
show("float-getformat", lambda: MyFloat.__getformat__("double"))

# A class-dict override still wins over the inherited classmethod.
class Over(int):
    @classmethod
    def from_bytes(cls, *a, **k):
        return "overridden"


show("override", lambda: Over.from_bytes(b"\x01", "big"))

# The plain base is unchanged and yields the base type.
show("plain-int", lambda: int.from_bytes(b"\x01\x02", "big"))
show("plain-float", lambda: float.fromhex("0x1.8p3"))
