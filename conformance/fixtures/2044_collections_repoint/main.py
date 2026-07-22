# The _collections accelerator exposes deque, defaultdict and OrderedDict as the
# C container types collections/__init__.py imports by underscore name. They are
# real type objects, not plain constructors: the vendored package registers
# deque with an abc and code runs isinstance against them, so each answers
# isinstance, issubclass and type() and reprs as a class. _weakref.proxy is the
# transparent forwarding reference the same package imports unconditionally. The
# public collections module shares these three type objects.
import _collections
import _weakref
import collections

d = _collections.deque([1, 2, 3], maxlen=3)
d.append(4)
print("deque:", list(d), "maxlen:", d.maxlen)
print("deque class repr:", repr(_collections.deque))
print("deque is a type:", isinstance(_collections.deque, type))
print("deque instance:", isinstance(d, _collections.deque))
print("deque self-subclass:", issubclass(_collections.deque, _collections.deque))
print("deque subclasses object:", issubclass(_collections.deque, object))
print("type(d) is deque:", type(d) is _collections.deque)

dd = _collections.defaultdict(list)
dd["a"].append(1)
print("defaultdict:", dict(dd), isinstance(dd, _collections.defaultdict))

od = _collections.OrderedDict(a=1, b=2)
od.move_to_end("a")
print("ordereddict:", list(od.items()), isinstance(od, _collections.OrderedDict))
print("ordereddict class repr:", repr(_collections.OrderedDict))


class C:
    def __init__(self):
        self.x = 5


# Keep the referent alive so the proxy stays valid: this tier holds its referent
# strongly, and CPython keeps it through the local binding.
c = C()
p = _weakref.proxy(c)
print("proxy forwards:", p.x)

print("shared deque type:", collections.deque is _collections.deque)
print("counter still works:", collections.Counter("aab").most_common(1))
