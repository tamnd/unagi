import array


def show(label, fn):
    try:
        print(label, "->", repr(fn()))
    except Exception as e:
        print(label, "->", type(e).__name__, e)


ba = bytearray(b"abcd")
m = memoryview(ba)
r = m.toreadonly()
show("readonly flag", lambda: r.readonly)
show("orig still writable", lambda: m.readonly)
show("tolist", lambda: r.tolist())
show("format", lambda: r.format)
show("itemsize", lambda: r.itemsize)
show("nbytes", lambda: r.nbytes)
show("shape", lambda: r.shape)
show("obj is ba", lambda: r.obj is ba)


def write_blocked():
    r[1] = 65
    return "ok"


def slice_blocked():
    r[0:2] = b"XY"
    return "ok"


show("write blocked", write_blocked)
show("slice write blocked", slice_blocked)

# a write through the original still shows in the read-only twin (aliasing)
m[0] = 90
show("aliases orig write", lambda: r.tolist())

# a read-only twin over a bytearray is still unhashable, its exporter is
show("hash twin over bytearray", lambda: hash(r))
show("orig hash still fails", lambda: hash(m))

# a read-only twin over bytes hashes like the bytes
show("hash twin over bytes", lambda: hash(memoryview(b"abcd").toreadonly()) == hash(b"abcd"))

# toreadonly over a non-byte array keeps the format and restricts hashing
show("ro of array format", lambda: memoryview(array.array("i", [1, 2])).toreadonly().format)
show("ro of array tolist", lambda: memoryview(array.array("i", [1, 2])).toreadonly().tolist())
show("hash ro array i", lambda: hash(memoryview(array.array("i", [1, 2])).toreadonly()))

# toreadonly on an already-read-only view
show("ro of ro", lambda: memoryview(b"xy").toreadonly().readonly)

# toreadonly on a released view
show("ro after release", lambda: (lambda v: (v.release(), v.toreadonly())[1])(memoryview(b"ab")))

# arg count
show("toreadonly arg", lambda: memoryview(b"ab").toreadonly(1))
