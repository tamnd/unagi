# The collections.deque type, provided in Go behind the collections import.
# This exercises construction and repr, the end operations, maxlen eviction,
# rotate, the search and count methods, indexing, iteration, equality, and the
# maxlen attribute, all consumed the way a Python program would.
import collections
from collections import deque

d = deque([1, 2, 3])
print(d)
print(len(d), bool(d), bool(deque()))

d.append(4)
d.appendleft(0)
print(d)
print(d.pop(), d.popleft())
print(d)

d.extend([5, 6])
d.extendleft([-1, -2])
print(d)

print(d[0], d[-1], d[3])
d[0] = 99
print(d)

d2 = deque([1, 2, 3, 4, 5])
d2.rotate(2)
print(d2)
d2.rotate(-3)
print(d2)

d2.reverse()
print(d2)

d3 = deque([1, 2, 3, 2, 1])
print(d3.index(2), d3.count(1), d3.count(2))
d3.remove(2)
print(d3)
d3.insert(1, 42)
print(d3)

bounded = deque([1, 2, 3], maxlen=3)
print(bounded, bounded.maxlen)
bounded.append(4)
print(bounded)
bounded.appendleft(0)
print(bounded)
print(deque().maxlen)

print(list(deque([7, 8, 9])))
print(deque([1, 2]) == deque([1, 2]), deque([1, 2]) == deque([1, 3]))
print(deque([1, 2]) == [1, 2])

seeded = deque(range(5), 2)
print(seeded)

c = d3.copy()
c.append(100)
print(d3, c)


def pop_empty():
    try:
        deque().pop()
    except IndexError as e:
        return str(e)


print(pop_empty())
