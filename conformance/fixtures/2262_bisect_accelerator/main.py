"""bisect on top of the _bisect C accelerator: the four primitives, key=, lo/hi
windows, the insort mutators, and the argument-error surface."""
import bisect
import _bisect

# The public functions are the C ones (bisect.py does `from _bisect import *`).
print("bisect_left is C:", bisect.bisect_left is _bisect.bisect_left)
print("bisect is bisect_right:", bisect.bisect is bisect.bisect_right)
print("insort is insort_right:", bisect.insort is bisect.insort_right)

a = [1, 2, 2, 2, 3, 5]
# Left lands before a run of equals, right after it.
print("left:", [bisect.bisect_left(a, x) for x in range(7)])
print("right:", [bisect.bisect_right(a, x) for x in range(7)])

# lo/hi confine the search; outside elements are invisible.
print("windowed left:", bisect.bisect_left(a, 2, 3, 6))
print("windowed right:", bisect.bisect_right(a, 2, 0, 3))
print("hi default spans all:", bisect.bisect_right(a, 5))

# key= searches on the transformed value.
data = [2, -4, 6, 8, -10]  # abs -> 2,4,6,8,10, sorted
print("keyed left:", [bisect.bisect_left(data, x, key=abs) for x in (2, 4, 6, 8, 10)])
print("keyed right:", [bisect.bisect_right(data, x, key=abs) for x in (2, 4, 6, 8, 10)])

# insort keeps a list sorted; left vs right decides duplicate placement.
b = [1, 3, 5]
bisect.insort_left(b, 3)
bisect.insort_right(b, 4)
bisect.insort(b, 0)
print("insorted:", b)

# insort with a key inserts the original value at the keyed position.
words = ["a", "ccc", "dddd"]
bisect.insort(words, "bb", key=len)
print("keyed insort:", words)

# A custom __lt__ returning a non-bool is taken for its truth, like list.sort.
class Weird:
    def __init__(self, v):
        self.v = v
    def __lt__(self, other):
        return [self.v < other.v]  # truthy non-bool
w = [Weird(1), Weird(3), Weird(5)]
print("non-bool lt index:", bisect.bisect_left(w, Weird(4)))

# Argument errors mirror the accelerator's clinic.
try:
    bisect.bisect_left(a, 2, -1)
except ValueError as e:
    print("negative lo:", e)
try:
    bisect.bisect_left(a, 2, 0, 1, 3, 4)
except TypeError as e:
    print("too many positional:", type(e).__name__)
try:
    bisect.bisect_left(x=2)
except TypeError as e:
    print("missing a:", type(e).__name__)

# Big search space: bisecting a maxsize-long range exercises the overflow-safe
# midpoint (a naive (lo+hi)/2 would overflow to a negative index).
import sys
r = range(sys.maxsize)
print("big bisect:", bisect.bisect_left(r, 2 ** 40))
