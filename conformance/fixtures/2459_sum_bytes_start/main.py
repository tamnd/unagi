# sum() refuses a bytes or bytearray start value the way it already refuses a
# str one, pointing at b''.join, and it is raised even for an empty iterable and
# for a subclass, while a memoryview start (or a list, tuple or dict) is fine.
def show(label, fn):
    try:
        r = fn()
        print(label, type(r).__name__, repr(r))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


class B(bytes):
    pass


class BA(bytearray):
    pass


show("bytes-start", lambda: sum([b"a", b"b"], b""))
show("bytearray-start", lambda: sum([bytearray(b"a")], bytearray()))
show("str-start", lambda: sum(["a", "b"], ""))
show("empty-bytes-start", lambda: sum([], b""))
show("empty-bytearray-start", lambda: sum([], bytearray()))
show("bytes-subclass-start", lambda: sum([], B()))
show("bytearray-subclass-start", lambda: sum([], BA()))
show("memoryview-start", lambda: type(sum([], memoryview(b""))).__name__)
show("list-start", lambda: sum([[1], [2]], []))
show("tuple-start", lambda: sum([(1,), (2,)], ()))
show("dict-start", lambda: sum([{1: 2}], {}))
show("int-normal", lambda: sum([1, 2, 3]))
show("float-normal", lambda: sum([1.5, 2.5]))
show("bytes-no-start", lambda: sum([b"a", b"b"]))
show("empty-int", lambda: sum([]))
