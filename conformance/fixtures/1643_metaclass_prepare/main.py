# Metaclass __prepare__ and the class namespace: the mapping __prepare__
# returns receives every synthesized member and body binding in definition
# order, and the same object reaches the metaclass __new__ and __init__.

class LogNS:
    def __init__(self):
        self.data = {}
        self.order = []

    def __setitem__(self, key, value):
        self.order.append(key)
        self.data[key] = value

    def __getitem__(self, key):
        return self.data[key]

    def keys(self):
        return list(self.data)


class Meta(type):
    @classmethod
    def __prepare__(mcs, name, bases, **kwargs):
        print("prepare:", mcs.__name__, name, len(bases), sorted(kwargs))
        return LogNS()

    def __new__(mcs, name, bases, ns, **kwargs):
        print("new sees:", ns.order)
        plain = {k: ns[k] for k in ns.keys()}
        cls = super().__new__(mcs, name, bases, plain)
        cls._built_from = ns
        return cls

    def __init__(cls, name, bases, ns, **kwargs):
        print("init same ns:", ns is cls._built_from)
        super().__init__(name, bases, {k: ns[k] for k in ns.keys()})


class Vars(metaclass=Meta, flavor="salty"):
    """vars doc"""
    a = 1
    b = 2
    a = 3

print("Vars.a:", Vars.a)
print("Vars.b:", Vars.b)
print("Vars.__doc__:", Vars.__doc__)
print("Vars.__module__:", Vars.__module__)
print("Vars.__firstlineno__:", Vars.__firstlineno__)


class DunderLog(type):
    @classmethod
    def __prepare__(mcs, name, bases, **kwargs):
        return LogNS()

    def __new__(mcs, name, bases, ns, **kwargs):
        for key in ns.order:
            if not key.startswith("__"):
                print("bound:", key)
        return super().__new__(mcs, name, bases, {k: ns[k] for k in ns.keys()})


class Methods(metaclass=DunderLog):
    kind = "m"

    def __init__(self, v):
        self.v = v
        self.tag, self.extra = v, v

    def clear(self):
        del self.v
        def helper():
            self.deep = 1
        return helper

print("static:", Methods.__static_attributes__)
m = Methods(5)
print("m.v:", m.v)


def plain_prepare(name, bases, **kwargs):
    print("plain prepare:", name)
    return {"injected": 42}


class PlainMeta(type):
    __prepare__ = plain_prepare


class Seeded(metaclass=PlainMeta):
    pass

print("Seeded.injected:", Seeded.injected)


class BadPrep(type):
    @classmethod
    def __prepare__(mcs, name, bases, **kwargs):
        return 7

try:
    class Broken(metaclass=BadPrep):
        pass
except TypeError as e:
    print("bad prep:", e)


class RaisingPrep(type):
    @classmethod
    def __prepare__(mcs, name, bases, **kwargs):
        raise ValueError("prep boom")

try:
    class Boom(metaclass=RaisingPrep):
        pass
except ValueError as e:
    print("raising prep:", e)


class PureToDefault(type):
    @classmethod
    def __prepare__(mcs, name, bases, **kwargs):
        return LogNS()

try:
    class NoConvert(metaclass=PureToDefault):
        pass
except TypeError as e:
    print("pure ns to type.__new__:", e)


class Ordinary:
    """ordinary doc"""

    def touch(self):
        self.z = 1

print("Ordinary.__module__:", Ordinary.__module__)
print("Ordinary.__doc__:", Ordinary.__doc__)
print("Ordinary.__firstlineno__:", Ordinary.__firstlineno__)
print("Ordinary static:", Ordinary.__static_attributes__)
