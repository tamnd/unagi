d = {"a": 1, "b": 2}
c = d.copy()
c["c"] = 3
print(d, c)
print(d.setdefault("a", 99), d.setdefault("z", 26), d)
print(d.popitem(), d)
d.update({"b": 20, "x": 7})
print(d)
d.update([("y", 8), ("b", 21)])
print(d)
d.update()
print(d)
print(d.fromkeys("ab"), d.fromkeys([1, 2], 0))
d.clear()
print(d, len(d))
e = {1: "one"}
f = e.copy()
e[2] = "two"
print(e, f)
try:
    {}.popitem()
except KeyError as e:
    print("caught", e)
try:
    d.update([("a", 1, 2)])
except ValueError as e:
    print("caught", e)
try:
    d.fromkeys([[1]])
except TypeError as e:
    print("caught", e)
try:
    d.update({}, {})
except TypeError as e:
    print("caught", e)
