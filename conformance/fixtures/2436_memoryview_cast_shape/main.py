def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# memoryview.cast(format, shape) reshapes a one-dimensional byte view into a
# multi-dimensional one over the same buffer, reporting the C-contiguous ndim,
# shape and strides CPython derives.
c = memoryview(bytes(range(24))).cast("i", shape=[2, 3])
print("meta", c.ndim, c.shape, c.strides, c.itemsize, c.nbytes, c.format, c.readonly)
print("contig", c.c_contiguous, c.f_contiguous, c.contiguous)
print("len", len(c))
print("tolist", c.tolist())
print("tobytes", c.tobytes())
print("hex", c.hex())

# A tuple key whose length matches the dimensions reads one element, each
# component normalised against its own dimension, and an out-of-range component
# names the one-based dimension it left.
show("elem[1,2]", lambda: c[1, 2])
show("elem[-1,-1]", lambda: c[-1, -1])
show("oob-dim1", lambda: c[2, 0])
show("oob-dim2", lambda: c[0, 3])

# A longer tuple is the arity error, a shorter one and a bare integer would name
# a sub-view, which is not implemented, and iterating or membership on a
# multi-dimensional view raise the same way.
show("too-many", lambda: c[0, 0, 0])
show("short-tuple", lambda: c[0,])
show("empty-tuple", lambda: c[()])
show("int-index", lambda: c[0])
show("iter", lambda: list(c))
show("contains", lambda: 5 in c)

# A tuple assignment writes through to the shared buffer, an out-of-range write
# raises, and an integer or short-tuple write names the unimplemented sub-view.
w = memoryview(bytearray(range(24))).cast("B", shape=[2, 3, 4])
w[1, 2, 3] = 200
print("write-3d", w.tolist())
show("write-oob", lambda: w.__setitem__((2, 0, 0), 1))
show("write-int", lambda: w.__setitem__(0, 1))
show("write-short", lambda: w.__setitem__((0, 0), 1))

# Slicing picks rows out of the leading dimension: a step of one stays a
# contiguous sub-view and an extended step becomes a strided one whose reads walk
# the recorded strides, and a strided view refuses to be recast.
b48 = bytes(range(48))
cc = memoryview(b48).cast("i", shape=[4, 3])
show("slice-0:1", lambda: cc[0:1].tolist())
show("slice-0:2", lambda: (cc[0:2].shape, cc[0:2].tolist()))
show("strided-rows", lambda: cc[0:4:2].tolist())
show("strided-meta", lambda: (cc[0:4:2].shape, cc[0:4:2].strides, cc[0:4:2].c_contiguous))
show("strided-tobytes", lambda: cc[0:4:2].tobytes())
show("recast-strided", lambda: cc[0:4:2].cast("B"))
show("recast-rowslice", lambda: cc[0:2].cast("B").shape)


# f_contiguous is true only when at most one dimension is wider than one element.
def fc(shape):
    n = 1
    for s in shape:
        n *= s
    v = memoryview(bytes(range(n * 4))).cast("i", shape=shape)
    return (v.c_contiguous, v.f_contiguous)


print("fc", fc([1, 12]), fc([12, 1]), fc([3, 4]), fc([12]))

# A three-dimensional view nests one list per dimension and lays out C-order
# strides.
d = memoryview(bytes(range(24))).cast("B", shape=[2, 3, 4])
print("3d", d.tolist())
print("3d-strides", d.strides)

# Equality is shape sensitive: a two-dimensional view and a flat view of the same
# bytes are unequal, while two views of the same shape and bytes are equal.
b8 = bytes(range(8))
print("2d==1d", memoryview(b8).cast("i", shape=[1, 2]) == memoryview(b8).cast("i"))
print(
    "2d==2d",
    memoryview(b8).cast("i", shape=[1, 2]) == memoryview(bytes(range(8))).cast("i", shape=[1, 2]),
)

# The shape argument is validated with CPython's own messages, and cast keeps the
# clinic arity, duplicate and unknown-keyword errors with format and shape both
# position-or-keyword.
mm = memoryview(bytes(range(24)))
show("bad-shape-type", lambda: mm.cast("i", shape=5))
show("neg-elem", lambda: mm.cast("i", shape=[2, -3]))
show("zero-elem", lambda: mm.cast("i", shape=[2, 0]))
show("noninteger-elem", lambda: mm.cast("i", shape=[2, "x"]))
show("wrong-product", lambda: mm.cast("i", shape=[2, 2]))
show("shape-tuple", lambda: mm.cast("i", shape=(2, 3)).shape)
show("shape-empty", lambda: mm.cast("i", shape=[]))
show("positional-shape", lambda: mm.cast("i", [2, 3]).shape)
show("format-kw", lambda: mm.cast(format="i", shape=[2, 3]).shape)
show("no-args", lambda: mm.cast())
show("three-pos", lambda: mm.cast("i", [2, 3], "x"))
show("dup-format", lambda: mm.cast("i", format="i"))
show("unknown-kw", lambda: mm.cast("B", foo=1))
show("shape-only-kw", lambda: mm.cast(shape=[2, 3]))
show("recast-2d-to-2d", lambda: c.cast("B", shape=[4, 6]))

# A one-dimensional view still answers a single-element tuple key and an explicit
# one-element shape stays one-dimensional.
show("1d-tuple", lambda: memoryview(bytes(range(4))).cast("i")[(0,)])
show("1d-shape", lambda: (mm.cast("i", shape=[6]).ndim, mm.cast("i", shape=[6]).shape))
