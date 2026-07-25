import collections


class M(dict):
    def __missing__(self, key):
        return "default:" + key


m = M()
m["a"] = 1
print(m["a"])
print(m["z"])
print("z" in m)
print(dict(m))
print(m.get("q"))


class Store(dict):
    def __missing__(self, key):
        self[key] = len(key)
        return self[key]


s = Store()
print(s["hello"])
print(dict(s))


class N(dict):
    pass


try:
    N()["x"]
except KeyError as e:
    print("KeyError", e)


dd = collections.defaultdict(int)
dd["a"] += 1
print(dd["a"], dd["new"], dict(dd))
c = collections.Counter("aab")
print(c["a"], c["z"], "z" in c)
