# __getattribute__ intercepts every read and delegates through super()
class Proxy:
    def __init__(self, target):
        object.__setattr__(self, "_target", target)
    def __getattribute__(self, name):
        if name == "kind":
            return "proxy"
        return super().__getattribute__(name)
p = Proxy({"a": 1})
p.extra = 9
print(p.kind, p.extra, p._target)

# __getattribute__ raising AttributeError falls through to __getattr__
class Lazy:
    def __getattribute__(self, name):
        return super().__getattribute__(name)
    def __getattr__(self, name):
        return "made:" + name
lz = Lazy()
lz.here = 1
print(lz.here, lz.missing)

# a plain method still resolves through a delegating __getattribute__
class Talker:
    def __getattribute__(self, name):
        return super().__getattribute__(name)
    def greet(self):
        return "hi"
print(Talker().greet())

# __setattr__ intercepts, transforms, delegates two ways
class Doubler:
    def __setattr__(self, name, value):
        object.__setattr__(self, name, value * 2)
class Wrapper:
    def __setattr__(self, name, value):
        super().__setattr__(name, "w:" + str(value))
d = Doubler(); d.n = 5
w = Wrapper(); w.s = 7
print(d.n, w.s)

# a read-only object rejects writes
class Frozen:
    def __setattr__(self, name, value):
        raise AttributeError("Frozen is read-only")
try:
    Frozen().x = 1
except AttributeError as e:
    print("frozen:", e)

# __delattr__ intercepts and delegates
class Watched:
    def __delattr__(self, name):
        print("removing", name)
        super().__delattr__(name)
watched = Watched(); watched.tmp = 1
del watched.tmp
print("has tmp:", hasattr(watched, "tmp"))

# a __delattr__ that refuses
class Sticky:
    def __delattr__(self, name):
        raise AttributeError("cannot delete " + name)
s = Sticky(); s.v = 1
try:
    del s.v
except AttributeError as e:
    print("sticky:", e)

# object slots reached directly
class Bag: pass
b = Bag()
object.__setattr__(b, "q", 3)
print(object.__getattribute__(b, "q"))
object.__delattr__(b, "q")
print("after direct del:", hasattr(b, "q"))
try:
    object.__getattribute__(b, "gone")
except AttributeError as e:
    print("direct miss:", e)

# a subclass overriding the parent's __getattribute__
class Base:
    def __getattribute__(self, name):
        return super().__getattribute__(name)
class Sub(Base):
    def __getattribute__(self, name):
        if name == "tag":
            return "sub"
        return super().__getattribute__(name)
sub = Sub(); sub.y = 2
print(sub.tag, sub.y)

# __getattribute__ and __setattr__ compose with a property
class Temp:
    def __init__(self):
        self._c = 0
    def __setattr__(self, name, value):
        super().__setattr__(name, value)
    @property
    def c(self):
        return self._c
    @c.setter
    def c(self, v):
        self._c = v
t = Temp(); t.c = 21
print("prop:", t.c, t._c)

# slot wrapper reprs are the object-objects form, address-free
print(repr(object.__getattribute__))
print(repr(object.__setattr__))
print(repr(object.__delattr__))
