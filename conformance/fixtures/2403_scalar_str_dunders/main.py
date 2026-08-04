# int, bool and float expose their string dunders (__repr__, __str__,
# __format__) as readable instance attributes, the way complex already does.
# str(x), repr(x) and format(x, spec) still evaluate through their own paths;
# reading a slot only hands back a callable that matches.
def show(f):
    try: print("ok", repr(f()))
    except Exception as e: print(type(e).__name__, e)
for v in [5, True, 1.5]:
    print(type(v).__name__, hasattr(v,"__repr__"), hasattr(v,"__str__"), hasattr(v,"__format__"))
print((5).__repr__(), (5).__str__(), (5).__format__(""), (255).__format__("x"))
print((1.5).__repr__(), (1.5).__str__(), (1.5).__format__(""), (1.5).__format__(".3f"))
print((True).__repr__(), (True).__str__(), (True).__format__("d"))
print((-0.0).__repr__(), (2.5e300).__str__(), (100).__format__(","))
print((10**30).__repr__(), (10**30).__format__(","))
show(lambda: (5).__repr__(1))
show(lambda: (1.5).__str__(1))
show(lambda: (5).__format__())
show(lambda: (1.5).__format__())
show(lambda: (5).__format__(5))
show(lambda: (1.5).__format__(5))
show(lambda: (5).__format__("q"))
show(lambda: (1.5).__format__("d"))
