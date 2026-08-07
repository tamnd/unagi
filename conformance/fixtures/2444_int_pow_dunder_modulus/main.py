def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


# int.__pow__ and int.__rpow__ take the optional modulus the three-argument pow
# protocol threads through the slot, so (3).__pow__(4, 5) is 3**4 % 5 and the
# reflected slot computes the base raised to the receiver. A None modulus, or none
# at all, is the plain power, and bool shares int's slot.
show("pow-mod", lambda: (3).__pow__(4, 5))
show("pow-one", lambda: (3).__pow__(4))
show("pow-mod-none", lambda: (3).__pow__(4, None))
show("rpow-mod", lambda: (4).__rpow__(3, 5))
show("rpow-one", lambda: (4).__rpow__(3))
show("bool-pow-mod", lambda: True.__pow__(4, 5))
show("bool-rpow", lambda: True.__rpow__(3, 5))

# A negative exponent with a modulus goes through the modular inverse, and a
# negative modulus shifts the result into that sign the way the builtin pow does.
show("pow-neg-mod", lambda: (3).__pow__(-1, 7))
show("pow-neg-modulus", lambda: (3).__pow__(4, -5))
show("pow-noninvertible", lambda: (2).__pow__(-1, 4))

# A negative exponent with no modulus is the float power the plain slot gives.
show("pow-neg-nomod", lambda: (2).__pow__(-1))
show("pow-big", lambda: (2).__pow__(100))

# A non-int exponent or a non-int modulus declines with NotImplemented so a mixed
# pair hands off to the other operand, and a zero modulus is the pow() ValueError.
show("pow-float-exp", lambda: (3).__pow__(4.0))
show("pow-float-mod", lambda: (3).__pow__(4, 5.0))
show("rpow-float", lambda: (3).__rpow__(4.0))
show("pow-mod-zero", lambda: (3).__pow__(4, 0))

# The wrong argument count is the one-or-two-arguments TypeError.
show("pow-zero-args", lambda: (3).__pow__())
show("pow-three-args", lambda: (3).__pow__(1, 2, 3))

# The unbound slot off the type takes the receiver first, so int.__pow__(3, 4, 5)
# matches the bound three-argument form.
show("unbound-pow-mod", lambda: int.__pow__(3, 4, 5))
show("unbound-pow-one", lambda: int.__pow__(3, 4))
show("unbound-bad-recv", lambda: int.__pow__("x", 4))
