# _weakref.ref and the pure-Python WeakSet built on it, the pair abc leans on
# for its class registries. This tier holds referents strongly, so a ref never
# goes dead; what matters here is that a ref calls back to its referent and
# hashes and compares by it, so a set of refs dedups by identity.

from _weakref import ref


class C:
    pass


a = C()
r1 = ref(a)
r2 = ref(a)

# Calling a ref returns its referent.
print(r1() is a)

# Two refs to one object are equal and hash alike, so they share a set slot.
print(r1 == r2, hash(r1) == hash(r2), hash(r1) == hash(a))
print(len({r1, r2}))

# Refs to different objects are distinct.
b = C()
print(ref(a) == ref(b), len({ref(a), ref(b)}))

# A ref accepts a callback argument, stored and ignored here.
print(ref(a, lambda w: None)() is a)

# A class and a function are weakly referenceable; an int is not.
print(ref(C)() is C)
try:
    ref(5)
except TypeError as e:
    print("no ref to int:", type(e).__name__)


from _weakrefset import WeakSet

ws = WeakSet()
ws.add(a)
ws.add(b)
ws.add(a)
print(len(ws), a in ws, b in ws, C() in ws)

# An unreferenceable value is simply not a member, through the try/except in
# __contains__.
print(5 in ws)

# Iteration yields the live referents, so gathering them recovers the members.
print({x for x in ws} == {a, b})

ws.discard(a)
print(len(ws), a in ws, b in ws)

# update and the set algebra WeakSet inherits.
ws2 = WeakSet()
ws2.update([a, b])
print(len(ws2))
ws2.remove(b)
print(len(ws2), a in ws2)
