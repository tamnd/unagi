# A bound builtin method reprs the way CPython's method-wrapper does: the method
# name, the receiver's type and the receiver's address, so [].append reads
# "<built-in method append of list object at 0x...>" rather than the generic
# function form. A classmethod carries its type as the receiver, whose type is
# "type", so int.from_bytes names "of type object". The addresses are scrubbed by
# the harness, so only the structure is compared.
from collections import deque


class D(dict):
    pass


class MyInt(int):
    pass


cases = [
    ("list.append", [].append),
    ("int.bit_length", (5).bit_length),
    ("str.upper", "hi".upper),
    ("dict.get", {}.get),
    ("bytes.hex", b"ab".hex),
    ("bytearray.append", bytearray(b"ab").append),
    ("set.add", {1, 2}.add),
    ("tuple.count", (1, 2).count),
    ("complex.conjugate", (1 + 2j).conjugate),
    ("float.hex", (2.5).hex),
    ("int.from_bytes classmethod", int.from_bytes),
    ("dict.fromkeys classmethod", dict.fromkeys),
    ("deque.append", deque().append),
    ("dict subclass D.get", D().get),
    ("int subclass MyInt.bit_length", MyInt(5).bit_length),
    ("plain builtin len", len),
]
for label, value in cases:
    print(label, repr(value))
