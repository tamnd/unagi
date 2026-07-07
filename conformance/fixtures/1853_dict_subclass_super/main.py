# A dict subclass reaches the builtin dict base through super(), the shape
# enum's EnumDict relies on: super().__init__() to seed the store and
# super().__setitem__ from an overriding __setitem__.
class Recorder(dict):
    def __init__(self):
        super().__init__()
        self.order = []

    def __setitem__(self, key, value):
        if not key.startswith("_"):
            self.order.append(key)
        super().__setitem__(key, value)


r = Recorder()
r["RED"] = 1
r["GREEN"] = 2
r["_hidden"] = 3
print(r)
print(r["RED"], r["GREEN"])
print(r.order)
print(len(r))
print("RED" in r, "BLUE" in r)
print(list(r.keys()))
print(list(r.items()))
print(dict(r))
print(isinstance(r, dict))


# super().__delitem__ and super().__contains__ through overrides.
class Guarded(dict):
    def __delitem__(self, key):
        if super().__contains__(key):
            super().__delitem__(key)


g = Guarded()
g["a"] = 1
g["b"] = 2
del g["a"]
del g["missing"]
print(dict(g))


# The attribute-read form: bind super().__setitem__ then call it later.
class Seeder(dict):
    def load(self, pairs):
        setter = super().__setitem__
        for k, v in pairs:
            setter(k, v)


s = Seeder()
s.load([("x", 10), ("y", 20)])
print(s)


# super().update reaching the builtin base, mapping plus keyword items.
class Merger(dict):
    def fill(self):
        super().update({"a": 1}, b=2)


m = Merger()
m.fill()
print(m)
