def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A numeric operator or special dunder read off an int instance, (5).__add__, is
# a bound method, so it carries the receiver through __self__ the way CPython's
# method-wrapper does. bool shares int's slots, so True.__and__ binds True. The
# reflected, unary and special slots all bind the same receiver, and __name__
# stays the bare slot name. The unbound form int.__add__ builds its own callable
# and stays self-less.
print("== a bound binary dunder binds its number ==")
n = 5
print("(5).__add__.__self__:", n.__add__.__self__, "| is n:", n.__add__.__self__ is n)
print("(5).__radd__.__self__:", (5).__radd__.__self__)
print("(5).__sub__.__self__:", (5).__sub__.__self__)
print("(5).__mul__.__self__:", (5).__mul__.__self__)
print("(5).__floordiv__.__self__:", (5).__floordiv__.__self__)
print("(5).__and__.__self__:", (5).__and__.__self__)
print("(5).__lshift__.__self__:", (5).__lshift__.__self__)
print("(5).__pow__.__self__:", (5).__pow__.__self__)
print("(5).__rpow__.__self__:", (5).__rpow__.__self__)
print("True.__and__.__self__:", True.__and__.__self__)
print("True.__or__.__self__:", True.__or__.__self__)

print("== a bound unary or special dunder binds its number ==")
print("(5).__neg__.__self__:", (5).__neg__.__self__)
print("(5).__pos__.__self__:", (5).__pos__.__self__)
print("(5).__invert__.__self__:", (5).__invert__.__self__)
print("(5).__abs__.__self__:", (5).__abs__.__self__)
print("(5).__bool__.__self__:", (5).__bool__.__self__)
print("(5).__hash__.__self__:", (5).__hash__.__self__)
print("(5).__divmod__.__self__:", (5).__divmod__.__self__)
print("(5).__rdivmod__.__self__:", (5).__rdivmod__.__self__)
print("(5).__round__.__self__:", (5).__round__.__self__)
print("(5).__floor__.__self__:", (5).__floor__.__self__)
print("(5).__getnewargs__.__self__:", (5).__getnewargs__.__self__)

print("== __name__ stays the bare slot name ==")
print("(5).__add__.__name__:", (5).__add__.__name__)
print("(5).__neg__.__name__:", (5).__neg__.__name__)
print("(5).__divmod__.__name__:", (5).__divmod__.__name__)

print("== a bound dunder still computes ==")
print("(5).__add__(3):", (5).__add__(3))
print("(5).__neg__():", (5).__neg__())
print("(5).__divmod__(2):", (5).__divmod__(2))
print("(5).__pow__(3, 7):", (5).__pow__(3, 7))
print("True.__and__(False):", True.__and__(False))
