# float() and int() honour an instance's conversion dunders. float() reads
# __float__, then __index__; int() reads __int__, then __index__. statistics.mean
# leans on float(Fraction), which is exactly the __float__ path.
from fractions import Fraction

print(float(Fraction(7, 2)))
print(int(Fraction(7, 2)))
print(int(Fraction(-9, 4)))


class HasFloat:
    def __float__(self):
        return 1.5


class HasInt:
    def __int__(self):
        return 7


class HasIndex:
    def __index__(self):
        return 9


class IntAndIndex:
    def __int__(self):
        return 1

    def __index__(self):
        return 2


print(float(HasFloat()), int(HasInt()))
# float() falls back to __index__, int() honours __index__ too.
print(float(HasIndex()), int(HasIndex()))
# __int__ wins over __index__ for int().
print(int(IntAndIndex()))
# float() has no __int__ fallback, only __index__.
try:
    float(HasInt())
except TypeError as e:
    print("float-no-int:", e)


class BadFloat:
    def __float__(self):
        return "x"


class BadInt:
    def __int__(self):
        return "y"


class BadIndex:
    def __index__(self):
        return "z"


try:
    float(BadFloat())
except TypeError as e:
    print("badfloat:", e)
try:
    int(BadInt())
except TypeError as e:
    print("badint:", e)
try:
    float(BadIndex())
except TypeError as e:
    print("badindex-float:", e)
try:
    int(BadIndex())
except TypeError as e:
    print("badindex-int:", e)

# statistics.mean now runs: it accumulates Fractions and converts at the end.
import statistics

print(statistics.mean([1, 2, 3, 4]))
print(statistics.mean([1.5, 2.5, 3.5]))
print(statistics.fmean([1, 2, 3, 4, 5]))
