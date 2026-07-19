import pickle

# A set and a frozenset pickle their elements in CPython's set-iteration order,
# which is the hash-table slot order. That order is observable in the pickle
# bytes, so it must match CPython slot for slot. The harness pins
# PYTHONHASHSEED=0, so the order is deterministic across runs. Every set here is
# built from an explicit list or by .add(), not a set display, so the insertion
# order is unambiguous and the same value is compared byte for byte.

print("default protocol:", pickle.DEFAULT_PROTOCOL)

# Small sets, a collision chain, a resize past the initial table, string
# elements (hashed with siphash under the pinned seed), and negative ints where
# -1 and -2 share a hash and collide.
sets = [
    set(),
    set([1]),
    set([1, 2, 3]),
    set([1, 2, 3, 17, 33]),
    set([8, 16, 24, 1]),
    set(["a", "b", "c"]),
    set([-1, -2, -3, -4, -5]),
    set(range(20)),
    set(range(50)),
]
for v in sets:
    data = pickle.dumps(v)
    print(sorted(v), data.hex(), pickle.loads(data) == v)

# Frozensets take the FROZENSET opcode and memoize after their members.
frozens = [
    frozenset(),
    frozenset([1, 2, 3]),
    frozenset(["x", "y", "z"]),
    frozenset(range(12)),
]
for v in frozens:
    data = pickle.dumps(v)
    print(sorted(v), data.hex(), pickle.loads(data) == v)

# A set built incrementally lands in insertion order, exactly what the pickler
# walks; adding a duplicate does not move an element.
s = set()
for x in [5, 3, 9, 3, 1, 5]:
    s.add(x)
sdata = pickle.dumps(s)
print("incremental:", sdata.hex(), pickle.loads(sdata) == s)

# A frozenset shared through a tuple is written once and fetched back by memo.
f = frozenset([1, 2])
shared = pickle.dumps((f, f))
sb = pickle.loads(shared)
print("shared frozenset:", shared.hex(), sb[0] is sb[1])

# Sets nest inside other containers and each keeps its own slot order.
nested = pickle.dumps({"a": set([1, 2]), "b": [frozenset([3, 4])]})
print("nested:", nested.hex(), pickle.loads(nested) == {"a": set([1, 2]), "b": [frozenset([3, 4])]})

# Protocols 4 and 5 carry the native EMPTY_SET and FROZENSET opcodes; protocols
# 2 and 3 pickle a set through the object-reduction protocol, builtins.set (named
# __builtin__ under protocol 2's fix_imports) applied to a list of the elements
# in the same slot order. Every protocol keeps the bytes byte for byte, and each
# round-trips back to the value.
for proto in (2, 3, 4, 5):
    for v in (set([1, 2, 3, 17, 33]), frozenset([9, 8, 7])):
        data = pickle.dumps(v, protocol=proto)
        print("proto", proto, data.hex(), pickle.loads(data) == v)
