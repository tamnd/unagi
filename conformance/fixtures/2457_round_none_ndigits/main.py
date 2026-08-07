# round(x, None) is the single-argument round: for a built-in number it returns
# an int (banker's rounding, nan raising ValueError) just like round(x) with no
# second argument, while a Decimal, a Fraction and a custom __round__ keep their
# own None handling.
def show(label, fn):
    try:
        r = fn()
        print(label, type(r).__name__, repr(r))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


import decimal
import fractions

D = decimal.Decimal
F = fractions.Fraction

show("f-none", lambda: round(3.7, None))
show("f-noarg", lambda: round(3.7))
show("f-0", lambda: round(3.7, 0))
show("i-none", lambda: round(5, None))
show("i-noarg", lambda: round(5))
show("i-2", lambda: round(5, 2))
show("bool-none", lambda: round(True, None))
show("dec-none", lambda: round(D("3.7"), None))
show("dec-noarg", lambda: round(D("3.7")))
show("dec-1", lambda: round(D("3.14159"), 1))
show("frac-none", lambda: round(F(7, 2), None))
show("frac-noarg", lambda: round(F(7, 2)))
show("frac-1", lambda: round(F(22, 7), 1))
show("f-nan-none", lambda: round(float("nan"), None))
show("f-nan-noarg", lambda: round(float("nan")))
show("f-inf-noarg", lambda: round(float("inf")))
show("f-neg-none", lambda: round(-2.5, None))


class R:
    def __round__(self, n=None):
        return ("round", n)


show("custom-none", lambda: round(R(), None))
show("custom-noarg", lambda: round(R()))
show("custom-2", lambda: round(R(), 2))
