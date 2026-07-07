# A comprehension in a class body runs in an implicit function scope that skips
# the class namespace. Only the leftmost iterable is evaluated in the class
# scope, so it alone may read a class variable; an inner iterable, a condition,
# or the element expression cannot, and a class-only name there raises
# NameError when the comprehension runs. A module global stays visible
# throughout, since the implicit scope still nests in the module.

base = 100


class C:
    xs = [1, 2, 3]
    ks = [10, 20]
    mul = 2

    # The leftmost iterable reads the class variable xs.
    doubled = [x * 2 for x in xs]

    # The element reads the module global base, not a class variable.
    shifted = [x + base for x in xs]

    # A class variable named in an inner iterable is invisible: the read
    # raises NameError when the comprehension runs, which the body catches.
    inner = "leftmost-only"
    try:
        cross = [(x, k) for x in xs for k in ks]
        inner = "unexpected"
    except NameError:
        inner = "inner-iterable-skips-class"

    # A class variable named in the element is invisible the same way.
    elt = "leftmost-only"
    try:
        bad = [x * mul for x in xs]
        elt = "unexpected"
    except NameError:
        elt = "element-skips-class"


print(C.doubled)
print(C.shifted)
print(C.inner)
print(C.elt)
