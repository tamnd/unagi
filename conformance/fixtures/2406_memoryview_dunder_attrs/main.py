# A memoryview exposes its own dunders as readable instance attributes, the
# buffer analog of the surfaces bytes and the containers carry. Each bound read
# routes through the same operator the interpreter already runs for len(mv),
# mv[i], mv == x and iter(mv), so the attribute and the operator agree on the
# result and the errors. A memoryview exposes no __contains__, unlike a list.


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


mv = memoryview(bytearray(b"abc"))

# hasattr answers True across the protocol, comparison, hash and context dunders,
# and False for the __contains__ a memoryview does not carry.
names = [
    "__len__", "__getitem__", "__setitem__", "__delitem__", "__iter__",
    "__contains__", "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__",
    "__hash__", "__enter__", "__exit__",
]
print("has:", [n for n in names if hasattr(mv, n)])

# The size, subscript and iterate dunders route through the operators.
show("__len__", lambda: mv.__len__())
show("__getitem__", lambda: mv.__getitem__(0))
show("__setitem__", lambda: mv.__setitem__(0, 122))
show("after set", lambda: mv.tobytes())
show("__delitem__", lambda: mv.__delitem__(0))
print("iter:", list(mv.__iter__()), type(mv.__iter__()).__name__)

# __eq__ and __ne__ compare bytes against a buffer operand and decline a
# non-buffer with NotImplemented; the ordering slots always decline.
show("__eq__ bytes", lambda: mv.__eq__(b"zbc"))
show("__eq__ bytearray", lambda: mv.__eq__(bytearray(b"zbc")))
show("__eq__ memoryview", lambda: mv.__eq__(memoryview(b"zbc")))
show("__eq__ list", lambda: mv.__eq__([1, 2, 3]))
show("__eq__ int", lambda: mv.__eq__(5))
show("__ne__ bytes", lambda: mv.__ne__(b"zbc"))
show("__lt__", lambda: mv.__lt__(memoryview(b"zzz")))
show("__ge__ int", lambda: mv.__ge__(5))

# __hash__ hashes a read-only view by its bytes and rejects a writable one.
show("ro __hash__ eq", lambda: memoryview(b"abc").__hash__() == hash(b"abc"))
show("rw __hash__", lambda: mv.__hash__())

# The context-manager pair returns the view then releases it.
with memoryview(bytearray(b"q")) as v:
    show("in with", lambda: v.__len__())

# The wrapper arity errors match CPython, including the named __setitem__ one.
show("__len__(1)", lambda: mv.__len__(1))
show("__getitem__()", lambda: mv.__getitem__())
show("__getitem__(0,1)", lambda: mv.__getitem__(0, 1))
show("__setitem__(0)", lambda: mv.__setitem__(0))
show("__eq__()", lambda: mv.__eq__())

# A released view still binds its dunder wrappers, but a call through one raises.
r = memoryview(bytearray(b"x"))
r.release()
print("released has __len__:", hasattr(r, "__len__"))
show("r.__len__()", lambda: r.__len__())
show("r.__enter__()", lambda: r.__enter__())
show("r.__hash__()", lambda: r.__hash__())
show("r.__eq__ bytes", lambda: r.__eq__(b"x"))
