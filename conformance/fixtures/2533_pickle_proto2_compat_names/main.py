# Protocol 2 turns fix_imports on, so a builtin global goes out under its
# Python 2 spelling: int as __builtin__.long, map as itertools.imap, a builtin
# exception under the exceptions module. Protocol 3 and up keep the modern
# builtins name. This checks the raw bytes at protocol 2 and 3, the round trip
# back to the live object, and the same for globals nested in a container.
import pickle
import builtins


def show(label, fn):
    try:
        print(label, fn())
    except Exception as e:
        print(label, type(e).__name__, ":", e)


# Raw protocol 2 bytes: the Python 2 name that fix_imports writes.
name_mapped = ["int", "str", "chr", "range", "map", "filter", "zip"]
for n in name_mapped:
    obj = getattr(builtins, n)
    print("p2", n, pickle.dumps(obj, 2))

# A builtin function with no name entry only has its module remapped.
module_only = ["len", "abs", "sorted", "repr", "getattr", "object", "type"]
for n in module_only:
    obj = getattr(builtins, n)
    print("p2", n, pickle.dumps(obj, 2))

# Builtin exceptions all move to the old exceptions module at protocol 2.
excs = ["ValueError", "TypeError", "KeyError", "IndexError", "Exception",
        "OSError", "StopIteration", "RuntimeError", "ZeroDivisionError"]
for n in excs:
    obj = getattr(builtins, n)
    print("p2", n, pickle.dumps(obj, 2))

# Protocol 3 keeps the modern builtins name for the same set.
for n in name_mapped + module_only[:3] + excs[:3]:
    obj = getattr(builtins, n)
    print("p3", n, pickle.dumps(obj, 3))

# Round trip: a name mapped or module mapped global at protocol 2 loads back to
# the very same object. Compare identity against the live builtin.
for n in name_mapped + module_only + excs:
    obj = getattr(builtins, n)
    data = pickle.dumps(obj, 2)
    show("rt2 " + n, lambda obj=obj, data=data: pickle.loads(data) is obj)

# Round trip at protocol 3 for good measure.
for n in name_mapped + excs:
    obj = getattr(builtins, n)
    data = pickle.dumps(obj, 3)
    show("rt3 " + n, lambda obj=obj, data=data: pickle.loads(data) is obj)

# Globals nested in a container keep the same protocol 2 mapping, and the memo
# still shares a repeated reference. A tuple of (int, map, int) writes two
# distinct globals and fetches the third back.
tup = (int, map, int)
print("p2 tuple", pickle.dumps(tup, 2))
back = pickle.loads(pickle.dumps(tup, 2))
print("tuple rt", back[0] is int, back[1] is map, back[2] is int, back[0] is back[2])

# A list mixing a name mapped global, a module mapped one, and an exception.
mixed = [str, len, ValueError]
data = pickle.dumps(mixed, 2)
print("p2 mixed", data)
back = pickle.loads(data)
print("mixed rt", back[0] is str, back[1] is len, back[2] is ValueError)

# Loading the exact bytes CPython writes for a Python 2 global resolves forward
# to the live Python 3 object.
show("xload long", lambda: pickle.loads(b'\x80\x02c__builtin__\nlong\nq\x00.') is int)
show("xload imap", lambda: pickle.loads(b'\x80\x02citertools\nimap\nq\x00.') is map)
show("xload unicode", lambda: pickle.loads(b'\x80\x02c__builtin__\nunicode\nq\x00.') is str)
show("xload exc", lambda: pickle.loads(b'\x80\x02cexceptions\nValueError\nq\x00.') is ValueError)
