# bytes and bytearray repeat through the sequence-repeat slot the way str does:
# b.__mul__(n) and b.__rmul__(n) coerce the count through __index__ and raise the
# interpreted-as-an-integer TypeError for a non-index operand, rather than the
# binary * operator's sequence-repeat message. This closes the gap the str repeat
# work surfaced on the binary sequences.


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


class Idx:
    def __index__(self):
        return 3


class Bad:
    pass


for ctor, tag in [(bytes, "bytes"), (bytearray, "bytearray")]:
    b = ctor(b"ab")
    # A plain int repeats, a negative count clamps to empty.
    show(tag + " mul 3", lambda b=b: b.__mul__(3))
    show(tag + " mul 0", lambda b=b: b.__mul__(0))
    show(tag + " mul -1", lambda b=b: b.__mul__(-1))
    show(tag + " rmul 3", lambda b=b: b.__rmul__(3))
    # A bool counts as the int it is.
    show(tag + " mul True", lambda b=b: b.__mul__(True))
    # A float or other non-index operand raises the integer-coercion error.
    show(tag + " mul 2.0", lambda b=b: b.__mul__(2.0))
    show(tag + " mul Bad", lambda b=b: b.__mul__(Bad()))
    # An operand carrying __index__ is coerced and repeats.
    show(tag + " mul Idx", lambda b=b: b.__mul__(Idx()))
    show(tag + " rmul Idx", lambda b=b: b.__rmul__(Idx()))

# The bytearray in-place repeat already coerces the same way; a float still raises.
show("bytearray imul 3", lambda: bytearray(b"ab").__imul__(3))
show("bytearray imul 2.0", lambda: bytearray(b"ab").__imul__(2.0))

# The binary * operator keeps its own sequence-repeat message, unchanged.
show("bytes * 2.0", lambda: b"ab" * 2.0)
