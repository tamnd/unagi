import types


# A dict subclass used as a class namespace by a metaclass __prepare__.
class TrackingDict(dict):
    def __init__(self):
        super().__init__()
        self.order = []

    def __setitem__(self, key, value):
        if key not in self:
            self.order.append(key)
        super().__setitem__(key, value)


class Meta(type):
    @classmethod
    def __prepare__(metacls, name, bases):
        return TrackingDict()

    def __new__(metacls, name, bases, ns):
        # ns is the dict subclass, and it kept the definition order it saw.
        cls = super().__new__(metacls, name, bases, dict(ns))
        cls._order = [k for k in ns.order if not k.startswith("__")]
        return cls


class Sample(metaclass=Meta):
    a = 1
    b = 2

    def method(self):
        return self.a


print(Sample._order)
print(Sample.a, Sample.b)
print(Sample().method())


# dict.update from a mappingproxy.
proxy = types.MappingProxyType({"x": 10, "y": 20})
d = {"y": 0, "z": 30}
d.update(proxy)
print(d)

# dict.update from a dict subclass instance.
sub = TrackingDict()
sub["p"] = 1
sub["q"] = 2
d2 = {}
d2.update(sub)
print(d2)


# dict.update from a user mapping that only offers keys() and __getitem__.
class Mapping:
    def __init__(self, data):
        self._data = data

    def keys(self):
        return self._data.keys()

    def __getitem__(self, key):
        return self._data[key]


d3 = {"a": 0}
d3.update(Mapping({"a": 1, "b": 2}))
print(d3)

# The pair-sequence path still works.
d4 = {}
d4.update([("m", 1), ("n", 2)])
print(d4)
