# bytes and bytearray expose their operator, hash and string dunders as readable
# instance attributes, the binary-data analog of the scalar-dunder surface
# numbers carry. Each bound read routes through the same operator the interpreter
# already runs, so the attribute and the operator agree on the result and errors.

b = b"abc"
ba = bytearray(b"abc")


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


# hasattr answers True across the operator, hash and string dunders.
names = [
    "__add__", "__mul__", "__rmul__", "__mod__", "__rmod__",
    "__hash__", "__repr__", "__str__", "__bytes__", "__getnewargs__",
    "__iadd__", "__imul__",
]
print("bytes:", [n for n in names if hasattr(b, n)])
print("barr :", [n for n in names if hasattr(ba, n)])

# bytes operator dunders route through +, * and %.
show("b.__add__", lambda: b.__add__(b"de"))
show("b.__add__int", lambda: b.__add__(5))
show("b.__mul__", lambda: b.__mul__(2))
show("b.__rmul__", lambda: b.__rmul__(2))
show("b.__mod__", lambda: b"%d has %d".__mod__((2, 3)))
show("b.__rmod__bytes", lambda: b"x".__rmod__(b"%s"))
show("b.__rmod__int", lambda: b"x".__rmod__(5))

# bytes hash, string and reconstruction dunders.
show("b.__hash__eq", lambda: b.__hash__() == hash(b))
show("b.__repr__", lambda: b.__repr__())
show("b.__str__", lambda: b.__str__())
show("b.__bytes__", lambda: b.__bytes__())
show("b.__getnewargs__", lambda: b.__getnewargs__())

# The no-argument dunders reject a stray positional, the one-argument ones a
# wrong count, with the method-wrapper wording.
show("b.__repr__arg", lambda: b.__repr__(1))
show("b.__add__noarg", lambda: b.__add__())
show("b.__add__2arg", lambda: b.__add__(b"d", b"e"))

# bytearray carries the same operator and string surface, its __hash__ is None,
# and it has no __bytes__ or __getnewargs__.
show("ba.__add__", lambda: ba.__add__(b"de"))
show("ba.__mul__", lambda: ba.__mul__(2))
show("ba.__mod__", lambda: bytearray(b"%d").__mod__(7))
show("ba.__repr__", lambda: ba.__repr__())
show("ba.__str__", lambda: ba.__str__())
show("ba.__hash__is_none", lambda: ba.__hash__ is None)
show("ba.__hash__call", lambda: ba.__hash__())
show("ba has __bytes__", lambda: hasattr(ba, "__bytes__"))
show("ba has __getnewargs__", lambda: hasattr(ba, "__getnewargs__"))


# __iadd__ and __imul__ mutate a bytearray in place and return the same object.
def iadd_case():
    z = bytearray(b"x")
    r = z.__iadd__(b"y")
    return r is z, bytes(z)


def imul_case():
    z = bytearray(b"ab")
    r = z.__imul__(3)
    return r is z, bytes(z)


show("ba.__iadd__", iadd_case)
show("ba.__imul__", imul_case)
show("ba.__imul__zero", lambda: bytes(bytearray(b"ab").__imul__(0)))
show("ba.__iadd__str", lambda: bytearray(b"x").__iadd__("y"))
show("ba.__imul__str", lambda: bytearray(b"ab").__imul__("x"))
