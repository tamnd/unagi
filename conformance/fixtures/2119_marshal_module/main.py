# marshal is a builtin module. importlib/_bootstrap_external imports it to read
# and write .pyc code objects, a path unagi never takes, so the slice covers the
# documented value surface: dump/dumps/load/loads round-trip the basic types and
# an unsupported value raises ValueError. The byte format carries interning and
# reference details unagi does not reproduce, so this tests round-trip equality
# rather than exact bytes.

import marshal
import sys

# marshal is a builtin the way the other C accelerators are.
print("marshal" in sys.builtin_module_names)
print(marshal.version)
print(callable(marshal.dumps), callable(marshal.loads))
print(callable(marshal.dump), callable(marshal.load))

# dumps produces bytes; loads reads the first object back.
print(type(marshal.dumps(0)).__name__)

# Each value type round-trips through dumps/loads unchanged.
values = [
    None, True, False, ...,
    0, 1, -1, 255, 256, 65535, 2 ** 31, -(2 ** 31),
    10 ** 40, -(10 ** 40),
    3.5, -2.25, 0.0,
    complex(1.5, -2.25),
    "", "hello", "wide é中",
    b"", b"bytes\x00\xff",
    (), (1, 2, 3), (1, (2, 3), "x"),
    [], [4, 5, [6, 7]],
    {}, {"a": 1, 2: "b", (3, 4): [5, 6]},
    {1, 2, 3}, frozenset({4, 5, 6}),
]
for v in values:
    r = marshal.loads(marshal.dumps(v))
    print(type(r).__name__, r == v)

# Nesting round-trips as a whole.
nested = {"list": [1, {"set": frozenset({7, 8})}], "tuple": (None, True, b"z")}
print(marshal.loads(marshal.dumps(nested)) == nested)

# A bytearray marshals as bytes, matching CPython.
r = marshal.loads(marshal.dumps(bytearray(b"buf")))
print(type(r).__name__, r)

# Trailing bytes past the first object are ignored.
blob = marshal.dumps(42) + b"leftover"
print(marshal.loads(blob))

# dump and load drive a file object's write and read.
import io

buf = io.BytesIO()
marshal.dump([1, "two", 3.0], buf)
buf.seek(0)
print(marshal.load(buf))

# A value marshal cannot represent raises ValueError.
try:
    marshal.dumps(lambda: 0)
except ValueError as e:
    print("dumps error:", e)
