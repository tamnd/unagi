from collections import ChainMap, defaultdict

# construction
print(ChainMap().maps)
c = ChainMap({"a": 1, "b": 2}, {"b": 20, "c": 30})
print(c["a"], c["b"], c["c"])           # search order: first wins for b
print("b" in c, "z" in c)
print(len(c))                           # unique keys
print(list(c), dict(c))
print(list(c.keys()), list(c.values()), list(c.items()))
print(c.get("c"), c.get("z"), c.get("z", 99))
print(bool(ChainMap()), bool(ChainMap({}, {})), bool(c))
print(repr(c))

# writes only touch maps[0]
c["b"] = 200
c["d"] = 4
print(c.maps[0], c.maps[1])
del c["b"]
print(c.maps[0])
try:
    del c["c"]
except KeyError as e:
    print("del", e)
try:
    ChainMap({}).popitem()
except KeyError as e:
    print("popitem", e)
try:
    c.pop("nope")
except KeyError as e:
    print("pop", e)
print(c.pop("d"), c.maps[0])

# new_child / parents
base = ChainMap({"x": 1})
child = base.new_child({"y": 2})
print(child["x"], child["y"], len(child.maps))
child2 = base.new_child(z=9)
print(child2.maps[0])
child3 = base.new_child({"p": 1}, q=2)
print(sorted(child3.maps[0].items()))
print(child.parents.maps == base.maps)

# copy independence
cp = c.copy()
cp["a"] = 999
print(c["a"], cp["a"], c.maps[1] is cp.maps[1])

# maps identity and reassign
m0 = c.maps[0]
c.maps[0]["new"] = 1
print(m0 is c.maps[0], "new" in c)
c.maps = [{"only": 1}]
print(dict(c))

# fromkeys
print(dict(ChainMap.fromkeys(["a", "b"], 0)))
print(dict(ChainMap.fromkeys("xy")))

# or operators
d1 = ChainMap({"a": 1})
merged = d1 | {"a": 9, "b": 2}
print(type(merged).__name__, merged["a"], merged["b"], d1["a"])
r = {"a": 5, "z": 1} | d1
print(type(r).__name__, dict(r))
d1 |= {"c": 3}
print(d1.maps[0])

# update / setdefault
u = ChainMap({"a": 1})
u.update({"b": 2}, c=3)
print(sorted(u.maps[0].items()))
print(u.setdefault("a", 100), u.setdefault("d", 4))
print(sorted(u.maps[0].items()))

# equality as mapping
print(ChainMap({"a": 1, "b": 2}) == {"a": 1, "b": 2})
print(ChainMap({"a": 1}, {"a": 9}) == {"a": 1})
print(ChainMap({"a": 1}) == {"a": 2})
print(ChainMap({"a": 1}) != {"a": 2})
print(ChainMap({"a": 1}) == 5)

dd = defaultdict(int, {"a": 1})
c = ChainMap({"b": 2}, dd)
print(c["a"], "a" in c, dict(dd))   # contains must not create
print("z" in c, dict(dd))           # miss must not create
# nested chainmap value repr
inner = ChainMap({"k": 1})
outer = ChainMap({"nested": inner})
print(repr(outer))
# empty first map
e = ChainMap({}, {"x": 9})
print(e["x"], list(e), len(e))
# three overlapping maps ordering
t = ChainMap({"a": 1, "d": 4}, {"a": 9, "b": 2}, {"c": 3, "a": 8})
print(list(t))
print(type(ChainMap().maps).__name__)
