# Float floor division and divmod follow C fmod-based semantics, so an infinite
# dividend yields a nan quotient, a finite dividend over an opposite-signed
# infinity floors to -1.0, and a zero quotient keeps the sign of the true
# quotient. A naive floor(a/b) gets all three wrong.
def show(label, v):
    print(label, "=>", repr(v))

inf = float('inf')
nan = float('nan')

show("inf // 2.0", inf // 2.0)
show("-inf // 2.0", -inf // 2.0)
show("inf // -2.0", inf // -2.0)
show("-inf // -2.0", -inf // -2.0)
show("2.0 // inf", 2.0 // inf)
show("-2.0 // inf", -2.0 // inf)
show("2.0 // -inf", 2.0 // -inf)
show("-2.0 // -inf", -2.0 // -inf)
show("inf // inf", inf // inf)
show("0.0 // inf", 0.0 // inf)
show("0.0 // -3.0", 0.0 // -3.0)
show("-0.0 // 3.0", -0.0 // 3.0)
show("nan // 3.0", nan // 3.0)
show("3.0 // nan", 3.0 // nan)
show("7.0 // 3.0", 7.0 // 3.0)
show("-7.0 // 3.0", -7.0 // 3.0)
show("7.0 // -3.0", 7.0 // -3.0)
show("0.5 // 0.1", 0.5 // 0.1)
show("5 // 2.0", 5 // 2.0)
show("5.0 // 2", 5.0 // 2)

show("divmod(inf, 2.0)", divmod(inf, 2.0))
show("divmod(-inf, 2.0)", divmod(-inf, 2.0))
show("divmod(2.0, inf)", divmod(2.0, inf))
show("divmod(-2.0, inf)", divmod(-2.0, inf))
show("divmod(2.0, -inf)", divmod(2.0, -inf))
show("divmod(7.5, 2.5)", divmod(7.5, 2.5))
show("divmod(-7.5, 2.5)", divmod(-7.5, 2.5))
show("divmod(nan, 2.0)", divmod(nan, 2.0))

show("inf.__floordiv__(2.0)", inf.__floordiv__(2.0))
show("(2.0).__rfloordiv__(inf)", (2.0).__rfloordiv__(inf))
