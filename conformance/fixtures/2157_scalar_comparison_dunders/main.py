# The builtin scalar types expose their rich-comparison slots as readable,
# callable methods. Each returns NotImplemented for an operand outside its own
# comparison domain, the way the C slot does rather than the operator's
# post-fallback result. Both the direct call (1).__eq__(2) and the bound read
# f = (1).__eq__ resolve, and getattr reaches the same method.


def show(v):
    print(repr(v))


# int and bool compare against int-like operands only.
show((1).__eq__(1))
show((1).__eq__(2))
show((1).__ne__(2))
show((1).__eq__(1.0))
show((1).__eq__(1j))
show((1).__lt__(2))
show((1).__lt__(1.0))
show(True.__eq__(1))

# float admits any real number, but not complex.
show((1.0).__eq__(1))
show((1.0).__eq__(1j))
show((1.5).__lt__(2))

# complex has equality against any number and ordering slots that always
# decline, so (1j).__lt__ is defined yet never orders.
show((1j).__eq__(1j))
show((1 + 0j).__eq__(1))
show((1j).__eq__(1))
show((1j).__eq__("x"))
show((1j).__lt__(1j))

# str compares against str only.
show("a".__eq__("a"))
show("a".__eq__(b"a"))
show("a".__lt__("b"))

# bytes accepts bytes only; bytearray accepts bytes or bytearray.
show(b"a".__eq__(b"a"))
show(b"a".__eq__(bytearray(b"a")))
show(b"a".__lt__(b"b"))
show(bytearray(b"a").__eq__(b"a"))
show(bytearray(b"a").__eq__(bytearray(b"a")))
show(bytearray(b"a").__eq__(1))

# A bound read calls the same, and getattr reaches the slot.
f = (5).__eq__
show(f(5))
show(f(6))
show(getattr("x", "__eq__")("x"))

# The slots are inherited off the type object, so every scalar exposes them.
print(hasattr(1, "__eq__"), hasattr(1.0, "__lt__"), hasattr(1j, "__lt__"))
