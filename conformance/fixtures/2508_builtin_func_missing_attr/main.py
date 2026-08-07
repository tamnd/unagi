import math
import os


def show(label, fn):
    try:
        fn()
        print(label, "=> NO ERROR")
    except AttributeError as e:
        print(label, "=>", e)
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A plain builtin function and a bound builtin method both type as
# builtin_function_or_method, so a missing attribute names that type rather than
# the internal "function". A type constructor stays a type object, and a real
# Python function keeps "function".
print("== a builtin function types as builtin_function_or_method ==")
print("type(len).__name__:", type(len).__name__)
print("type(math.sqrt).__name__:", type(math.sqrt).__name__)
print("type([].append).__name__:", type([].append).__name__)
print("type((5).bit_length).__name__:", type((5).bit_length).__name__)

print("== a missing attribute names builtin_function_or_method ==")
show("len.nope", lambda: len.nope)
show("print.nope", lambda: print.nope)
show("abs.nope", lambda: abs.nope)
show("iter.nope", lambda: iter.nope)
show("repr.nope", lambda: repr.nope)
show("sorted.nope", lambda: sorted.nope)
show("math.sqrt.nope", lambda: math.sqrt.nope)
show("os.getpid.nope", lambda: os.getpid.nope)

print("== a bound builtin method names builtin_function_or_method ==")
show("[].append.nope", lambda: [].append.nope)
show("'a'.upper.nope", lambda: "a".upper.nope)
show("{}.get.nope", lambda: {}.get.nope)
show("(5).bit_length.nope", lambda: (5).bit_length.nope)
show("dict.fromkeys.nope", lambda: dict.fromkeys.nope)

print("== a type constructor stays a type object ==")
show("int.nope", lambda: int.nope)
show("str.nope", lambda: str.nope)

print("== a real Python function keeps function ==")


def myfunc():
    pass


print("type(myfunc).__name__:", type(myfunc).__name__)
show("myfunc.nope", lambda: myfunc.nope)

print("== a resolving read on a builtin function is unaffected ==")
print("len.__name__:", len.__name__)
print("math.sqrt.__name__:", math.sqrt.__name__)
print("[].append.__name__:", [].append.__name__)
print("callable(len):", callable(len))
