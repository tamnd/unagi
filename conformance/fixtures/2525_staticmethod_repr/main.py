# A staticmethod bound to a type carries a None receiver but names its owning
# type in its repr, so str.maketrans reads "<built-in method maketrans of type
# object at 0x...>" the way CPython's does. The address is scrubbed by the
# harness, so only the structure is compared. Its __self__ stays None.
cases = [
    ("str.maketrans", str.maketrans),
    ("bytes.maketrans", bytes.maketrans),
    ("bytearray.maketrans", bytearray.maketrans),
]
for label, value in cases:
    print(label, repr(value))
    print(label, "__self__", value.__self__)
    print(label, "__qualname__", value.__qualname__)
    print(label, "__name__", value.__name__)
