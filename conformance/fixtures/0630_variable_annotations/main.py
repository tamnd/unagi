x: int = 5
print("module valued", x)

label: str
print("bare annotation is a no-op")

late: int
late = 7
print("bare then assigned", late)

z: Undefined = 42
print("annotation deferred", z)

w: int = 1, 2
print("tuple rhs", w)

class Point:
    count: int = 0
    tag: str

    def __init__(self, a, b):
        self.a: int = a
        self.b: int = b

p = Point(3, 4)
print("attr annotated", p.a, p.b, Point.count)
try:
    print(Point.tag)
except AttributeError:
    print("class bare not bound")

d = {}
d["k"]: int = 99
print("subscript annotated", d)

def f():
    n: int = 10
    m: Whatever
    m = 20
    return n + m
print("func locals", f())

def g():
    obj = {}
    obj["x"]: int
    print("bare subscript no store", obj)
g()
