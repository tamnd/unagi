# CPython's PyObject_RichCompareBool compares identity before equality, so a
# value that is not equal to itself -- a NaN -- is still found in a sequence that
# holds that very object. Membership, index, count, remove, and sequence equality
# all follow this rule for list, tuple, and deque.
from collections import deque


class NeverEq:
    def __eq__(self, other):
        return False

    def __hash__(self):
        return 1


nan = float("nan")
never = NeverEq()

# A NaN found in the containers that hold it, and a distinct NaN that is not.
lst = [1, nan, 2]
tup = (1, nan, 2)
dq = deque([1, nan, 2])
print(nan in lst, nan in tup, nan in dq)
print(float("nan") in lst)

# index and count see the self-unequal element by identity.
print(lst.index(nan), lst.count(nan))
print(tup.index(nan), tup.count(nan))

# A NeverEq object behaves the same way -- present by identity.
vals = (nan, 1, None, "abc", never)
for ctor in (list, tuple, dict.fromkeys, set, frozenset, deque):
    c = ctor(vals)
    print(ctor.__name__, all(e in c for e in c), c == ctor(vals), c == c)

# Sequence equality: sharing the same NaN object is equal; a distinct one is not.
print(lst == [1, nan, 2])
print(lst == [1, float("nan"), 2])
print(tup == (1, nan, 2))

# remove drops the NaN by identity.
lst.remove(nan)
print(lst)

# index of a missing self-unequal value still raises.
try:
    [1, 2].index(nan)
except ValueError as e:
    print("ValueError:", e)
