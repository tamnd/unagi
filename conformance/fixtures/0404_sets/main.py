s = {3, 1, 2}
print(len(s), sorted(s))
print(1 in s, 5 in s)
s.add(5)
s.discard(1)
print(sorted(s))
t = {2, 5, 7}
print(sorted(s & t), sorted(s | t), sorted(s - t), sorted(s ^ t))
print({1, 2} <= {1, 2, 3}, {1, 2} < {1, 2}, {1, 2, 3} >= {1, 2})
fs = frozenset([1, 2, 2, 3])
print(len(fs), sorted(fs))
print(sorted(fs | {4}))
print({1} == frozenset([1]))
d = {fs: "key"}
print(d[frozenset([3, 2, 1])])
print(set())
print({9})
print(frozenset())
e = set()
e.update([1, 2])
print(e.issubset({1, 2, 3}), e.isdisjoint({7}))
print(sorted({1, 2}.union([2, 3])))
print(sorted({1, 2, 3}.intersection([2, 3, 4])))
print(sorted({1, 2, 3}.difference([3])))
c = {1, 2}.copy()
c.clear()
print(c)
try:
    {1}.remove(9)
except KeyError as err:
    print("caught", err)
try:
    bad = {[1]}
except TypeError as err:
    print("caught", err)
try:
    print({1} | [2])
except TypeError as err:
    print("caught", err)
try:
    d2 = {{1}: 2}
except TypeError as err:
    print("caught", err)
