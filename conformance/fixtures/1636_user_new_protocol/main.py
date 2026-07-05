# The __new__/__init__ creation protocol: type.__call__ allocates through
# __new__ then initializes through __init__ only when __new__ returned an
# instance of the class.


class Point:
    def __new__(cls, *args, **kw):
        print("Point.__new__", args, kw)
        return super().__new__(cls)

    def __init__(self, x, y):
        print("Point.__init__", x, y)
        self.x = x
        self.y = y


p = Point(3, 4)
print("point", p.x, p.y, type(p).__name__)


# A __new__ that returns another type skips __init__ entirely.
class Sneaky:
    def __new__(cls, *args):
        print("Sneaky.__new__ ->", args)
        return 99

    def __init__(self, *args):
        print("Sneaky.__init__ must not run")


s = Sneaky(1, 2)
print("sneaky", s, type(s).__name__)


# An exception subclass allocates through BaseException.__new__ and stays
# raisable and catchable, its args seeded through super().__init__.
class MyErr(Exception):
    def __new__(cls, *args):
        print("MyErr.__new__", args)
        return super().__new__(cls, *args)

    def __init__(self, *args):
        print("MyErr.__init__", args)
        super().__init__(*args)


try:
    raise MyErr("boom", 7)
except MyErr as e:
    print("caught", e.args, type(e).__name__)


# A @staticmethod __new__ with an explicit super(Cls, cls) chain.
class Tagged:
    @staticmethod
    def __new__(cls, *args):
        print("Tagged.__new__", args)
        obj = super(Tagged, cls).__new__(cls)
        obj.tag = "T"
        return obj

    def __init__(self, *args):
        print("Tagged.__init__", args)


t = Tagged(1)
print("tagged", t.tag)


# __new__ overridden with no __init__: the object root ignores extra arguments.
class Loose:
    def __new__(cls, *args, **kw):
        print("Loose.__new__", args, kw)
        return super().__new__(cls)


loose = Loose(1, 2, z=3)
print("loose", type(loose).__name__)


# Keywords thread through both __new__ and __init__.
class Config:
    def __new__(cls, **kw):
        print("Config.__new__", kw)
        return super().__new__(cls)

    def __init__(self, **kw):
        print("Config.__init__", kw)
        self.settings = kw


c = Config(debug=True, level=5)
print("config", c.settings)
