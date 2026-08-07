def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


class Idx:
    def __index__(self):
        return 2


class BadIdx:
    def __index__(self):
        return "x"


# A memoryview subscript runs its integer index through __index__ the way CPython
# feeds a subscript to PyNumber_Index, so an object spelling __index__ reads or
# writes the element at its value and a bool counts as 0 or 1.
def set_idx():
    m = memoryview(bytearray(b"abcde"))
    m[Idx()] = 88
    return bytes(m)


def set_bool():
    m = memoryview(bytearray(b"abcde"))
    m[True] = 90
    return bytes(m)


show("get-idx", lambda: memoryview(b"abcde")[Idx()])
show("get-bool", lambda: memoryview(b"abcde")[True])
show("get-neg", lambda: memoryview(b"abcde")[-1])
show("set-idx", set_idx)
show("set-bool", set_bool)

# A tuple subscript over a multi-dimensional view runs each component through
# __index__ the same way, reading or writing the addressed element.
def tuple_get():
    return memoryview(bytearray(b"abcd")).cast("B", shape=[2, 2])[Idx() - 1, Idx() - 1]


def tuple_set():
    m = memoryview(bytearray(b"abcd")).cast("B", shape=[2, 2])
    m[Idx() - 1, Idx() - 1] = 88
    return bytes(m.cast("B"))


show("tuple-get", tuple_get)
show("tuple-set", tuple_set)

# A bad __index__ return propagates its non-int TypeError, a float or str with no
# __index__ is the invalid-slice-key TypeError, and an out-of-range coerced index
# is the ordinary memoryview index error.
show("get-bad", lambda: memoryview(b"abcde")[BadIdx()])
show("get-float", lambda: memoryview(b"abcde")[1.0])
show("get-str", lambda: memoryview(b"abcde")["x"])
show("get-oob", lambda: memoryview(b"ab")[Idx()])


def set_bad():
    m = memoryview(bytearray(b"abcde"))
    m[BadIdx()] = 1
    return bytes(m)


def set_ro():
    m = memoryview(b"abcde")
    m[Idx()] = 1
    return bytes(m)


show("set-bad", set_bad)
show("set-readonly", set_ro)
