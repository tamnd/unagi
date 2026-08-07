def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# An int, str or complex subclass with no __new__ or __init__ of its own inherits
# the base constructor, keyword form included, so the keyword arguments build the
# immutable payload the way int(...), str(...) and complex(...) do at the top
# level. int takes base by name, str takes object, encoding and errors, and
# complex takes real and imag.
class MI(int):
    pass


class MS(str):
    pass


class MC(complex):
    pass


show("int-base-kw", lambda: MI("10", base=2))
show("int-value-and-base-kw", lambda: MI("ff", base=16))
show("int-base-positional", lambda: MI("10", 2))
show("int-one-positional", lambda: MI("12"))
show("int-empty", lambda: MI())
show("str-object-kw", lambda: MS(object=5))
show("str-encoding-kw", lambda: MS(b"hi", encoding="utf-8"))
show("str-encoding-errors-kw", lambda: MS(b"\xff", encoding="ascii", errors="replace"))
show("str-object-only", lambda: MS(42))
show("str-empty", lambda: MS())
show("complex-real-imag-kw", lambda: MC(real=1, imag=2))
show("complex-imag-only", lambda: MC(imag=3))
show("complex-from-string", lambda: MC("1+2j"))

# The payload is a real base value, so it compares equal to the plain value and
# still reports the subclass type and its own repr.
mi = MI("10", base=2)
print("int-value", mi == 2, type(mi).__name__, isinstance(mi, int), repr(mi))
ms = MS(object=b"hi", encoding="utf-8")
print("str-value", ms == "hi", type(ms).__name__, isinstance(ms, str), repr(ms))
mc = MC(real=1, imag=2)
print("complex-value", mc == (1 + 2j), type(mc).__name__, isinstance(mc, complex), repr(mc))

# The base constructor validates the keywords itself. int takes only base by
# name, the value being positional-only, so a value keyword is the unexpected
# argument and base with no value is the missing-string error. str reports a
# name-and-position clash and an unknown keyword, and complex reports its own
# unknown keyword. The total-argument count check outranks a clash.
show("int-value-kw", lambda: MI(x=5))
show("int-unknown-kw", lambda: MI("10", foo=2))
show("int-base-only", lambda: MI(base=2))
show("int-non-string-base", lambda: MI(5, base=2))
show("int-count", lambda: MI("10", 2, base=3))
show("str-dup-object", lambda: MS(b"hi", object=b"yo"))
show("str-unknown-kw", lambda: MS(b"hi", foo=1))
show("str-count", lambda: MS(b"", "ascii", "replace", errors="ignore"))
show("complex-unknown-kw", lambda: MC(x=1))
show("complex-dup-real", lambda: MC(1, real=2))
show("complex-count", lambda: MC(1, 2, 3))


# A subclass with a custom __init__ still runs the value through the base
# __new__, which sees the same keywords, so a keyword the base rejects fails
# there even though __init__ would accept it.
class MIinit(int):
    def __init__(self, *args, **kwargs):
        self.saved = kwargs.get("tag")


show("init-extra-kw", lambda: MIinit("10", base=2, tag=1))
show("init-base-kw", lambda: MIinit("10", base=2))
