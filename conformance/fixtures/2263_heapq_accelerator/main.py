"""heapq on top of the _heapq C accelerator: the min and max primitives, the
heap invariant, and the argument-error surface."""
import heapq
import _heapq

# The public functions are the C ones (heapq.py does `from _heapq import *`).
print("heappush is C:", heapq.heappush is _heapq.heappush)
print("heapify_max is C:", heapq.heapify_max is _heapq.heapify_max)


def min_invariant(h):
    return all(h[(i - 1) >> 1] <= h[i] for i in range(1, len(h)))


def max_invariant(h):
    return all(h[(i - 1) >> 1] >= h[i] for i in range(1, len(h)))


data = [5, 1, 8, 3, 9, 2, 7, 0, 4, 6, 5, 1]

# Min-heap: push each, then drain -- comes out sorted, invariant holds throughout.
h = []
ok = True
for x in data:
    heapq.heappush(h, x)
    ok = ok and min_invariant(h)
print("push invariant:", ok)
drained = [heapq.heappop(h) for _ in range(len(data))]
print("min drain sorted:", drained == sorted(data))

# heapify builds the same heap in O(n); draining sorts.
h2 = data[:]
heapq.heapify(h2)
print("heapify invariant:", min_invariant(h2))
print("heapify drain sorted:", [heapq.heappop(h2) for _ in range(len(h2))] == sorted(data))

# heapreplace pops the root then inserts; heappushpop returns the smaller.
h3 = data[:]
heapq.heapify(h3)
print("heapreplace old root:", heapq.heapreplace(h3, 100))
print("heappushpop keeps small:", heapq.heappushpop(h3, -100))
print("still a heap:", min_invariant(h3))

# Max-heap variants mirror everything in reverse.
hm = []
for x in data:
    heapq.heappush_max(hm, x)
print("push_max invariant:", max_invariant(hm))
print("max drain desc:", [heapq.heappop_max(hm) for _ in range(len(data))] == sorted(data, reverse=True))
hm2 = data[:]
heapq.heapify_max(hm2)
print("heapify_max invariant:", max_invariant(hm2))
print("heapreplace_max old root:", heapq.heapreplace_max(hm2, 100))
print("heappushpop_max keeps large:", heapq.heappushpop_max(hm2, 1000))

# The argument-error surface: non-list -> TypeError, empty replace -> IndexError.
def show(fn, *args):
    try:
        fn(*args)
        print(fn.__name__, "NO-RAISE")
    except Exception as e:
        print(fn.__name__, "->", type(e).__name__)

show(heapq.heappush, [])            # wrong arg count
show(heapq.heappush, None, None)    # not a list
show(heapq.heappop, None)
show(heapq.heapify, None)
show(heapq.heapreplace, [], 0)      # empty heap
show(heapq.heappush_max, (1, 2), 0)
