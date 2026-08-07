# The six rich-comparison dunders read off a scalar or binary type are unbound
# descriptors that run that type's own comparison with the receiver passed first,
# so int.__lt__(1, 2) orders and str.__eq__('a', 'a') answers. Read off the type
# they must not fall through to object's slot, which only returns NotImplemented.
from fractions import Fraction
from decimal import Decimal

def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)

types = {
    'int': (int, 1, 2),
    'float': (float, 1.5, 2.5),
    'str': (str, 'a', 'b'),
    'bytes': (bytes, b'a', b'b'),
    'bytearray': (bytearray, bytearray(b'a'), bytearray(b'b')),
    'complex': (complex, 1j, 2j),
    'bool': (bool, False, True),
}
for tname, (T, a, b) in types.items():
    for op in ['__lt__', '__le__', '__gt__', '__ge__', '__eq__', '__ne__']:
        m = getattr(T, op)
        show(f"{tname}.{op}(a, b)", lambda m=m, a=a, b=b: m(a, b))
        show(f"{tname}.{op}(a, a)", lambda m=m, a=a: m(a, a))

# Cross-type comparison domains: each type declines out-of-domain operands with
# NotImplemented rather than raising or coercing.
show("int.__lt__(1, 2.0)", lambda: int.__lt__(1, 2.0))
show("int.__eq__(1, 1.0)", lambda: int.__eq__(1, 1.0))
show("float.__eq__(1.5, Fraction(3, 2))", lambda: float.__eq__(1.5, Fraction(3, 2)))
show("float.__eq__(0.5, Decimal('0.5'))", lambda: float.__eq__(0.5, Decimal('0.5')))
show("int.__eq__(1, Fraction(1))", lambda: int.__eq__(1, Fraction(1)))
show("bytes.__eq__(b'a', bytearray(b'a'))", lambda: bytes.__eq__(b'a', bytearray(b'a')))
show("bytearray.__eq__(bytearray(b'a'), b'a')", lambda: bytearray.__eq__(bytearray(b'a'), b'a'))
show("str.__eq__('a', 97)", lambda: str.__eq__('a', 97))
show("complex.__eq__(2+0j, Fraction(2))", lambda: complex.__eq__(2 + 0j, Fraction(2)))
show("int.__gt__(True, 0)", lambda: int.__gt__(True, 0))

# A value subclass receiver runs the builtin's own comparison off its payload.
class MyInt(int):
    pass

class MyStr(str):
    pass

show("int.__lt__(MyInt(1), 2)", lambda: int.__lt__(MyInt(1), 2))
show("str.__lt__(MyStr('a'), 'b')", lambda: str.__lt__(MyStr('a'), 'b'))

# Argument and receiver-type errors match the wrapper_descriptor.
show("int.__lt__()", lambda: int.__lt__())
show("int.__lt__(1)", lambda: int.__lt__(1))
show("int.__lt__(1, 2, 3)", lambda: int.__lt__(1, 2, 3))
show("int.__lt__('x', 1)", lambda: int.__lt__('x', 1))
show("float.__lt__(1, 2)", lambda: float.__lt__(1, 2))
