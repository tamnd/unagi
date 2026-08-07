def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


class Idx:
    def __index__(self):
        return 1


def released_count():
    m = memoryview(bytearray(b"ab"))
    m.release()
    return m.count(97)


def released_index():
    m = memoryview(bytearray(b"ab"))
    m.release()
    return m.index(97)


def released_index_badstart():
    m = memoryview(bytearray(b"abcd"))
    m.release()
    return m.index(97, "x")


md = memoryview(b"\x01\x02\x03\x04").cast('b', shape=[2, 2])
md3 = memoryview(bytes(range(8))).cast('b', shape=[2, 2, 2])
oneD = memoryview(b"\x01\x02\x03\x02")

# count and index over a plain one-dimensional view still work, reading the
# flat element run, and honor a typed or cast view element by element.
show("count-1d", lambda: oneD.count(2))
show("count-1d-zero", lambda: oneD.count(9))
show("index-1d", lambda: oneD.index(2))
show("index-1d-start", lambda: oneD.index(2, 2))
show("index-1d-idx", lambda: oneD.index(Idx()))
show("count-typed", lambda: memoryview(b"\x01\x00\x02\x00").cast('h').count(2))
show("index-typed", lambda: memoryview(b"\x01\x00\x02\x00").cast('h').index(2))
show("index-1d-notfound", lambda: oneD.index(9))

# count and index over a multi-dimensional view decline: count would be
# counting sub-views, index a multi-dimensional lookup, each its own wording.
show("count-md", lambda: md.count(1))
show("count-md3", lambda: md3.count(0))
show("index-md", lambda: md.index(1))
show("index-md3", lambda: md3.index(0))

# The argument-count and released checks outrank the sub-view decline for
# count, so a bad arity or a released view reports first.
show("count-noargs", lambda: md.count())
show("count-2args", lambda: md.count(1, 2))
show("count-released", released_count)
show("count-md-badtype", lambda: md.count([]))

# index converts its start and stop bounds before touching the buffer, so a
# non-integer bound is the slice-index TypeError even on a released or
# multi-dimensional view, and None is rejected the way a bad type is.
show("index-noargs", lambda: oneD.index())
show("index-4args", lambda: oneD.index(1, 2, 3, 4))
show("index-released", released_index)
show("index-released-badstart", released_index_badstart)
show("index-md-badstart", lambda: md.index(1, "x"))
show("index-start-none", lambda: oneD.index(2, None))
show("index-start-float", lambda: oneD.index(2, 1.5))
show("index-start-idx", lambda: oneD.index(2, Idx()))
show("index-start-bool", lambda: oneD.index(2, True))
show("index-start-neg", lambda: oneD.index(2, -3))
show("index-start-big", lambda: oneD.index(2, 10 ** 30))
show("index-start-negbig", lambda: oneD.index(2, -(10 ** 30)))
