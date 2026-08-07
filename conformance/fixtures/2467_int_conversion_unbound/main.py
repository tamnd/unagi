# int and bool carry their conversion dunders (__int__, __float__, __index__,
# __trunc__) off the type as unbound descriptors, so int.__index__(7) reads back a
# callable and runs int's own conversion the way (7).__index__() does. These four
# names had fallen through to an AttributeError off the type while float and the
# binary types already exposed them. __complex__ and __bytes__ stay absent because
# int does not carry them, matching CPython's per-type slot table.
def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)

for t in (int, bool):
    for n in ('__int__', '__float__', '__index__', '__trunc__',
              '__round__', '__bool__', '__getnewargs__',
              '__complex__', '__bytes__'):
        print(t.__name__, n, hasattr(t, n))

# The four conversion descriptors run int's own conversion with the receiver first.
show("int.__index__(7)", lambda: int.__index__(7))
show("int.__int__(9)", lambda: int.__int__(9))
show("int.__float__(5)", lambda: int.__float__(5))
show("int.__trunc__(4)", lambda: int.__trunc__(4))
show("int.__float__(-3)", lambda: int.__float__(-3))
show("int.__float__(2 ** 70)", lambda: int.__float__(2 ** 70))
show("int.__index__(True)", lambda: int.__index__(True))

# bool inherits int's slot unchanged, so bool.__index__ is int.__index__: it names
# 'int' and accepts any int, not just a bool.
show("bool.__index__(True)", lambda: bool.__index__(True))
show("bool.__int__(5)", lambda: bool.__int__(5))
show("bool.__float__(False)", lambda: bool.__float__(False))

# A value subclass receiver runs int's own conversion off its payload, ignoring an
# override.
class MyInt(int):
    def __index__(self):
        return 999

show("int.__index__(MyInt(5))", lambda: int.__index__(MyInt(5)))
show("int.__float__(MyInt(5))", lambda: int.__float__(MyInt(5)))

# The descriptor's own errors: a missing receiver, a wrong-typed one, an extra
# argument, and a float that is not an integer.
show("int.__index__()", lambda: int.__index__())
show("int.__index__('x')", lambda: int.__index__('x'))
show("int.__index__(1.0)", lambda: int.__index__(1.0))
show("int.__index__(7, 8)", lambda: int.__index__(7, 8))
show("bool.__index__('x')", lambda: bool.__index__('x'))
