# A builtin type or function pickles as a bare builtins.<name> global reference,
# the same by-name form CPython writes off __module__/__qualname__, and loads back
# to the very same object. This exercises the round-trip identity across the binary
# protocols, the raw byte shape at protocols 3-5, and the two-identity module-name
# memo (builtin types share one 'builtins' string, builtin functions a separate
# one). Protocol 2 rides the fix_imports name mapping (int -> __builtin__.long) on
# the dump side, a distinct concern, so only round-trip identity is checked there.
import pickle
import builtins as B


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


names = [
    "int", "float", "str", "bytes", "bytearray", "bool", "complex", "list",
    "dict", "set", "frozenset", "tuple", "object", "type", "len", "abs",
    "sorted", "reversed", "map", "filter", "zip", "enumerate", "range", "slice",
    "memoryview", "super", "staticmethod", "classmethod", "property", "min",
    "max", "sum", "repr", "getattr", "iter", "next", "hex", "ord", "chr",
    "print", "hash", "id", "divmod", "pow",
]

# Round-trip to identity across the binary protocols.
for p in (2, 3, 4, 5):
    for n in names:
        o = getattr(B, n)
        show("rt %s %d" % (n, p), lambda o=o, pp=p: pickle.loads(pickle.dumps(o, pp)) is o)

# Raw dump bytes at protocols 3-5, byte-identical to CPython.
for p in (3, 4, 5):
    for n in ["int", "len", "object", "dict", "map", "filter", "type"]:
        o = getattr(B, n)
        show("bytes %s %d" % (n, p), lambda o=o, pp=p: pickle.dumps(o, pp))

# Module-name memo grouping: types share one 'builtins', functions another, so a
# mixed tuple interleaves fresh writes and memo fetches.
show("mix bytes 4", lambda: pickle.dumps((int, len, str, abs, map, filter, list), 4))
show("mix bytes 5", lambda: pickle.dumps((int, len, str, abs, map, filter, list), 5))
show("pair bytes", lambda: pickle.dumps((len, abs, len), 4))

# A shared reference to the same builtin memoizes: both slots load as the one int.
show("shared", lambda: (lambda t: t[0] is t[1] is int)(pickle.loads(pickle.dumps((int, int), 5))))

# A builtin carried inside a container round-trips element by element.
show("in-list", lambda: pickle.loads(pickle.dumps([int, str, len, object], 5)) == [int, str, len, object])
