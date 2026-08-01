# weakref exposes weak references (ref and proxy) and the weak-keyed and
# weak-valued containers built on them: WeakSet, WeakValueDictionary,
# WeakKeyDictionary, plus WeakMethod and finalize. This walks the live surface,
# meaning references whose referents are held for the whole run, which is what
# abc leans on for its class registries.
import weakref

class C:
    def __init__(self, n):
        self.n = n
    def __repr__(self):
        return "C(%d)" % self.n
    def method(self):
        return self.n * 10

# A ref calls back to its referent, and two refs to the same object are equal
# and hash alike, so a set of refs dedups by referent.
c = C(1)
r1 = weakref.ref(c)
r2 = weakref.ref(c)
print("ref alive:", r1() is c)
print("ref eq:", r1 == r2, hash(r1) == hash(r2))

# A proxy forwards attribute and method access straight to the referent.
p = weakref.proxy(c)
print("proxy:", p.n, p.method())

# WeakSet is a set that holds its members weakly; as a live container it adds,
# tests membership, iterates, discards and compares like a plain set.
ws = weakref.WeakSet()
items = [C(3), C(1), C(2)]
for it in items:
    ws.add(it)
print("ws len:", len(ws), items[0] in ws)
print("ws sorted:", sorted(x.n for x in ws))
ws.discard(items[0])
print("ws after discard:", sorted(x.n for x in ws))
print("ws subset:", weakref.WeakSet([items[1]]) <= ws)

# WeakValueDictionary holds its values weakly and its keys strongly.
wvd = weakref.WeakValueDictionary()
vals = [C(10), C(20)]
wvd["a"] = vals[0]
wvd["b"] = vals[1]
print("wvd:", wvd["a"].n, sorted(wvd.keys()), sorted(v.n for v in wvd.values()))

# WeakKeyDictionary holds its keys weakly and its values strongly.
wkd = weakref.WeakKeyDictionary()
keys = [C(100), C(200)]
wkd[keys[0]] = "x"
wkd[keys[1]] = "y"
print("wkd:", wkd[keys[0]], len(wkd))

# getweakrefcount and getweakrefs introspect the refs to an object.
print("count:", weakref.getweakrefcount(c) >= 1)
print("refs:", len(weakref.getweakrefs(c)) >= 1)

# WeakMethod holds a bound method weakly and rebuilds it on call.
class D:
    def greet(self):
        return "hi"
d = D()
wm = weakref.WeakMethod(d.greet)
print("weakmethod:", wm()())

# finalize registers a callback; calling it runs the callback once and marks
# the finalizer dead.
log = []
fin = weakref.finalize(c, log.append, "done")
print("finalize alive:", fin.alive)
fin()
print("finalize after:", log, fin.alive)
