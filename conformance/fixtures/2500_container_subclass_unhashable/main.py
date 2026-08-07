def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A subclass of list, dict or set inherits __hash__ = None from its mutable base,
# so it is unhashable unless it defines its own __hash__.
class L(list):
    pass


class D(dict):
    pass


class S(set):
    pass


print("== hash() on a mutable-container subclass ==")
show("hash L", lambda: hash(L([1, 2])))
show("hash D", lambda: hash(D(a=1)))
show("hash S", lambda: hash(S([1, 2])))

print("== keying a dict with such a subclass ==")
show("{L: 1}", lambda: {L([1]): 1})
show("{D: 1}", lambda: {D(): 1})
show("{S: 1}", lambda: {S([1]): 1})

print("== using such a subclass as a set element ==")
show("{L}", lambda: {L([1])})
show("{D}", lambda: {D()})
show("{S}", lambda: {S([1])})

print("== membership takes the same hash path ==")
show("L in dict", lambda: L() in {1: 2})
show("D in dict", lambda: D() in {1: 2})

print("== the report names the outermost key, the innermost unhashable ==")
show("tuple(L) key", lambda: {(L(),): 1})
show("tuple(1, D) key", lambda: {(1, D()): 1})

print("== a subclass that defines __hash__ stays hashable ==")


class LH(list):
    def __hash__(self):
        return 7


show("hash LH", lambda: hash(LH([1, 2, 3])))
show("{LH: 1}", lambda: repr({LH([9]): 1}))


class DH(dict):
    def __hash__(self):
        return 11


show("hash DH", lambda: hash(DH()))

print("== a subclass that defines only __eq__ stays unhashable ==")


class LE(list):
    def __eq__(self, o):
        return NotImplemented


show("hash LE", lambda: hash(LE([1])))

print("== immutable-container subclasses stay hashable ==")


class T(tuple):
    pass


class FS(frozenset):
    pass


show("hash T", lambda: hash(T([1, 2])))
show("T keys by value", lambda: repr({T([1, 2]): "a", (1, 2): "b"}))
show("hash FS is int", lambda: type(hash(FS([1, 2]))).__name__)

print("== scalar value subclasses keep hashing by their payload ==")


class MyInt(int):
    pass


show("hash MyInt", lambda: hash(MyInt(5)))
show("MyInt shares 5's slot", lambda: repr({MyInt(5): "a", 5: "b"}))
