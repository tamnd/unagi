# Builtin keyword arguments, wordings probed on 3.14.

print(1, 2, 3, sep=":", end="|\n")
print("a", "b", sep="", end="")
print()
try:
    print(1, sep=0)
except TypeError as e:
    print(e)
print(str(object=42))
print(sum([1, 2, 3], start=10))
try:
    sum(strt=[1])
except TypeError as e:
    print(e)
print(round(number=2.675, ndigits=2))
try:
    round(ndigits=2)
except TypeError as e:
    print(e)
try:
    round(1.5, number=2.5)
except TypeError as e:
    print(e)
print(pow(base=2, exp=10), pow(2, exp=5), pow(base=3, exp=4, mod=7))
try:
    pow(exp=3)
except TypeError as e:
    print(e)
print(list(enumerate("ab", start=5)))
print(list(enumerate(iterable="xy")))
try:
    enumerate(start=1)
except TypeError as e:
    print(e)
print(sorted([3, 1, 2], reverse=True))
print(sorted([2, 1], key=None))
try:
    sorted([1], revers=True)
except TypeError as e:
    print(e)
try:
    sorted([1, 2], key=5)
except TypeError as e:
    print(e)
print(min([], default=99), max([], default=None))
try:
    min(1, 2, default=0)
except TypeError as e:
    print(e)
try:
    min([1, 2], key=7)
except TypeError as e:
    print(e)
print(list(zip("ab", [1, 2], strict=True)))
try:
    for p in zip("abc", "ab", strict=True):
        print("row", p)
except ValueError as e:
    print(e)
try:
    list(zip("ab", "ab", "abcd", strict=True))
except ValueError as e:
    print(e)
print(dict(a=1, b=2))
print(dict([("x", 1), ("a", 5)], a=9, z=3))
try:
    len([1], x=2)
except TypeError as e:
    print(e)
try:
    format(3, spec="d")
except TypeError as e:
    print(e)
