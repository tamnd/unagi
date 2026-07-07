# type(name, bases, ns, **kwds) forwards its keywords to the winning metaclass
# picked from the bases, so a metaclass __new__/__init__ that names them sees
# them, and a metaclass that refuses extra keywords surfaces its own binding
# error, catchable.

class Meta(type):
    def __new__(mcls, name, bases, ns, **kwds):
        print("new", sorted(kwds.items()))
        ns["_kwds"] = sorted(kwds.items())
        return super().__new__(mcls, name, bases, ns)

    def __init__(cls, name, bases, ns, **kwds):
        print("init", sorted(kwds.items()))
        super().__init__(name, bases, ns)


class Base(metaclass=Meta):
    pass


X = type("X", (Base,), {"a": 1}, color="red", n=3)
print(type(X).__name__)
print(X.a)
print(X._kwds)


class Strict(type):
    def __new__(mcls, name, bases, ns):
        return super().__new__(mcls, name, bases, ns)


class SBase(metaclass=Strict):
    pass


try:
    type("Q", (SBase,), {}, bad=1)
except TypeError as e:
    print("caught", type(e).__name__)
