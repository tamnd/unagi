import array
import collections
import types


def name(obj):
    return type(obj).__name__


d = {1: 10, 2: 20}

# The iter() builtin now reports the same iterator type CPython gives a
# container's own __iter__, rather than a generic name. Each builtin container
# has its own iterator type, so type(iter(seq)).__name__ is the faithful name.
print("== iter() reports the container's iterator type ==")
cases = [
    ("list", [1, 2]),
    ("tuple", (1, 2)),
    ("str", "ab"),
    ("str_nonascii", "éè"),
    ("bytes", b"ab"),
    ("bytearray", bytearray(b"ab")),
    ("range", range(2)),
    ("dict", d),
    ("dict_keys", d.keys()),
    ("dict_values", d.values()),
    ("dict_items", d.items()),
    ("set", {1, 2}),
    ("frozenset", frozenset({1, 2})),
    ("mappingproxy", types.MappingProxyType(d)),
    ("array", array.array("i", [1, 2])),
    ("memoryview", memoryview(b"ab")),
    ("OrderedDict", collections.OrderedDict([(1, 2)])),
    ("defaultdict", collections.defaultdict(int, {1: 2})),
    ("Counter", collections.Counter({1: 2})),
    ("mappingproxy_odict", types.MappingProxyType(collections.OrderedDict([(1, 2)]))),
]
for label, obj in cases:
    print(label, "=>", name(iter(obj)))

print("== iter(seq) agrees with the container's own iterator ==")
# Every one of these builds the same iterator either way, so a for loop and a
# hand-rolled iter() call walk an identically typed cursor.
for label, obj in [("list", [1, 2]), ("bytes", b"ab"), ("range", range(3)), ("set", {1, 2})]:
    a = name(iter(obj))
    b = name(iter(iter(obj)))
    print(label, "iter==iter(iter)", a == b, a)

print("== iter() stays idempotent ==")
it = iter([1, 2, 3])
print("iter(it) is it", iter(it) is it)
print("iter(it) type", name(iter(it)))

print("== the lazy and paired iterators keep their own names ==")


def gen():
    yield 1


lazy = [
    ("enumerate", enumerate([1])),
    ("zip", zip([1], [2])),
    ("map", map(str, [1])),
    ("filter", filter(None, [1])),
    ("reversed_list", reversed([1, 2])),
    ("generator", gen()),
]
for label, obj in lazy:
    print(label, "=>", name(iter(obj)))

print("== a subclass of a builtin container iterates as the base ==")


class MyList(list):
    pass


class MyDict(dict):
    pass


class MySet(set):
    pass


class MyTuple(tuple):
    pass


class MyStr(str):
    pass


for label, obj in [
    ("list_sub", MyList([1, 2])),
    ("dict_sub", MyDict({1: 2})),
    ("set_sub", MySet({1, 2})),
    ("tuple_sub", MyTuple((1, 2))),
    ("str_sub", MyStr("ab")),
]:
    print(label, "=>", name(iter(obj)))

print("== two-argument iter is a callable_iterator ==")
print("callable", name(iter(lambda: 0, 1)))

print("== an old-style __getitem__ sequence gets the generic iterator ==")


class Seq:
    def __getitem__(self, i):
        if i < 3:
            return i
        raise IndexError


print("getitem", name(iter(Seq())))
