# heapq and bisect ship a pure-Python implementation and then override it with
# the _heapq / _bisect C accelerator via `from _mod import *` guarded by
# ImportError. The accelerators are SOFT: the modules must run correctly without
# them. This proves the pure path by masking the accelerator in sys.modules so
# the import fails and the pure functions stand, then exercises the full surface.
import sys
import importlib

# Masking the accelerator to None makes `from _heapq import *` raise ImportError,
# so reload falls back to the pure Python definitions.
sys.modules['_heapq'] = None
import heapq
importlib.reload(heapq)

# The public functions are now the pure Python ones, not the C builtins.
print("heappush kind:", type(heapq.heappush).__name__)
print("heapify kind:", type(heapq.heapify).__name__)


def min_invariant(h):
    return all(h[(i - 1) >> 1] <= h[i] for i in range(1, len(h)))


def max_invariant(h):
    return all(h[(i - 1) >> 1] >= h[i] for i in range(1, len(h)))


data = [5, 1, 8, 3, 9, 2, 7, 0, 4, 6, 5, 1]

# push then drain comes out sorted, invariant holds throughout.
h = []
ok = True
for x in data:
    heapq.heappush(h, x)
    ok = ok and min_invariant(h)
print("pure push invariant:", ok)
print("pure min drain sorted:", [heapq.heappop(h) for _ in range(len(data))] == sorted(data))

# heapify builds the heap in place; heapreplace and heappushpop combine ops.
h2 = data[:]
heapq.heapify(h2)
print("pure heapify invariant:", min_invariant(h2))
print("pure heapreplace root:", heapq.heapreplace(h2, 100))
print("pure heappushpop small:", heapq.heappushpop(h2, -100))

# The max variants are pure too.
hm = data[:]
heapq.heapify_max(hm)
print("pure heapify_max invariant:", max_invariant(hm))
print("pure max drain desc:", [heapq.heappop_max(hm) for _ in range(len(hm))] == sorted(data, reverse=True))

# merge, nlargest and nsmallest live only in pure heapq; they never had a C form.
print("merge:", list(heapq.merge([1, 4, 7], [2, 3, 8], [0, 5, 6])))
print("nlargest:", heapq.nlargest(3, data))
print("nsmallest:", heapq.nsmallest(3, data))
print("nlargest key:", heapq.nlargest(2, ["a", "ccc", "bb", "dddd"], key=len))

# Same proof for bisect.
sys.modules['_bisect'] = None
import bisect
importlib.reload(bisect)

print("bisect_left kind:", type(bisect.bisect_left).__name__)
print("insort_right kind:", type(bisect.insort_right).__name__)

a = [1, 2, 2, 2, 3, 5]
print("pure left:", [bisect.bisect_left(a, x) for x in range(7)])
print("pure right:", [bisect.bisect_right(a, x) for x in range(7)])
print("pure windowed:", bisect.bisect_left(a, 2, 3, 6), bisect.bisect_right(a, 2, 0, 3))

# key= searches on the transformed value.
signed = [2, -4, 6, 8, -10]
print("pure keyed:", [bisect.bisect_left(signed, x, key=abs) for x in (2, 4, 6, 8, 10)])

# insort keeps a list sorted; left vs right decides duplicate placement.
b = [1, 3, 5]
bisect.insort_left(b, 3)
bisect.insort_right(b, 4)
bisect.insort(b, 0)
print("pure insorted:", b)

words = ["a", "ccc", "dddd"]
bisect.insort(words, "bb", key=len)
print("pure keyed insort:", words)
