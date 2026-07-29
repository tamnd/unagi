# operator.py defines every function in pure Python, then does
# `from _operator import *` to override them with the C accelerator. This
# exercises that surface: the star-import must bind the native functions, and
# each must match the pure semantics it replaces.
import operator
import _operator

# The public name resolves to the native accelerator, not the pure fallback.
print("routed", operator.add is _operator.add)

# Arithmetic and bitwise.
print(operator.add(2, 3), operator.sub(10, 4), operator.mul(3, 4))
print(operator.truediv(7, 2), operator.floordiv(7, 2), operator.mod(7, 3), operator.pow(2, 10))
print(operator.and_(6, 3), operator.or_(4, 1), operator.xor(5, 3))
print(operator.lshift(1, 4), operator.rshift(32, 2), operator.inv(5), operator.invert(5))
print(operator.neg(3), operator.pos(-3), operator.abs(-9))

# Comparisons.
print(operator.lt(1, 2), operator.le(2, 2), operator.eq(2, 3),
      operator.ne(2, 3), operator.gt(3, 2), operator.ge(2, 3))

# Logical and identity.
print(operator.not_(0), operator.truth([]), operator.truth([0]))
print(operator.is_(None, None), operator.is_not(1, 2))
print(operator.is_none(None), operator.is_none(0), operator.is_not_none(5))

# Sequence operations.
print(operator.getitem([10, 20, 30], 1), operator.contains([1, 2, 3], 2))
print(operator.concat([1, 2], [3, 4]), operator.concat("ab", "cd"))
print(operator.countOf([1, 2, 2, 3, 2], 2), operator.indexOf([5, 6, 7], 7))
d = {}
operator.setitem(d, "k", 9)
print(d)
operator.delitem(d, "k")
print(d)

# index() reads __index__, preserving arbitrary precision.
print(operator.index(True), operator.index(10 ** 30))


class Hint:
    def __length_hint__(self):
        return 42


print(operator.length_hint([1, 2, 3]), operator.length_hint(Hint()), operator.length_hint(5, 7))


# call() forwards positional and keyword arguments.
def collect(*args, **kwargs):
    return (args, sorted(kwargs.items()))


print(operator.call(collect, 1, 2, x=3, y=4))

# The generalized-lookup helpers stay pure but keep working after the import.
print(operator.itemgetter(1)([9, 8, 7]), operator.itemgetter(0, 2)([9, 8, 7]))
print(operator.attrgetter("real")(3 + 0j), operator.methodcaller("upper")("hi"))

# Error behavior: concat rejects a non-sequence, index rejects a non-integer.
try:
    operator.concat(3, 4)
except TypeError as e:
    print("concat:", "concatenated" in str(e))
try:
    operator.index("x")
except TypeError as e:
    print("index:", "integer" in str(e))
try:
    operator.indexOf([1, 2, 3], 9)
except ValueError as e:
    print("indexOf:", "not in sequence" in str(e))
