import weakref


class C:
    pass


c = C()
r = weakref.ref(c)
print(r() is c)                  # a plain ref is callable


# weakref.py's KeyedRef is a ref subclass that stores a key; _ModuleLock in
# importlib reads one by calling it, so a subclass instance has to be callable.
class KeyedRef(weakref.ref):
    def __new__(type, ob, callback, key):
        self = weakref.ref.__new__(type, ob, callback)
        self.key = key
        return self

    def __init__(self, ob, callback, key):
        super().__init__(ob, callback)


k = KeyedRef(c, None, "id42")
print(k.key)
print(k() is c)                  # inherited call returns the referent
print(callable(k))


# A subclass that reaches the base call through super().
class SuperRef(weakref.ref):
    def deref(self):
        return super().__call__()


s = SuperRef(c)
print(s.deref() is c)


# WeakValueDictionary uses KeyedRef internally; this is the importlib path.
d = weakref.WeakValueDictionary()
d["k"] = c
print(d["k"] is c)
print(sorted(d.keys()))
print("k" in d, len(d))
