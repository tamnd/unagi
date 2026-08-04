# memoryview.count and index, the sequence search CPython 3.14 added to
# memoryview. count tallies the elements equal to the value and index returns the
# first position within an optional start/stop window, both reading the
# format-decoded elements so a value finds a byte across an int-vs-float compare.


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


mv = memoryview(bytearray(b"abcabc"))

print("has:", hasattr(mv, "count"), hasattr(mv, "index"))

# count tallies elements equal to the value, with Python equality.
show("count(97)", lambda: mv.count(97))
show("count(122)", lambda: mv.count(122))
show("count(97.0)", lambda: mv.count(97.0))
show("count(b'a')", lambda: mv.count(b"a"))
show("count(None)", lambda: mv.count(None))

# index returns the first position, with an optional start/stop window clamped
# the way a slice is, and raises the not-found ValueError past the end.
show("index(98)", lambda: mv.index(98))
show("index(122)", lambda: mv.index(122))
show("index(97,1)", lambda: mv.index(97, 1))
show("index(97,1,3)", lambda: mv.index(97, 1, 3))
show("index(97,-3)", lambda: mv.index(97, -3))
show("index(97,0,100)", lambda: mv.index(97, 0, 100))

# The arity errors carry each method's own wording.
show("count()", lambda: mv.count())
show("count(1,2)", lambda: mv.count(1, 2))
show("index()", lambda: mv.index())
show("index(1,2,3,4)", lambda: mv.index(1, 2, 3, 4))

# A cast view searches the decoded elements of the new format.
mvi = memoryview(bytearray(b"\x01\x00\x00\x00\x02\x00\x00\x00")).cast("i")
show("cast-i count(1)", lambda: mvi.count(1))
show("cast-i index(2)", lambda: mvi.index(2))

# Both raise on a released view through the element read.
r = memoryview(bytearray(b"x"))
r.release()
show("released count(1)", lambda: r.count(1))
show("released index(1)", lambda: r.index(1))
