import array


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# clear empties the array in place and returns None.
a = array.array("i", [1, 2, 3])
show("clear_return", lambda: a.clear())
show("after_clear", lambda: a.tolist())
show("len_after", lambda: len(a))

# The typecode survives, so the emptied array takes new elements of the same code.
a.append(7)
show("append_after_clear", lambda: a.tolist())
show("typecode_kept", lambda: a.typecode)

# clear on an already empty array is a no-op.
show("clear_empty", lambda: (lambda x: (x.clear(), x.tolist())[1])(array.array("d")))

# Every typecode clears the same way.
show("clear_float", lambda: (lambda x: (x.clear(), x.tolist())[1])(array.array("d", [1.5, 2.5])))
show("clear_unicode", lambda: (lambda x: (x.clear(), x.tolist())[1])(array.array("w", "hi")))

# clear takes no arguments.
show("clear_extra_arg", lambda: array.array("i", [1]).clear(1))

# The bound method reads back and calls the same.
b = array.array("b", [5, 6])
f = b.clear
show("bound_clear", lambda: (f(), b.tolist())[1])
