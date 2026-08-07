# math.modf splits a float into its fractional and integer parts, both
# carrying the sign of the input. For an infinity C's modf returns a zero
# fraction with the infinity's sign and the infinity as the integer part,
# where Go's math.Modf hands back a nan fraction, so the infinity cases are
# the ones that had to be corrected. A nan input keeps nan for both parts.
import math


def show(label, x):
    frac, whole = math.modf(x)
    print(label, repr(frac), repr(whole))


show("inf", float("inf"))
show("neg-inf", float("-inf"))
show("nan", float("nan"))
show("neg-nan", float("-nan"))
show("pos", 3.75)
show("neg", -3.75)
show("zero", 0.0)
show("neg-zero", -0.0)
show("tiny", 5e-324)
show("big-int-float", 1e16)
show("half", 0.5)
show("neg-half", -0.5)

# the fractional part of an infinity keeps the sign, so copysign sees it
fi, _ = math.modf(float("inf"))
fni, _ = math.modf(float("-inf"))
print("inf-frac-sign", math.copysign(1.0, fi))
print("neg-inf-frac-sign", math.copysign(1.0, fni))
