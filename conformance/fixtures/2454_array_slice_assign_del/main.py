import array


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


def A(*v):
    return array.array('i', list(v))


# A contiguous slice assignment splices an array of the same typecode in, and
# may grow or shrink the array; an empty right side deletes the span.
show("contig-replace", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(0, 2), A(9, 8)), b.tolist())[1])())
show("contig-grow", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(1, 2), A(7, 8, 9)), b.tolist())[1])())
show("contig-shrink", lambda: (lambda b=A(1, 2, 3, 4): (b.__setitem__(slice(1, 3), A(9)), b.tolist())[1])())
show("contig-insert", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(1, 1), A(9)), b.tolist())[1])())
show("contig-empty-del", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(0, 2), A()), b.tolist())[1])())
show("self-assign", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(None, None, None), b), b.tolist())[1])())

# An extended slice assignment needs an exact length match and a negative step
# writes in reverse; a length mismatch is the extended-slice ValueError.
show("ext-assign", lambda: (lambda b=A(1, 2, 3, 4, 5): (b.__setitem__(slice(None, None, 2), A(7, 8, 9)), b.tolist())[1])())
show("ext-neg-step", lambda: (lambda b=A(1, 2, 3, 4, 5): (b.__setitem__(slice(None, None, -1), A(9, 8, 7, 6, 5)), b.tolist())[1])())
show("ext-badlen", lambda: (lambda b=A(1, 2, 3, 4, 5): (b.__setitem__(slice(None, None, 2), A(7, 8)), b.tolist())[1])())

# The right side must be an array of the same typecode, so a list or bytes is
# the "can only assign array" TypeError and a different code is "bad argument
# type"; a zero step is the slice-step error ahead of either check.
show("assign-list", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(0, 2), [9, 8]), b.tolist())[1])())
show("assign-bytes", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(0, 1), b'x'), b.tolist())[1])())
show("assign-int", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(0, 0), 5), b.tolist())[1])())
show("assign-codemismatch", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(0, 2), array.array('d', [9.0, 8.0])), b.tolist())[1])())
show("ext-codemismatch", lambda: (lambda b=A(1, 2, 3, 4, 5): (b.__setitem__(slice(None, None, 2), array.array('h', [7, 8, 9])), b.tolist())[1])())
show("step0-assign", lambda: (lambda b=A(1, 2, 3): (b.__setitem__(slice(None, None, 0), A(1)), b.tolist())[1])())

# Slice deletion drops a contiguous span or the extended step in reverse, and a
# zero step is again the slice-step error.
show("del-contig", lambda: (lambda b=A(1, 2, 3, 4): (b.__delitem__(slice(1, 3)), b.tolist())[1])())
show("del-ext", lambda: (lambda b=A(1, 2, 3, 4, 5): (b.__delitem__(slice(None, None, 2)), b.tolist())[1])())
show("del-ext-neg", lambda: (lambda b=A(1, 2, 3, 4, 5): (b.__delitem__(slice(None, None, -2)), b.tolist())[1])())
show("del-full", lambda: (lambda b=A(1, 2, 3): (b.__delitem__(slice(None, None, None)), b.tolist())[1])())
show("del-step0", lambda: (lambda b=A(1, 2, 3): (b.__delitem__(slice(None, None, 0)), b.tolist())[1])())


# The syntactic slice statements route the same way.
def syn():
    b = A(1, 2, 3, 4, 5)
    b[1:3] = A(9, 8, 7)
    del b[0:2]
    b[::2] = A(100, 200)
    del b[::2]
    return b.tolist()


show("syntactic", syn)
