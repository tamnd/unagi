# A descriptor whose __set_name__ deletes its own name from the owner during the
# hook still leaves every sibling intact, because the class-creation loop fires
# over a snapshot of the descriptors rather than the live class namespace. This
# is the shape enum member creation runs when each _proto_member.__set_name__
# calls delattr on itself before rebinding the real member.


class Proto:
    def __init__(self, value):
        self.value = value

    def __set_name__(self, owner, name):
        # Remove ourselves, then bind the final value under the same name.
        delattr(owner, name)
        setattr(owner, name, self.value)


class C:
    A = Proto("a")
    B = Proto("b")
    D = Proto("d")
    E = Proto("e")


# All four survive, in definition order, none skipped by a sibling's delete.
print(C.A, C.B, C.D, C.E)
print([n for n in ("A", "B", "D", "E") if hasattr(C, n)])


# Order is independent of how many descriptors there are; an odd count lands too.
class Odd:
    P = Proto(1)
    Q = Proto(2)
    R = Proto(3)


print(Odd.P, Odd.Q, Odd.R)
