import array


def show(label, fn):
    try:
        print(label, "->", repr(fn()))
    except Exception as e:
        print(label, "->", type(e).__name__, e)


a = array.array("i", [1, 2, 3, 4])
m = memoryview(a)
show("format", lambda: m.format)
show("itemsize", lambda: m.itemsize)
show("nbytes", lambda: m.nbytes)
show("ndim", lambda: m.ndim)
show("shape", lambda: m.shape)
show("strides", lambda: m.strides)
show("readonly", lambda: m.readonly)
show("obj is a", lambda: m.obj is a)
show("len", lambda: len(m))
show("tolist", lambda: m.tolist())
show("index", lambda: m[2])
show("index neg", lambda: m[-1])
show("contains", lambda: 3 in m)
show("iter", lambda: list(m))
show("tobytes", lambda: m.tobytes())
show("hex", lambda: m.hex())
show("slice", lambda: m[1:3].tolist())
show("slice step", lambda: m[::2].tolist())

# writes alias the array
m[1] = 20
show("after set (array)", lambda: a.tolist())
m[1:3] = memoryview(array.array("i", [200, 300]))
show("after slice assign (array)", lambda: a.tolist())


def store(label, arr, key, value):
    mv = memoryview(arr)
    try:
        mv[key] = value
        print(label, "-> ok", mv.tolist())
    except Exception as e:
        print(label, "->", type(e).__name__, e)


store("set str", array.array("i", [1, 2]), 0, "x")
store("set float", array.array("i", [1, 2]), 0, 1.5)
store("set overflow", array.array("i", [1, 2]), 0, 2 ** 40)
store("b set 300", array.array("b", [1]), 0, 300)
store("B set -1", array.array("B", [1]), 0, -1)
store("H set big", array.array("H", [1]), 0, 70000)
store("d set int", array.array("d", [1.0]), 0, 7)
store("d set str", array.array("d", [1.0]), 0, "x")
store("slice bytes", array.array("i", [1, 2, 3, 4]), slice(1, 3), b"aaaaaaaa")

# typed decode across formats
show("d tolist", lambda: memoryview(array.array("d", [1.5, 2.5, 3.5])).tolist())
show("f tolist", lambda: memoryview(array.array("f", [1.5, 2.5])).tolist())
show("Q big", lambda: memoryview(array.array("Q", [2 ** 63 + 5])).tolist())
show("l signed", lambda: memoryview(array.array("l", [-7, 7])).tolist())
show("h signed", lambda: memoryview(array.array("h", [-100, 100])).tolist())
show("H unsigned", lambda: memoryview(array.array("H", [65535, 0])).tolist())
show("B bytes", lambda: memoryview(array.array("B", [1, 254])).tolist())
show("empty", lambda: memoryview(array.array("d", [])).tolist())
show("empty nbytes", lambda: memoryview(array.array("d", [])).nbytes)

# nested cast still works over a byte source
show("cast bytes", lambda: memoryview(b"abcd").cast("i").tolist())
