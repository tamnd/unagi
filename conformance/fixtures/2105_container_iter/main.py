# Every builtin container exposes __iter__ as a bound method that hands back a
# fresh iterator, and CPython gives each iterator its own type name. A dict
# iterates its keys, a frozenset shares the plain set iterator, and an all-ASCII
# str uses the compact str_ascii_iterator while a wider one uses str_iterator.
import array

cases = [
    [1, 2],
    (1, 2),
    {"a": 1, "b": 2},
    {1, 2},
    frozenset({1}),
    "abc",
    "café",
    b"ab",
    bytearray(b"xy"),
    range(2),
    array.array("i", [7, 8]),
]
for obj in cases:
    it = obj.__iter__()
    print(type(it).__name__, list(it))

# The iterator is its own iterator, so next() and a second walk share one
# cursor: after one next() the rest of the elements are what is left.
it = {"a": 1, "b": 2}.__iter__()
print(next(it))
print(list(it))

# The bound method reads back as a callable before it is called.
m = [10, 20].__iter__
print(list(m()))

# A dict iterates its keys, matching iter(dict).
print(list({"x": 1, "y": 2}.__iter__()))

# The section-proxy iteration configparser needs: keys() over a mapping walks
# with __iter__ under the hood.
import configparser

cp = configparser.ConfigParser()
cp.read_string("[db]\nhost = localhost\nport = 5432\n")
print(cp.getint("db", "port"))
print(list(cp["db"].keys()))
for k in cp["db"]:
    print(k, cp["db"][k])
print("host" in cp["db"])
