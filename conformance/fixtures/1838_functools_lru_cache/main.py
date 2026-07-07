# functools.lru_cache and cache, the memoizing wrappers the runtime owns behind
# the functools import. A bounded cache counts hits and misses and evicts the
# least recently used entry once it is full; cache_info reports the tallies as a
# CacheInfo namedtuple and cache_clear resets them. The wrapper is used bare and
# with arguments, typed keys 3 and 3.0 apart, maxsize None is unbounded, and
# maxsize 0 disables caching. The wrapper's own repr carries an address, so the
# fixture only prints the stable CacheInfo tuples and results.
import functools
from functools import lru_cache, cache

calls = []


@lru_cache(maxsize=2)
def sq(n):
    calls.append(n)
    return n * n


print(sq(2), sq(3), sq(2))
print(sq.cache_info())
sq(4)  # evicts the least recently used entry, 3
sq(3)  # recomputed
print(sq.cache_info())
print(calls)


@lru_cache
def bare(x):
    return x + 1


print(bare(10), bare(10))
print(bare.cache_info())


@lru_cache(maxsize=None)
def unb(x):
    return x


unb(1)
unb(2)
unb(1)
print(unb.cache_info())


@cache
def c(x):
    return x * 10


print(c(5), c(5))
print(c.cache_info())
c.cache_clear()
print(c.cache_info())


@lru_cache(typed=True)
def t(x):
    return x


t(3)
t(3.0)
t(3)
print(t.cache_info())


@lru_cache(typed=False)
def nt(x):
    return x


nt(3)
nt(3.0)
print(nt.cache_info())


@lru_cache
def kw(a=0, b=0):
    return (a, b)


kw(a=1, b=2)
kw(b=2, a=1)
print(kw.cache_info())

print(bare.__wrapped__ is not None)


@lru_cache(maxsize=0)
def z(x):
    return x


z(1)
z(1)
print(z.cache_info())
