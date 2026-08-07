import math
def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)
show("fact(0)", lambda: math.factorial(0))
show("fact(1)", lambda: math.factorial(1))
show("fact(5)", lambda: math.factorial(5))
show("fact(20)", lambda: math.factorial(20))
show("fact(-1)", lambda: math.factorial(-1))
show("fact(10**19)", lambda: math.factorial(10**19))
show("fact(10**100)", lambda: math.factorial(10**100))
show("fact(5.0)", lambda: math.factorial(5.0))
show("fact(5.2)", lambda: math.factorial(5.2))
show("fact(1e100)", lambda: math.factorial(1e100))
show("fact(True)", lambda: math.factorial(True))
show("fact(9223372036854775808)", lambda: math.factorial(9223372036854775808))
