# A class that names dict as a base gets dict-backed instances: the mapping
# protocol and the inherited dict methods read and write the instance's own
# storage, and isinstance and issubclass report the dict layout.

class Plain(dict):
    pass


d = Plain()
d["a"] = 1
d["b"] = 2
print(d)
print(len(d), bool(d), bool(Plain()))
print(d["a"], d.get("b"), d.get("missing", -1))
print("a" in d, "z" in d)
print(sorted(d.keys()), sorted(d.values()))
print(sorted(d.items()))

del d["a"]
print(sorted(d.keys()))
try:
    d["nope"]
except KeyError as e:
    print("KeyError", e)

# Construction forwards to dict.__init__: a mapping, then keyword items.
seeded = Plain({"x": 10}, y=20)
print(sorted(seeded.items()))
print(sorted(Plain([("p", 1), ("q", 2)]).items()))

# isinstance and issubclass see the dict layout.
print(isinstance(d, dict), isinstance(d, Plain))
print(issubclass(Plain, dict), issubclass(Plain, object))
print(type(d).__name__)

# Iteration walks the keys in insertion order.
order = Plain()
for k in "cab":
    order[k] = k.upper()
print([k for k in order])
print(list(order.items()))


# A subclass may add methods and a custom __init__ that seeds the mapping
# through the inherited __setitem__, alongside plain instance attributes.
class Scored(dict):
    def __init__(self, **scores):
        self.origin = "scores"
        for name, value in scores.items():
            self[name] = value

    def total(self):
        return sum(self.values())


s = Scored(math=3, art=4)
print(sorted(s.items()), s.total(), s.origin)


# A deeper subclass of a dict subclass stays dict-backed.
class Deeper(Plain):
    pass


deep = Deeper()
deep["k"] = 9
print(isinstance(deep, dict), isinstance(deep, Plain), deep["k"])
