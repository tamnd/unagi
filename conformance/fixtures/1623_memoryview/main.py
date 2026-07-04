# memoryview over bytes and bytearray: a 1-D unsigned-byte view that reads
# through the buffer, slices into shared sub-views and writes back into a
# writable base.

def show(fn):
    try:
        print(repr(fn()))
    except Exception as e:
        print(type(e).__name__ + ": " + str(e))


ro = memoryview(b"hello")
print(len(ro), ro[0], ro[-1])
print(ro.format, ro.itemsize, ro.ndim, ro.shape, ro.strides, ro.nbytes, ro.readonly)
print(ro.contiguous, ro.c_contiguous, ro.f_contiguous)
print(ro.tobytes(), ro.tolist(), ro.hex())
print(type(ro.obj).__name__)

# slicing: contiguous shares the buffer, extended reads the picked bytes.
print(bytes(ro[1:4]), bytes(ro[::2]), bytes(ro[::-1]))
sub = ro[1:4]
print(len(sub), sub.tolist(), sub[0])

# equality against every bytes-like shape, and inequality against the rest.
print(ro == b"hello", b"hello" == ro, ro == bytearray(b"hello"))
print(ro == memoryview(b"hello"), ro == b"world", ro == "hello", ro == 5)

# hashing: a read-only view hashes like its bytes.
print(hash(ro) == hash(b"hello"))

# iteration and membership yield and test the element ints.
print(list(ro), tuple(ro))
print(104 in ro, 999 in ro, b"he" in ro)

# a nested view re-views the same root buffer.
nested = memoryview(ro)
print(nested.tolist(), type(nested.obj).__name__)

# type identity and structural match.
print(type(memoryview).__name__, type(ro).__name__, isinstance(ro, memoryview))
match ro:
    case memoryview():
        print("matched memoryview")

# a writable view mutates the underlying bytearray in place.
buf = bytearray(b"hello")
wm = memoryview(buf)
print(wm.readonly)
wm[0] = 88
wm[1:3] = b"YZ"
print(wm.tobytes(), bytes(buf))
wm[::2] = b"..."
print(wm.tobytes(), bytes(buf))

# error catalog.
show(lambda: memoryview())
show(lambda: memoryview(b"a", b"b"))
show(lambda: memoryview(5))
show(lambda: memoryview("ab"))
show(lambda: ro[10])
show(lambda: ro[-10])
show(lambda: ro["x"])
show(lambda: hash(wm))


def setro():
    ro[0] = 65


def badval():
    wm[0] = 300


def badtype():
    wm[0] = "x"


def badstruct():
    wm[1:3] = b"Z"


def delone():
    del wm[0]


def delro():
    del ro[0]


show(setro)
show(badval)
show(badtype)
show(badstruct)
show(delone)
show(delro)
