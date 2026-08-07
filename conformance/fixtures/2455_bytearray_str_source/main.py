# bytearray rejects a str source for slice assignment and extend with CPython's
# exact wording, while a str element inside another iterable still hits the
# per-item integer check.
def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, type(ex).__name__, str(ex))


class S(str):
    pass


def slice_assign(src, sl=slice(0, 2)):
    b = bytearray(b"abcde")
    b[sl] = src
    return bytes(b)


def extend(src):
    b = bytearray(b"ab")
    b.extend(src)
    return bytes(b)


show("sa-str", lambda: slice_assign("xy"))
show("sa-str-empty", lambda: slice_assign(""))
show("sa-str-subclass", lambda: slice_assign(S("xy")))
show("sa-str-extended", lambda: slice_assign("xy", slice(0, 4, 2)))
show("sa-list-strelem", lambda: slice_assign(["x"], slice(0, 1)))
show("sa-list-ints", lambda: slice_assign([65, 66]))
show("sa-bytes", lambda: slice_assign(b"XY"))
show("sa-range", lambda: slice_assign(range(66, 68)))
show("sa-int", lambda: slice_assign(5))

show("ext-str", lambda: extend("x"))
show("ext-str-empty", lambda: extend(""))
show("ext-str-subclass", lambda: extend(S("hi")))
show("ext-list-strelem", lambda: extend(["x"]))
show("ext-gen-str", lambda: extend(c for c in "x"))
show("ext-bytes", lambda: extend(b"CD"))
show("ext-range", lambda: extend(range(67, 69)))
show("ext-int", lambda: extend(5))
show("ext-dict", lambda: extend({67: 1, 68: 2}))
