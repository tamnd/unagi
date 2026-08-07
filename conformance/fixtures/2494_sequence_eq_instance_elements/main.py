from fractions import Fraction as F
from decimal import Decimal as D

nan = float("nan")

# A user-number instance sitting in a sequence compares by its own __eq__ against
# the builtin it represents, the way a direct == does.
print("tuple:", (F(1, 1), F(5, 6)) == (1, F(5, 6)))
print("list:", [F(1), F(5, 6)] == [1, F(5, 6)])
print("divmod:", (F(1), F(5, 6)) == divmod(F(7, 3), F(3, 2)))
print("reversed:", (1, F(5, 6)) == (F(1, 1), F(5, 6)))
print("nested:", [(F(1), 2)] == [(1, 2)])
print("decimal:", (D('1.5'), 2) == (D('1.5'), 2))

# Membership and the sequence methods run the same per-element comparison.
print("in:", 1 in (F(1, 1), F(2, 1)))
print("frac in list:", F(1, 1) in [1, 2, 3])
print("index:", (F(1, 1), F(5, 6)).index(1))
print("count:", [F(1, 1), 1, F(2, 2)].count(1))

# Identity still short-circuits so a tuple holding the same NaN object is equal,
# while two distinct NaNs are not.
print("nan same:", (nan,) == (nan,))
print("nan distinct:", (float("nan"),) == (float("nan"),))

# A plain unequal case stays unequal.
print("unequal:", (F(1, 2), 3) == (F(1, 3), 3))
