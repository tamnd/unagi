import math


def show(label, fn):
    try:
        print(label, "->", repr(fn()))
    except Exception as e:
        print(label, "|", type(e).__name__, "|", e)


# CPython's C math functions report a stray keyword under the module-qualified
# name, so the message reads "math.comb() takes no keyword arguments" rather
# than the bare "comb()". This spans the whole no-keyword surface.
cases = [
    ("comb", (10, 3)),
    ("perm", (10, 3)),
    ("gcd", (4, 6)),
    ("lcm", (4, 6)),
    ("factorial", (5,)),
    ("isqrt", (9,)),
    ("floor", (1.5,)),
    ("ceil", (1.5,)),
    ("trunc", (1.5,)),
    ("gamma", (1.5,)),
    ("lgamma", (1.5,)),
    ("hypot", (3, 4)),
    ("dist", ([1], [2])),
    ("fmod", (5, 2)),
    ("remainder", (5, 2)),
    ("ldexp", (1.0, 2)),
    ("copysign", (1, 2)),
    ("atan2", (1, 2)),
    ("pow", (2, 3)),
    ("log", (8, 2)),
    ("log2", (8,)),
    ("log10", (100,)),
    ("sqrt", (4,)),
    ("exp", (1,)),
    ("sin", (0,)),
    ("frexp", (8.0,)),
    ("modf", (3.5,)),
]
for name, args in cases:
    fn = getattr(math, name)
    show(name, lambda fn=fn, args=args: fn(*args, bogus=1))

# __name__ and __qualname__ stay bare even though the error is qualified.
print("comb_name", math.comb.__name__)
print("sqrt_name", math.sqrt.__name__)
print("comb_qualname", math.comb.__qualname__)

# The keyword-aware math functions keep their own bare-name messages, which is
# what CPython does for the keyword-parsing path.
show("prod_badkw", lambda: math.prod([1, 2], bogus=1))
show("nextafter_badkw", lambda: math.nextafter(1.0, 2.0, bogus=1))

# A builtins-module function is left unqualified, no "builtins." prefix.
show("len_kw", lambda: len([1], bogus=1))
show("abs_kw", lambda: abs(1, bogus=1))
