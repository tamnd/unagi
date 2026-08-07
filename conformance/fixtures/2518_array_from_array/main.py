from array import array


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# array(typecode, other_array) seeds value by value when the typecodes differ,
# the way CPython iterates the source elements rather than reinterpreting its raw
# bytes: a widening or narrowing between integer codes copies the values, an int
# source to a float code converts each, and a source value that does not fit the
# target raises the item error. A same-typecode source copies straight across.
print("== a different typecode iterates the source values ==")
show("array('h', array('i', [1, 2]))", lambda: array("h", array("i", [1, 2])))
show("array('l', array('b', [1, 2, 3]))", lambda: array("l", array("b", [1, 2, 3])))
show("array('i', array('l', [5, 6]))", lambda: array("i", array("l", [5, 6])))
show("array('q', array('h', [7]))", lambda: array("q", array("h", [7])))
show("array('d', array('i', [1, 2]))", lambda: array("d", array("i", [1, 2])))
show("array('f', array('d', [1.5, 2.5]))", lambda: array("f", array("d", [1.5, 2.5])))
show("array('d', array('f', [1.5]))", lambda: array("d", array("f", [1.5])))

print("== a same typecode copies the values across ==")
show("array('i', array('i', [1, 2]))", lambda: array("i", array("i", [1, 2])))
show("array('d', array('d', [1.5]))", lambda: array("d", array("d", [1.5])))
show("array('w', array('w', 'hi'))", lambda: array("w", array("w", "hi")))
show("empty source array('i', array('h', []))", lambda: array("i", array("h", [])))

print("== a source value that does not fit the target raises ==")
show("array('i', array('f', [1.0, 2.0]))", lambda: array("i", array("f", [1.0, 2.0])))
show("array('i', array('d', [1.5]))", lambda: array("i", array("d", [1.5])))
show("array('B', array('i', [300]))", lambda: array("B", array("i", [300])))
show("array('b', array('i', [200]))", lambda: array("b", array("i", [200])))

print("== a bytes-like initializer still reinterprets the raw bytes ==")
show("array('i', b'\\x01\\x00\\x00\\x00')", lambda: array("i", b"\x01\x00\x00\x00"))
show("array('h', bytearray(b'\\x01\\x00\\x02\\x00'))", lambda: array("h", bytearray(b"\x01\x00\x02\x00")))
