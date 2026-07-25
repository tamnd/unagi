def f(**kw):
    return kw


class Map:
    def keys(self):
        return ["x", "y"]

    def __getitem__(self, k):
        return k.upper()


print(f(**Map()))


class DS(dict):
    def keys(self):
        return ["OVERRIDE"]


d = DS()
d["a"] = 1
d["b"] = 2
print(f(**d))

print(f(**{"a": 1, "b": 2, "c": 3}))


try:
    f(**5)
except TypeError as e:
    print("E1:", e)


class NoKeys:
    def __getitem__(self, k):
        return 1


try:
    f(**NoKeys())
except TypeError as e:
    print("E2:", e)

try:
    f(x=1, **{"x": 2})
except TypeError as e:
    print("E3:", e)

try:
    f(**{1: 2})
except TypeError as e:
    print("E4:", e)


class C:
    def m(self, **kw):
        return kw


print(C().m(**Map()))
