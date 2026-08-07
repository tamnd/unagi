# pow(base, exp, None) is two-argument pow: an explicit None modulus means no
# modulus, so it matches pow(base, exp) for every type instead of raising about
# a NoneType third argument, while a real integer modulus keeps modular pow.
def show(label, fn):
    try:
        r = fn()
        print(label, type(r).__name__, repr(r))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


show("ii-none", lambda: pow(2, 3, None))
show("ii-noarg", lambda: pow(2, 3))
show("ii-mod", lambda: pow(2, 3, 5))
show("if-none", lambda: pow(2, 3.0, None))
show("if-noarg", lambda: pow(2, 3.0))
show("ff-none", lambda: pow(2.0, 3.0, None))
show("neg-exp-none", lambda: pow(2, -1, None))
show("bool-none", lambda: pow(True, True, None))
show("dunder-pow-none", lambda: (2).__pow__(3, None))
show("float-pow-none", lambda: (2.0).__pow__(3.0, None))
show("mod-with-float", lambda: pow(2, 3, 5.0))
show("big-none", lambda: pow(10, 20, None))
show("complex-none", lambda: pow(1 + 1j, 2, None))
show("int-mod-zero", lambda: pow(2, 3, 0))
show("neg-exp-mod", lambda: pow(3, -1, 7))


class P:
    def __pow__(self, e, m=None):
        return ("pow", e, m)


show("custom-none", lambda: pow(P(), 2, None))
show("custom-noarg", lambda: pow(P(), 2))
show("custom-mod", lambda: pow(P(), 2, 9))
