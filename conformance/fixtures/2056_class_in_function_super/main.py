# A class defined inside a method, subclassing a builtin value type, the shape
# importlib._bootstrap._WeakValueDictionary builds when it defines KeyedRef in
# __init__. The nested class uses a zero-argument super() in __new__, has an
# instance method, and a staticmethod that reads an enclosing nonlocal, so the
# whole closure chain of a class-in-a-method exercises at once.


class Reg:
    def __init__(self):
        removed = []

        class KeyedRef(int):
            __slots__ = ()

            def __new__(cls, ob, key):
                self = super().__new__(cls, ob)
                return self

            def label(self):
                return "k" + str(int(self))

            @staticmethod
            def note():
                nonlocal removed
                removed.append("hit")
                return len(removed)

        self.cls = KeyedRef
        self.removed = removed


r = Reg()
k = r.cls(7, "key")
print(k.__class__.__qualname__)
print(int(k), k.label())
print(k.note(), k.note())
print(r.removed)
print(issubclass(r.cls, int), isinstance(k, int))
