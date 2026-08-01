# dict.fromkeys called on the C-accelerated collections subclasses the vendored
# package re-exports: OrderedDict and defaultdict answer on the type and on an
# instance and keep their own kind, while Counter steers callers away.
from collections import OrderedDict, defaultdict, Counter

# On the type object.
print(OrderedDict.fromkeys(["a", "b", "c"], 0))
print(OrderedDict.fromkeys("xy"))
print(type(OrderedDict.fromkeys(["a"])).__name__)

print(defaultdict.fromkeys(["a", "b"], 0))
print(defaultdict.fromkeys("xy"))
print(type(defaultdict.fromkeys(["a"])).__name__)

# On an instance: the result is type(self), and its own contents never leak in.
od = OrderedDict(z=9)
print(od.fromkeys(["m", "n"]))
print(type(od.fromkeys(["m"])).__name__)

# A defaultdict instance's factory is not carried onto the fromkeys result.
dd = defaultdict(int, p=1)
print(dd.fromkeys(["q"], 5))

# A missing-key read on the fromkeys defaultdict has no factory, so it raises.
d = defaultdict.fromkeys(["a"], 1)
try:
    d["z"]
except KeyError as e:
    print("keyerror", e)

# Counter overrides fromkeys to steer callers to Counter(iterable).
try:
    Counter("ab").fromkeys(["x"])
except NotImplementedError as e:
    print("counter", str(e))
