# Exercises the native _abc accelerator directly and through abc.ABCMeta.
import _abc
import abc
from abc import ABC, abstractmethod

# _abc_init computes __abstractmethods__ for an ABCMeta class.
class Base(ABC):
    @abstractmethod
    def foo(self): ...
    @abstractmethod
    def bar(self): ...

print("abstractmethods", sorted(Base.__abstractmethods__))

# An abstract class refuses instantiation.
try:
    Base()
except TypeError:
    print("abstract refused")

# A concrete subclass clears the abstract set and instantiates.
class Impl(Base):
    def foo(self): return 1
    def bar(self): return 2
print("residual abstract", sorted(Impl.__abstractmethods__))
print("impl", Impl().foo(), Impl().bar())

# get_cache_token bumps on every register().
class V1: pass
class V2: pass
t0 = _abc.get_cache_token()
returned = Base.register(V1)
t1 = _abc.get_cache_token()
print("register returns arg", returned is V1)
print("token bumped", t1 != t0)

# register works as a decorator and makes a virtual subclass.
@Base.register
class V3: pass
print("decorator subclass", issubclass(V3, Base))

# Virtual subclass drives isinstance / issubclass.
print("isinstance virtual", isinstance(V1(), Base))
print("issubclass virtual", issubclass(V1, Base))
print("unrelated", issubclass(V2, Base), isinstance(V2(), Base))

# register only accepts classes.
try:
    Base.register(42)
except TypeError as e:
    print("register non-class", str(e))

# Refuses an inheritance cycle: cls is already a subclass of subclass.
class Child(Base):
    def foo(self): return 0
    def bar(self): return 0
try:
    Child.register(Base)
except RuntimeError as e:
    print("cycle", str(e))

# _get_dump exposes the (registry, cache, negative_cache, version) 4-tuple.
# The registry/cache membership is an implementation detail (CPython stores
# weakrefs), so only the tuple shape and the version type are portable.
dump = _abc._get_dump(Base)
print("dump len", len(dump))
reg, cache, negcache, ver = dump
print("cache is set", type(cache) is set)
print("negcache is set", type(negcache) is set)
print("registry is set", type(reg) is set)
print("ver is int", isinstance(ver, int))

# _reset_caches empties the positive/negative caches; a fresh check refills them.
print("cached before reset", issubclass(V1, Base))
_abc._reset_caches(Base)
print("cached after reset", issubclass(V1, Base))

# __subclasshook__ short-circuits the check: Sized recognizes any class with
# __len__ as a virtual subclass without an explicit register().
from _collections_abc import Sized
print("list Sized", issubclass(list, Sized))
print("int not Sized", issubclass(int, Sized))
