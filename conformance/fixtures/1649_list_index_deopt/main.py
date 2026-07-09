# A function builds a scalar list local and reads it by index. The read lowers to
# a bounds-guarded static form: an in-range index takes the unboxed fast path,
# while a negative or out-of-range index fails the bounds guard and deopts to the
# boxed twin, which owns CPython's negative-index wraparound and its IndexError.
# The float parameters are dead weight that clears the guard budget so the unit
# proves static, and a*b is zero, so each call returns the read element unchanged.
# The first read stays on the fast path, the second deopts and wraps to the last
# element, and the third deopts and raises. All three must land on the same bytes
# as python3.14.
def pick(i: int, a: float, b: float) -> float:
    xs = [1.5, 2.5, 3.5]
    return a * b + a * b + a * b + a * b + a * b + a * b + a * b + xs[i]

print(pick(0, 1.0, 0.0))
print(pick(-1, 1.0, 0.0))
print(pick(9, 1.0, 0.0))
