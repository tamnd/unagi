import array


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# __copy__ returns a fresh, independent array of the same typecode.
a = array.array("i", [1, 2, 3])
show("copy", lambda: a.__copy__())
show("copy_type", lambda: type(a.__copy__()).__name__)
show("copy_is_new", lambda: a.__copy__() is a)

# Mutating the copy must not touch the original.
b = a.__copy__()
b.append(9)
show("orig_after_copy_mutation", lambda: a.tolist())
show("copy_after_mutation", lambda: b.tolist())

# __copy__ takes no arguments.
show("copy_extra_arg", lambda: a.__copy__(1))

# __deepcopy__ takes exactly the memo argument and returns a fresh array.
show("deepcopy_none", lambda: a.__deepcopy__(None))
show("deepcopy_memo", lambda: a.__deepcopy__({}))
show("deepcopy_is_new", lambda: a.__deepcopy__(None) is a)
show("deepcopy_noarg", lambda: a.__deepcopy__())
show("deepcopy_two_args", lambda: a.__deepcopy__(None, None))

# Every typecode round-trips through both dunders.
show("copy_float", lambda: array.array("d", [1.5, -2.25]).__copy__())
show("deepcopy_float", lambda: array.array("d", [1.5, -2.25]).__deepcopy__(None))
show("copy_unicode", lambda: array.array("w", "héllo").__copy__().tolist())
show("copy_empty", lambda: array.array("b").__copy__())

# The bound methods read back and call the same.
f = a.__copy__
show("bound_copy", lambda: f().tolist())
g = a.__deepcopy__
show("bound_deepcopy", lambda: g({}).tolist())
