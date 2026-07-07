# Reading __class__ off an instance answers its type through object's getset
# descriptor: it is the class object itself, reports its name, a subclass reads
# its own type, and a class-level property override still wins.

class C:
    pass


class D(C):
    pass


c = C()
d = D()
print(c.__class__)
print(c.__class__.__name__)
print(c.__class__ is C)
print(d.__class__ is D)
print(issubclass(d.__class__, C))


class E:
    @property
    def __class__(self):
        return int


print(E().__class__ is int)
