import array


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# array.array's iterator is a C type declared in the array module, so its
# tp_name carries the module the way CPython's does: "array.arrayiterator". The
# type reprs and reports its module module-qualified, while the bare tail stays
# the __name__ and __qualname__. memoryview's iterator lives in builtins, so it
# stays bare, which this fixture keeps alongside as the contrast.
a = array.array("i", [1, 2, 3])
it = iter(a)

print("== the iterator type names itself module-qualified ==")
show("type name", lambda: type(it).__name__)
show("type qualname", lambda: type(it).__qualname__)
show("type module", lambda: type(it).__module__)
show("type str", lambda: str(type(it)))
show("type repr", lambda: repr(type(it)))

print("== the instance repr and errors carry the module too ==")
# The instance repr ends in a heap address, so match only the module-qualified
# head rather than the whole line.
show("instance repr head", lambda: repr(it).startswith("<array.arrayiterator object"))
show("missing attribute", lambda: getattr(it, "nope"))
# array.arrayiterator carries no __length_hint__, and the AttributeError names
# the type module-qualified.
show("length_hint absent", lambda: it.__length_hint__())

print("== the same name comes from iter() and from a for-loop's __iter__ ==")
show("iter() result module", lambda: type(iter(a)).__module__)
show("iter() result str", lambda: str(type(iter(a))))

print("== a subclass iterates through the same arrayiterator type ==")


class Vec(array.array):
    pass


sv = iter(Vec("d", [1.0, 2.0]))
show("subclass iter module", lambda: type(sv).__module__)
show("subclass iter str", lambda: str(type(sv)))

print("== memoryview's iterator stays bare in builtins ==")
mi = iter(memoryview(b"abcd"))
show("mv iter module", lambda: type(mi).__module__)
show("mv iter str", lambda: str(type(mi)))
show("mv instance repr head", lambda: repr(mi).startswith("<memory_iterator object"))

print("== iterating still yields the elements ==")
print("drain", list(iter(a)))
