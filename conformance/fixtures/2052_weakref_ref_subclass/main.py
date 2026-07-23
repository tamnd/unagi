# weakref.ref is a subclassable builtin type, the shape weakref.py leans on when
# it defines `class WeakMethod(ref)` and borrows `__hash__ = ref.__hash__` at
# import. Making ref a real type lets weakref, and copy through it, import.


import weakref
import copy


class Node:
    def __init__(self, name):
        self.name = name


n = Node("hi")

# A weak reference derefs to its object and reads its attributes back.
r = weakref.ref(n)
print(r() is n)
print(r().name)

# The referent-introspection helpers the module re-exports.
print(weakref.getweakrefcount(n))
print(len(weakref.getweakrefs(n)))

# ref is a real type: it answers __hash__ off the type object, the attribute
# WeakMethod borrows in its class body.
print(callable(weakref.ref.__hash__))

# A user subclass of ref carries the referent payload and stays an instance of
# ref, the WeakMethod relationship.
class MyRef(weakref.ref):
    __hash__ = weakref.ref.__hash__


m = MyRef(n)
print(type(m).__name__)
print(isinstance(m, weakref.ref))

# The rest of the _weakref surface weakref.py imports at once is present.
print(weakref.ProxyTypes == (weakref.ProxyType, weakref.CallableProxyType))
print(weakref.ReferenceType is weakref.ref)

# copy imports through the weakref machinery and copies its dispatched immutables.
print(copy.__name__)
print(copy.copy(5))
print(copy.copy("abc"))
print(copy.copy((1, 2)))
print(copy.deepcopy(7))
print("ok")
