# copy.copy is a shallow copy and copy.deepcopy recurses, with the __copy__,
# __deepcopy__, __reduce_ex__ and __reduce__ hooks steering both. This exercises
# the pure copy module over builtin containers, custom classes, the override
# hooks, the memo that preserves shared references, and a self-referential cycle.
import copy

# Shallow copy of a nested list: the outer list is new, the inner list is shared.
outer = [[1, 2], [3, 4]]
shallow = copy.copy(outer)
print("shallow new outer:", shallow is not outer)
print("shallow shares inner:", shallow[0] is outer[0])
shallow[0].append(99)
print("mutation seen by original:", outer[0])

# Deep copy of the same shape: nothing is shared.
src = [[1, 2], [3, 4]]
deep = copy.deepcopy(src)
print("deep new inner:", deep[0] is not src[0])
deep[0].append(7)
print("deep leaves original:", src[0])

# Dicts and sets round-trip; sets are printed sorted to stay deterministic.
d = {"a": [1], "b": [2]}
dd = copy.deepcopy(d)
print("dict deep inner:", dd["a"] is not d["a"], dd == d)
s = {1, 2, 3}
print("set deepcopy:", sorted(copy.deepcopy(s)))

# Atomic immutables copy to themselves.
n = 42
print("atomic int:", copy.deepcopy(n) is n)
print("atomic str:", copy.copy("hi") == "hi")
print("atomic None:", copy.deepcopy(None) is None)

# Tuples: a shallow copy of a tuple returns the same object.
t = (1, 2, 3)
print("tuple shallow same:", copy.copy(t) is t)
# A tuple of mutable items deepcopies to a new tuple with new items.
tm = ([1], [2])
tmc = copy.deepcopy(tm)
print("tuple deep new:", tmc is not tm, tmc[0] is not tm[0], tmc == tm)

# A plain class copies through its __dict__.
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __repr__(self):
        return f"Point({self.x}, {self.y})"

p = Point([1], 2)
ps = copy.copy(p)
pd = copy.deepcopy(p)
print("class shallow shares attr:", ps.x is p.x)
print("class deep copies attr:", pd.x is not p.x, pd.x == p.x)
print("class repr:", pd)

# __copy__ and __deepcopy__ hooks take over.
class Hooked:
    def __init__(self, tag):
        self.tag = tag
    def __copy__(self):
        return Hooked("shallow-of-" + self.tag)
    def __deepcopy__(self, memo):
        return Hooked("deep-of-" + self.tag)

h = Hooked("x")
print("hook copy:", copy.copy(h).tag)
print("hook deepcopy:", copy.deepcopy(h).tag)

# __reduce__ drives deepcopy when no hook is present.
class Reduced:
    def __init__(self, payload):
        self.payload = payload
    def __reduce__(self):
        return (Reduced, (self.payload,))

r = Reduced([1, 2])
rc = copy.deepcopy(r)
print("reduce deep new:", rc is not r, rc.payload is not r.payload, rc.payload == r.payload)

# The memo preserves shared references: one inner list referenced twice stays one.
inner = [1, 2]
shared = [inner, inner]
sc = copy.deepcopy(shared)
print("memo shares:", sc[0] is sc[1], sc[0] is not inner)

# A self-referential list deepcopies into its own cycle.
cyc = [1]
cyc.append(cyc)
cc = copy.deepcopy(cyc)
print("cycle rebuilt:", cc[1] is cc, cc is not cyc, cc[0])

# copy.deepcopy accepts an explicit memo dict and populates it.
memo = {}
obj = [1, [2, 3]]
copy.deepcopy(obj, memo)
print("explicit memo populated:", len(memo) > 0)
