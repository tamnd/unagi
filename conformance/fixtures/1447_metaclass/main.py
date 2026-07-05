# A metaclass drives class creation through __new__ and __init__, sees the class
# keywords, and becomes the type() of the classes it builds.
class Meta(type):
    def __new__(mcs, name, bases, ns, **kw):
        print("new", mcs.__name__, name, sorted(kw.items()))
        cls = super().__new__(mcs, name, bases, ns)
        cls.tag = kw.get("tag", 0)
        return cls
    def __init__(cls, name, bases, ns, **kw):
        print("init", cls.__name__)


class C(metaclass=Meta, tag=5):
    x = 1
    def m(self):
        return self.x


print("type(C)=", type(C).__name__)
print("type(C())=", type(C()).__name__)
print("C.tag=", C.tag, "C.x=", C.x)
print("isinstance(C, Meta)=", isinstance(C, Meta))
print("isinstance(C, type)=", isinstance(C, type))
print("c.m()=", C().m())

# A subclass inherits the metaclass without naming it.
class Sub(C):
    pass


print("type(Sub)=", type(Sub).__name__, "Sub.tag=", Sub.tag)

# A metaclass with only __init__ initializes the class in place.
class InitMeta(type):
    def __init__(cls, name, bases, ns):
        cls.stamped = True


class D(metaclass=InitMeta):
    pass


print("type(D)=", type(D).__name__, "D.stamped=", D.stamped)

# Most-derived determination: Deep subclasses Meta, so it wins over Meta.
class Deep(Meta):
    pass


class E(C, metaclass=Deep):
    pass


print("type(E)=", type(E).__name__)

# An explicit metaclass=type keeps the default metatype.
class F(metaclass=type):
    pass


print("type(F)=", type(F).__name__)

# Metaclass conflicts raise at class-creation time.
class M1(type):
    pass


class M2(type):
    pass


class G(metaclass=M1):
    pass


class H(metaclass=M2):
    pass


try:
    class Clash(G, H):
        pass
except TypeError as e:
    print("conflict:", e)
