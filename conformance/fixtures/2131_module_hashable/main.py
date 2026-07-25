import sys
import math

# A module hashes by identity: it works as a set element and a dict key, and two
# reads of the same module collide while distinct modules stay separate.
s = {math, sys, math}
print(len(s))
print(math in s, sys in s)
print(hash(math) == hash(math))
print(hash(math) == hash(sys))

d = {math: "m", sys: "s"}
print(d[math], d[sys])

# The dedupe site.abs_paths relies on: many modules through a set.
mods = set(sys.modules.values())
print(all(m in mods for m in (sys, math)))
print(len(set(sys.modules.values())) == len(set(id(m) for m in sys.modules.values())))
