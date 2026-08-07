import array


def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# A slice passed as a subscript object, the way m[sl] or m.__getitem__(slice(...))
# does, takes the same sub-view path the syntactic m[lo:hi:step] read compiles to,
# so a contiguous slice aliases the buffer and an extended step reads the picked
# elements. The dunder form and the slice-in-a-variable form agree with the
# syntactic form.
def read_dunder():
    return bytes(memoryview(b"abcde").__getitem__(slice(1, 4)))


def read_step():
    return bytes(memoryview(b"abcde").__getitem__(slice(0, 5, 2)))


def read_slice_var():
    sl = slice(1, 4)
    return bytes(memoryview(b"abcde")[sl])


def read_full():
    return bytes(memoryview(b"abcde")[:])


show("read-dunder", read_dunder)
show("read-step", read_step)
show("read-slice-var", read_slice_var)
show("read-full", read_full)


# A slice subscript object drives an assignment the same way, needing an
# exact-length bytes-like rvalue, contiguous or stepped, from bytes, bytearray or
# another memoryview.
def write_dunder():
    m = memoryview(bytearray(b"abc"))
    m.__setitem__(slice(0, 2), b"XY")
    return bytes(m)


def write_step():
    m = memoryview(bytearray(b"abcd"))
    m.__setitem__(slice(0, 4, 2), b"XY")
    return bytes(m)


def write_slice_var():
    m = memoryview(bytearray(b"abc"))
    sl = slice(1, 3)
    m[sl] = bytearray(b"YZ")
    return bytes(m)


def write_from_mv():
    m = memoryview(bytearray(b"abc"))
    m[0:2] = memoryview(b"XY")
    return bytes(m)


def write_neg_step():
    m = memoryview(bytearray(b"abcd"))
    m[::-1] = b"WXYZ"
    return bytes(m)


show("write-dunder", write_dunder)
show("write-step", write_step)
show("write-slice-var", write_slice_var)
show("write-from-mv", write_from_mv)
show("write-neg-step", write_neg_step)


# A view over a typed array slices and assigns whole elements, the rvalue needing
# the same struct format and element count, so a matching-format source lands and
# a plain-bytes source of the same byte length is the different-structures error.
def typed_read():
    return memoryview(array.array("i", [10, 20, 30, 40]))[1:3].tolist()


def typed_write():
    m = memoryview(array.array("i", [1, 2, 3, 4]))
    m[1:3] = memoryview(array.array("i", [20, 30]))
    return m.tolist()


def typed_write_format_mismatch():
    m = memoryview(array.array("i", [1, 2, 3]))
    m[0:2] = b"12345678"
    return m.tolist()


show("typed-read", typed_read)
show("typed-write", typed_write)
show("typed-format-mismatch", typed_write_format_mismatch)


# The assignment validates its rvalue: a length mismatch is the different-
# structures error, a non-bytes-like source is the bytes-like-required TypeError
# naming the source type, and a read-only view rejects every write.
def len_mismatch():
    m = memoryview(bytearray(b"abc"))
    m.__setitem__(slice(0, 2), b"X")
    return bytes(m)


def wrong_type_str():
    m = memoryview(bytearray(b"abc"))
    m[0:2] = "XY"
    return bytes(m)


def wrong_type_int():
    m = memoryview(bytearray(b"abc"))
    m[0:2] = 5
    return bytes(m)


def wrong_type_arr():
    m = memoryview(array.array("i", [1, 2, 3]))
    m[0:2] = "xx"
    return m.tolist()


def readonly_write():
    m = memoryview(b"abc")
    m[0:2] = b"XY"
    return bytes(m)


show("len-mismatch", len_mismatch)
show("wrong-type-str", wrong_type_str)
show("wrong-type-int", wrong_type_int)
show("wrong-type-arr", wrong_type_arr)
show("readonly-write", readonly_write)
