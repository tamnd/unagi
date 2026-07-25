# Subclassing bytes: an immutable value subclass whose instances carry a bytes
# payload. zipfile's `class _Extra(bytes)` needs this, defining __new__ that
# calls super().__new__(cls, data) and methods that read the underlying bytes.


class Extra(bytes):
    FIELD = b"xx"

    def __new__(cls, data=b"", tag=0):
        self = super().__new__(cls, data)
        self.tag = tag
        return self

    def head(self):
        return self[:2]


e = Extra(b"hello", tag=7)
print(type(e).__name__)
print(e)
print(e.tag)
print(e.head())
print(e.FIELD)

# It is a bytes: isinstance, length, indexing, iteration, slicing.
print(isinstance(e, bytes))
print(len(e))
print(e[0])
print(list(e))
print(e[1:3])

# Inherited bytes methods read the payload.
print(e.hex())
print(e.upper())
print(e.decode("ascii"))
print(e.startswith(b"he"))
print(e.replace(b"l", b"L"))
print(e.split(b"l"))

# Operators: concatenation, repetition, membership, comparison, hashing.
print(e + b"!")
print(b">" + e)
print(e * 2)
print(b"ell" in e)
print(e == b"hello")
print(e == Extra(b"hello"))
print(hash(e) == hash(b"hello"))

# repr shows the underlying bytes value.
print(repr(e))

# A subclass of the subclass keeps the layout.
class Extra2(Extra):
    pass

e2 = Extra2(b"hi")
print(type(e2).__name__)
print(isinstance(e2, Extra))
print(isinstance(e2, bytes))
print(e2.head())

# bytes() default and from an iterable of ints via the subclass.
print(Extra([104, 105]))
print(bool(Extra(b"")))
print(bool(Extra(b"x")))
