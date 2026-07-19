import pickle

# pickle serializes an object to a byte stream a stack machine can replay. The
# exact bytes are observable, so this fixture prints the hex of pickle.dumps for
# each scalar leaf and checks it against CPython byte for byte, then confirms
# loads rebuilds the value. Only the scalar leaves and the binary protocols this
# slice supports appear; containers and the object protocol come later.

print("default protocol:", pickle.DEFAULT_PROTOCOL)
print("highest protocol:", pickle.HIGHEST_PROTOCOL)

scalars = [
    None,
    True,
    False,
    0,
    1,
    255,
    256,
    65535,
    65536,
    -1,
    -256,
    2**31 - 1,
    2**31,
    2**63,
    -(2**63),
    2**63 - 1,
    2**100,
    -(2**100),
    3.14,
    0.0,
    -0.0,
    -2.5e300,
    "",
    "hi",
    "héllo",
    b"",
    b"ab",
    bytes([0, 1, 2, 255]),
]

# Byte-identity at the default protocol (5): print the wire bytes and round-trip.
for v in scalars:
    data = pickle.dumps(v)
    back = pickle.loads(data)
    print(repr(v), data.hex(), repr(back), back == v)

# The same value across the binary protocols keeps its protocol-specific bytes.
for proto in (2, 3, 4, 5):
    for v in (256, "hi", b"ab"):
        # bytes need SHORT_BINBYTES, which is protocol 3+; skip the pair pickle
        # would only reach through the object protocol here.
        if isinstance(v, bytes) and proto < 3:
            continue
        data = pickle.dumps(v, protocol=proto)
        print(proto, repr(v), data.hex(), pickle.loads(data) == v)

# Ordered containers: tuples pick a fixed opcode by arity, lists and dicts
# stream their contents in batches, and nesting and sharing round-trip.
containers = [
    (),
    (1,),
    (1, 2),
    (1, 2, 3),
    (1, 2, 3, 4),
    (1, (2, 3)),
    [],
    [1],
    [1, 2, 3],
    [1, [2, 3], "x"],
    {},
    {"k": 9},
    {"a": 1, "b": 2},
    {"nested": [1, {"deep": 2}], "t": (3, 4)},
]
for v in containers:
    data = pickle.dumps(v)
    print(repr(v), data.hex(), pickle.loads(data) == v)

# A shared child is written once and referenced by a memo get afterwards.
child = [1]
shared = [child, child]
sdata = pickle.dumps(shared)
sback = pickle.loads(sdata)
print("shared:", sdata.hex(), sback[0] is sback[1])

# A self-referential list closes on itself through the memo.
cyc = []
cyc.append(cyc)
cdata = pickle.dumps(cyc)
cback = pickle.loads(cdata)
print("cyclic:", cdata.hex(), cback[0] is cback)

# A negative protocol means the highest; None means the default.
print("neg proto:", pickle.dumps(42, protocol=-1).hex())
print("none proto:", pickle.dumps(42, protocol=None).hex())

# A value above the highest protocol is rejected the way CPython rejects it.
try:
    pickle.dumps(1, protocol=99)
except ValueError as e:
    print("high proto:", e)

# A type with no pickle support raises TypeError, naming the type.
try:
    pickle.dumps(pickle)
except TypeError as e:
    print("unpicklable:", e)
