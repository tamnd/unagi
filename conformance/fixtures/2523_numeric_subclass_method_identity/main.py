# A method read off a numeric value subclass instance binds to the instance, so
# __self__ is the instance and __qualname__ names the subclass, while a data
# attribute (real, imag, numerator, denominator) reads straight through as the
# plain value the payload gives.
def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


class MyInt(int):
    pass


class MyFloat(float):
    pass


class MyComplex(complex):
    pass


x = MyInt(12)
show("MyInt.bit_length.__self__ is x", lambda: x.bit_length.__self__ is x)
show("MyInt.bit_length.__qualname__", lambda: x.bit_length.__qualname__)
show("MyInt.bit_length.__name__", lambda: x.bit_length.__name__)
show("MyInt.bit_length()", lambda: x.bit_length())
show("MyInt.to_bytes.__self__ is x", lambda: x.to_bytes.__self__ is x)
show("MyInt.to_bytes.__qualname__", lambda: x.to_bytes.__qualname__)
show("MyInt.to_bytes(2, 'big')", lambda: x.to_bytes(2, "big"))
show("MyInt.as_integer_ratio.__qualname__", lambda: x.as_integer_ratio.__qualname__)
show("MyInt.as_integer_ratio()", lambda: x.as_integer_ratio())
show("MyInt.__getnewargs__.__self__ is x", lambda: x.__getnewargs__.__self__ is x)

# Data attributes read straight through as plain values, not bound methods.
show("x.real", lambda: x.real)
show("type(x.real)", lambda: type(x.real).__name__)
show("x.imag", lambda: x.imag)
show("x.numerator", lambda: x.numerator)
show("x.denominator", lambda: x.denominator)

f = MyFloat(2.5)
show("MyFloat.is_integer.__self__ is f", lambda: f.is_integer.__self__ is f)
show("MyFloat.is_integer.__qualname__", lambda: f.is_integer.__qualname__)
show("MyFloat.hex.__self__ is f", lambda: f.hex.__self__ is f)
show("MyFloat.hex()", lambda: f.hex())
show("f.real", lambda: f.real)

c = MyComplex(1 + 2j)
show("MyComplex.conjugate.__self__ is c", lambda: c.conjugate.__self__ is c)
show("MyComplex.conjugate.__qualname__", lambda: c.conjugate.__qualname__)
show("MyComplex.conjugate()", lambda: c.conjugate())
show("c.real", lambda: c.real)
show("c.imag", lambda: c.imag)
