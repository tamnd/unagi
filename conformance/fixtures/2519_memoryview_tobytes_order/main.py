def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# memoryview.tobytes(order='C') copies the view's logical bytes out in the given
# traversal order: 'C', 'A' and None read row-major, 'F' column-major. The two
# orders only differ once the view carries more than one dimension, and the order
# argument is validated the way CPython's clinic declares it, str or None with a
# value of 'C', 'F' or 'A'.
print("== a one-dimensional view reads the same in every order ==")
mv = memoryview(b"abcd")
show("tobytes()", lambda: mv.tobytes())
show("tobytes('C')", lambda: mv.tobytes("C"))
show("tobytes('F')", lambda: mv.tobytes("F"))
show("tobytes('A')", lambda: mv.tobytes("A"))
show("tobytes(None)", lambda: mv.tobytes(None))
show("tobytes(order='C')", lambda: mv.tobytes(order="C"))
show("tobytes(order='F')", lambda: mv.tobytes(order="F"))
show("tobytes(order=None)", lambda: mv.tobytes(order=None))

print("== a multi-dimensional view reads columns under F ==")
m2 = memoryview(bytes(range(6))).cast("B", shape=[2, 3])
show("2x3 tobytes('C')", lambda: m2.tobytes("C"))
show("2x3 tobytes('F')", lambda: m2.tobytes("F"))
show("2x3 tobytes('A')", lambda: m2.tobytes("A"))
show("2x3 tobytes()", lambda: m2.tobytes())
m3 = memoryview(bytes(range(24))).cast("i", shape=[2, 3])
show("2x3 i tobytes('C')", lambda: m3.tobytes("C"))
show("2x3 i tobytes('F')", lambda: m3.tobytes("F"))

print("== the order argument is validated ==")
show("tobytes('Q')", lambda: mv.tobytes("Q"))
show("tobytes('')", lambda: mv.tobytes(""))
show("tobytes('c')", lambda: mv.tobytes("c"))
show("tobytes('CF')", lambda: mv.tobytes("CF"))
show("tobytes(5)", lambda: mv.tobytes(5))
show("tobytes('C', 'F')", lambda: mv.tobytes("C", "F"))
show("tobytes(foo='C')", lambda: mv.tobytes(foo="C"))
show("tobytes('C', order='C')", lambda: mv.tobytes("C", order="C"))

print("== a released view checks arguments before the released error ==")
r = memoryview(bytearray(b"wxyz"))
r.release()
show("released tobytes('Q')", lambda: r.tobytes("Q"))
show("released tobytes('C')", lambda: r.tobytes("C"))
show("released tobytes(5)", lambda: r.tobytes(5))
show("released tobytes('C', 'F')", lambda: r.tobytes("C", "F"))
