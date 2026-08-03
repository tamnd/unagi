import array
import io


def show(fn):
    try:
        fn()
    except Exception as e:
        print(type(e).__name__, e)


# tofile writes the array's raw item bytes.
buf = io.BytesIO()
array.array("h", [1, 2, 3]).tofile(buf)
print(buf.getvalue().hex())

buf = io.BytesIO()
array.array("i", [-1, 258]).tofile(buf)
print(buf.getvalue().hex())

# An empty array writes nothing.
buf = io.BytesIO()
array.array("d").tofile(buf)
print(repr(buf.getvalue()))

# fromfile reads the requested item count off the stream.
src = io.BytesIO(bytes([7, 0, 0, 0, 8, 0, 0, 0]))
a = array.array("i")
a.fromfile(src, 2)
print(a)

# Reading zero items leaves the array and the stream untouched.
src = io.BytesIO(bytes([9, 0, 0, 0]))
a = array.array("i", [5])
a.fromfile(src, 0)
print(a, src.tell())

# A roundtrip through a single stream.
buf = io.BytesIO()
array.array("f", [1.5, -2.25]).tofile(buf)
buf.seek(0)
a = array.array("f")
a.fromfile(buf, 2)
print(a)

# A short read appends the whole items it did get, then raises EOFError.
src = io.BytesIO(bytes([7, 0, 0, 0]))
a = array.array("i", [99])
show(lambda: a.fromfile(src, 2))
print(a)

# A read that is not a whole multiple of the item size raises from frombytes
# first, before the short-read check, and leaves the array unchanged.
src = io.BytesIO(bytes([1, 2, 3]))
a = array.array("i", [42])
show(lambda: a.fromfile(src, 1))
print(a)

# A negative count is rejected before any read happens.
src = io.BytesIO(bytes([0, 0, 0, 0]))
show(lambda: array.array("i").fromfile(src, -1))
print(src.tell())

# A non-integer count is a TypeError.
show(lambda: array.array("i").fromfile(io.BytesIO(), "x"))

# tofile and fromfile against an object with no write/read raise AttributeError.
show(lambda: array.array("i", [1]).tofile(object()))
show(lambda: array.array("i").fromfile(object(), 1))

# Arity errors.
show(lambda: array.array("i").fromfile(io.BytesIO()))
show(lambda: array.array("i").fromfile(io.BytesIO(), 1, 2))
show(lambda: array.array("i", [1]).tofile())
show(lambda: array.array("i", [1]).tofile(io.BytesIO(), io.BytesIO()))
