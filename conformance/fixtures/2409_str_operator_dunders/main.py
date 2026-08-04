# str exposes its arithmetic, hash and string dunders as readable instance
# attributes, the text analog of the surface bytes and the numbers already carry.
# hasattr("", "__add__") answers True, and each bound read routes through the same
# operator the interpreter runs for s + x, s * n and s % x, so the attribute and
# the operator agree on the result and the errors. The repeat slot coerces its
# count through __index__ the way CPython's sequence-repeat does.


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


s = "abc"

# hasattr answers True across the arithmetic, hash and string dunder surface.
names = [
    "__add__",
    "__mul__",
    "__rmul__",
    "__mod__",
    "__rmod__",
    "__repr__",
    "__str__",
    "__hash__",
    "__format__",
    "__getnewargs__",
]
print("has:", [n for n in names if hasattr(s, n)])

# The bound reads run the same operator as +, * and %.
show("add", lambda: "ab".__add__("c"))
show("add wrong type", lambda: "ab".__add__(3))
show("mul", lambda: "ab".__mul__(2))
show("mul zero", lambda: "ab".__mul__(0))
show("mul neg", lambda: "ab".__mul__(-1))
show("mul float", lambda: "ab".__mul__(2.0))
show("rmul", lambda: "ab".__rmul__(2))
show("mod tuple", lambda: "%d-%s".__mod__((5, "x")))
show("mod one", lambda: "%d".__mod__(5))
show("rmod str", lambda: "x".__rmod__("%s"))
show("rmod int", lambda: "x".__rmod__(5))


# The repeat slot honors an operand carrying __index__, and declines the rest.
class Idx:
    def __index__(self):
        return 3


show("mul index", lambda: "ab".__mul__(Idx()))
show("rmul index", lambda: "ab".__rmul__(Idx()))

# hash agrees with the builtin, repr and str round-trip, format applies a spec.
print("hash eq:", "abc".__hash__() == hash("abc"))
show("str", lambda: "ab".__str__())
show("repr", lambda: "a'b".__repr__())
show("format empty", lambda: "ab".__format__(""))
show("format spec", lambda: "ab".__format__(">5"))
show("format bad code", lambda: "ab".__format__("d"))
show("format nonstr", lambda: "ab".__format__(5))
show("getnewargs", lambda: "ab".__getnewargs__())

# The arity errors carry each dunder's own wording.
show("add noargs", lambda: "ab".__add__())
show("add 2args", lambda: "ab".__add__("c", "d"))
show("str extra", lambda: "ab".__str__("x"))
show("format noargs", lambda: "ab".__format__())
show("format 2args", lambda: "ab".__format__("", "x"))
show("getnewargs extra", lambda: "ab".__getnewargs__("x"))
