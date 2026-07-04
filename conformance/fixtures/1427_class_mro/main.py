# The C3 linearization decides which method a diamond resolves to and lets a
# subclass override a name every branch defines. Method resolution is the
# observable face of the MRO.
class Base:
    def tag(self):
        return "base"

    def chain(self):
        return "base"


class Left(Base):
    def tag(self):
        return "left"

    def chain(self):
        return "left>" + Base.chain(self)


class Right(Base):
    def tag(self):
        return "right"

    def chain(self):
        return "right>" + Base.chain(self)


class Diamond(Left, Right):
    def chain(self):
        return "diamond>" + Left.chain(self) + "|" + Right.chain(self)


dm = Diamond()
# C3 order is Diamond, Left, Right, Base, so tag resolves to Left.
print(dm.tag())
print(dm.chain())

# A three-level line resolves through each level in turn.
class G1:
    def g(self):
        return "g1"


class G2(G1):
    pass


class G3(G2):
    def g(self):
        return "g3"


print(G2().g())
print(G3().g())

# An inconsistent base order is rejected when the class is created.
class A:
    pass


class B(A):
    pass


try:
    class Bad(A, B):
        pass
except TypeError as e:
    print("mro:", e)

# A repeated base is rejected too.
try:
    class Dup(A, A):
        pass
except TypeError as e:
    print("dup:", e)
