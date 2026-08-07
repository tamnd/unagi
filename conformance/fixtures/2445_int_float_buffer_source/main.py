import array


def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


# int() and float() parse a bytes-like or buffer source (bytes, bytearray,
# memoryview or array) as latin-1 text the way CPython reads the buffer protocol,
# so a numeric spelled in any of them converts the same as the str form.
show("int-bytes", lambda: int(b"42"))
show("int-bytearray", lambda: int(bytearray(b"99")))
show("int-mv", lambda: int(memoryview(b"42")))
show("int-array", lambda: int(array.array("b", [52, 50])))
show("int-mv-ws", lambda: int(memoryview(b"  10  ")))
show("int-bytes-sign", lambda: int(b"-7"))
show("float-bytes", lambda: float(b"3.14"))
show("float-bytearray", lambda: float(bytearray(b"2.5")))
show("float-mv", lambda: float(memoryview(b"3.5")))
show("float-array", lambda: float(array.array("b", [51, 46, 53])))
show("float-mv-inf", lambda: float(memoryview(b"inf")))
show("float-bytes-sci", lambda: float(b"1e3"))
show("float-bytes-underscore", lambda: float(b"1_000.0"))
show("float-bytes-sign", lambda: float(b"-2.5"))

# A parse failure on int() reports the buffer normalized to a bytes literal
# regardless of the source spelling, so bytes, bytearray, memoryview and array all
# name b'xy'. float() names the source object, so a bytes source is b'xy' and a
# bytearray source is bytearray(b'xy').
show("int-bytes-bad", lambda: int(b"xy"))
show("int-bytearray-bad", lambda: int(bytearray(b"xy")))
show("int-mv-bad", lambda: int(memoryview(b"xy")))
show("int-array-bad", lambda: int(array.array("b", [120, 121])))
show("int-mv-typed-bad", lambda: int(memoryview(array.array("i", [1]))))
show("int-bytes-empty", lambda: int(b""))
show("float-bytes-bad", lambda: float(b"abc"))
show("float-bytearray-bad", lambda: float(bytearray(b"xy")))
show("float-bytes-empty", lambda: float(b""))
show("float-bytes-nul", lambda: float(b"1\x002"))

# int(x, base) takes only a genuine bytes or bytearray with an explicit base, so a
# memoryview or array is the non-string TypeError, while a bytes or bytearray
# parses and a failure names the bytes-normalized literal.
show("int-bytes-base", lambda: int(b"101", 2))
show("int-bytearray-base", lambda: int(bytearray(b"ff"), 16))
show("int-mv-base", lambda: int(memoryview(b"101"), 2))
show("int-array-base", lambda: int(array.array("b", [49, 48, 49]), 2))
show("int-bytearray-base-bad", lambda: int(bytearray(b"xy"), 16))
show("int-bytes-base-bad", lambda: int(b"zz", 16))

# A released view can no longer expose its buffer, so int() and float() of it are
# the argument-type TypeError naming memoryview rather than a parse of empty bytes.
def int_released():
    m = memoryview(bytearray(b"42"))
    m.release()
    return int(m)


def float_released():
    m = memoryview(bytearray(b"42"))
    m.release()
    return float(m)


show("int-released", int_released)
show("float-released", float_released)
