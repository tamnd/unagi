import array
import operator
import types


def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


d = {1: 10, 2: 20, 3: 30}

# The sequence and mapping iterators report the remaining count through
# __length_hint__, counting down as the cursor advances (PEP 424).
print("== iterators that expose __length_hint__ ==")
hinted = [
    ("list", [1, 2, 3, 4]),
    ("tuple", (1, 2, 3, 4)),
    ("str", "abcd"),
    ("bytes", b"abcd"),
    ("bytearray", bytearray(b"abcd")),
    ("range", range(4)),
    ("dict", d),
    ("dict_keys", d.keys()),
    ("dict_values", d.values()),
    ("dict_items", d.items()),
    ("set", {1, 2, 3, 4}),
    ("frozenset", frozenset({1, 2, 3, 4})),
    ("mappingproxy", types.MappingProxyType(d)),
    ("reversed_list", reversed([1, 2, 3, 4])),
    ("reversed_tuple", reversed((1, 2, 3, 4))),
    ("reversed_str", reversed("abcd")),
    ("reversed_range", reversed(range(4))),
    ("reversed_dict", reversed({1: 1, 2: 2, 3: 3, 4: 4})),
    ("reversed_bytes", reversed(b"abcd")),
    ("reversed_bytearray", reversed(bytearray(b"abcd"))),
    ("reversed_array", reversed(array.array("i", [1, 2, 3, 4]))),
]
for name, obj in hinted:
    it = iter(obj)
    show(name + " full", lambda it=it: it.__length_hint__())
    next(it)
    next(it)
    show(name + " after 2", lambda it=it: it.__length_hint__())
    show(name + " operator", lambda it=it: operator.length_hint(it))

print("== a drained iterator hints zero ==")
drained = iter([1, 2])
next(drained)
next(drained)
show("drained hint", lambda: drained.__length_hint__())
show("drained operator", lambda: operator.length_hint(drained))

print("== the __iter__ handle hints the same as iter() ==")
show("list __iter__ hint", lambda: [1, 2, 3].__iter__().__length_hint__())

print("== the buffer iterators carry no __length_hint__ ==")
for name, obj in [("array", array.array("i", [1, 2, 3])), ("memoryview", memoryview(b"abcd"))]:
    it = iter(obj)
    show(name + " hasattr", lambda it=it: hasattr(it, "__length_hint__"))
    show(name + " operator falls back", lambda it=it: operator.length_hint(it))
    show(name + " operator default", lambda it=it: operator.length_hint(it, 99))

print("== lazy and paired iterators carry no __length_hint__ ==")
show("generator hasattr", lambda: hasattr((x for x in [1]), "__length_hint__"))
show("map hasattr", lambda: hasattr(map(str, [1]), "__length_hint__"))
show("filter hasattr", lambda: hasattr(filter(None, [1]), "__length_hint__"))
show("zip hasattr", lambda: hasattr(zip([1]), "__length_hint__"))
show("enumerate hasattr", lambda: hasattr(enumerate([1]), "__length_hint__"))

print("== operator.length_hint argument handling ==")
show("no __length_hint__ default 0", lambda: operator.length_hint(map(str, [1, 2])))
show("bad default type", lambda: operator.length_hint([], "x"))
show("too many args", lambda: operator.length_hint([], 0, 0))
