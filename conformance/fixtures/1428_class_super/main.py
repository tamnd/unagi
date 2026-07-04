# super() resolves the next class after the defining one in the instance's own
# MRO, so a chain and a diamond both cooperate, and __init__ threads up the
# hierarchy.
class A:
    def greet(self):
        return "A"


class B(A):
    def greet(self):
        return "B>" + super().greet()


class C(B):
    def greet(self):
        # explicit two-argument super reaches the same place as super()
        return "C>" + super(C, self).greet()


print(A().greet())
print(B().greet())
print(C().greet())


class Base:
    def m(self):
        return ["Base"]


class Left(Base):
    def m(self):
        return ["Left"] + super().m()


class Right(Base):
    def m(self):
        return ["Right"] + super().m()


class Diamond(Left, Right):
    def m(self):
        # super() in Left resolves to Right, not Base, because it walks the
        # Diamond MRO
        return ["Diamond"] + super().m()


print(Diamond().m())


class Point:
    def __init__(self, x):
        self.x = x


class Named(Point):
    def __init__(self, x, name):
        super().__init__(x)
        self.name = name


n = Named(3, "p")
print(n.x, n.name)


# super() has a deterministic repr
class Repr(A):
    def show(self):
        return repr(super())


print(Repr().show())


# a missing name off super raises the super-object AttributeError
class Miss(A):
    def bad(self):
        return super().nope


try:
    Miss().bad()
except AttributeError as e:
    print("attr:", e)


# super() with no method context has nothing to bind
def loose():
    return super()


try:
    loose()
except RuntimeError as e:
    print("noargs:", e)
