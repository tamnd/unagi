class Point:
    __match_args__ = ("x", "y")

    def __init__(self, x, y):
        self.x = x
        self.y = y


class Point3(Point):
    __match_args__ = ("x", "y", "z")

    def __init__(self, x, y, z):
        super().__init__(x, y)
        self.z = z


class Empty:
    pass


def classify(p):
    match p:
        case Point(0, 0):
            return "origin"
        case Point(x, 0):
            return f"on x-axis at {x}"
        case Point(0, y):
            return f"on y-axis at {y}"
        case Point(x, y):
            return f"point {x},{y}"
        case _:
            return "not a point"


print(classify(Point(0, 0)))
print(classify(Point(5, 0)))
print(classify(Point(0, 7)))
print(classify(Point(3, 4)))
print(classify(42))


# Keyword sub-patterns, in any order, and mixed with positional.
def named(p):
    match p:
        case Point(y=0, x=0):
            return "kw origin"
        case Point(x, y=b):
            return f"mixed {x} {b}"
        case _:
            return "no"


print(named(Point(0, 0)))
print(named(Point(9, 8)))


# Inheritance: a subclass instance matches the base class pattern, and the
# subclass pattern reads its own extra __match_args__ slot.
def depth(p):
    match p:
        case Point3(a, b, c):
            return f"3d {a} {b} {c}"
        case Point(a, b):
            return f"2d {a} {b}"
        case _:
            return "flat"


print(depth(Point3(1, 2, 3)))
print(depth(Point(4, 5)))


# Nested class patterns and captures inside them.
class Line:
    __match_args__ = ("start", "end")

    def __init__(self, start, end):
        self.start = start
        self.end = end


def describe(line):
    match line:
        case Line(Point(0, 0), Point(x, y)):
            return f"from origin to {x},{y}"
        case Line(Point(a, b), end):
            return f"from {a},{b} to {end.x},{end.y}"
        case _:
            return "?"


print(describe(Line(Point(0, 0), Point(2, 3))))
print(describe(Line(Point(1, 1), Point(2, 2))))


# A guard runs after the captures bind.
def graded(p):
    match p:
        case Point(x, y) if x == y:
            return "diagonal"
        case Point(x, y):
            return "off-diagonal"
    return "no"


print(graded(Point(3, 3)))
print(graded(Point(3, 4)))


# An or-pattern with a class alternative, and an as-binding over one.
def kind(p):
    match p:
        case Point(0, 0) | Empty():
            return "empty-ish"
        case Point() as whole:
            return f"any point {whole.x}"
        case _:
            return "other"


print(kind(Point(0, 0)))
print(kind(Empty()))
print(kind(Point(6, 1)))
print(kind(7))


# A class with no __match_args__ still matches on zero positional or by keyword.
class Box:
    def __init__(self, v):
        self.v = v


def boxed(b):
    match b:
        case Box(v=n):
            return f"box {n}"
        case Box():
            return "empty box"
        case _:
            return "not a box"


print(boxed(Box(10)))


# A missing attribute makes the case fall through rather than raise.
class Opt:
    __match_args__ = ("here",)


def optional(o):
    match o:
        case Opt(here=v):
            return f"has {v}"
        case Opt():
            return "no attr"
    return "no"


print(optional(Opt()))
o = Opt()
o.here = 99
print(optional(o))


# Error paths, each caught so the fixture stays a clean exit.
def err(label, fn):
    try:
        fn()
        print(label, "-> no error")
    except Exception as e:
        print(label + ":", e)


def bad_type():
    not_a_class = 5
    match Point(1, 2):
        case not_a_class():
            pass


err("nonclass", bad_type)


def too_many():
    match Empty():
        case Empty(a):
            pass


err("toomany", too_many)


class BadTuple:
    __match_args__ = ["x"]
    x = 1


def non_tuple():
    match BadTuple():
        case BadTuple(a):
            pass


err("nontuple", non_tuple)


class BadElem:
    __match_args__ = (1,)
    x = 1


def non_string():
    match BadElem():
        case BadElem(a):
            pass


err("nonstring", non_string)


def dup_attr():
    match Point(1, 2):
        case Point(a, x=b):
            pass


err("dupattr", dup_attr)
