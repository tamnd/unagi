import collections


# A defaultdict subclass inherits the factory-fill behavior and stays a dict.
class Grouped(collections.defaultdict):
    pass


g = Grouped(list)
g["a"].append(1)
g["a"].append(2)
g["b"].append(3)
print("groups", dict(g))
print("factory is list", g.default_factory is list)
print("isinstance defaultdict", isinstance(g, collections.defaultdict))
print("isinstance dict", isinstance(g, dict))
print("issubclass defaultdict", issubclass(Grouped, collections.defaultdict))
print("issubclass dict", issubclass(Grouped, dict))

# Inherited dict methods work on the subclass instance.
g["b"].append(4)
print("keys", sorted(g.keys()))
print("get missing", g.get("z", "none"))

# default_factory is settable and drives later missing-key fills.
g.default_factory = int
print("count", g["c"])


# A subclass can override __missing__ the way a hand-written dict subclass does.
class Constant(collections.defaultdict):
    def __missing__(self, key):
        return "const:" + key


c = Constant(None)
print("override", c["anything"])


# super().__init__ routes through defaultdict.__init__ to seed the factory.
class Seeded(collections.defaultdict):
    def __init__(self):
        super().__init__(list)


s = Seeded()
s["k"].append(9)
print("seeded", dict(s))
print("seeded factory is list", s.default_factory is list)

# A plain defaultdict is a dict instance too.
plain = collections.defaultdict(set)
print("plain isinstance dict", isinstance(plain, dict))
print("counter isinstance dict", isinstance(collections.Counter(), dict))
print("ordered isinstance dict", isinstance(collections.OrderedDict(), dict))
