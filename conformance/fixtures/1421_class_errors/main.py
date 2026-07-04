# The class and instance error catalog, each message caught and printed.
class C:
    def __init__(self, a):
        self.a = a

    def m(self, d):
        return d


class E:
    pass


try:
    C()
except TypeError as e:
    print(e)

try:
    C(1, 2)
except TypeError as e:
    print(e)

try:
    E(1)
except TypeError as e:
    print(e)

c = C(1)
try:
    c.m()
except TypeError as e:
    print(e)

try:
    c.m(1, 2)
except TypeError as e:
    print(e)

try:
    c()
except TypeError as e:
    print(e)

try:
    c.missing
except AttributeError as e:
    print(e)

try:
    C.nope
except AttributeError as e:
    print(e)


class R:
    def __init__(self):
        return 5


try:
    R()
except TypeError as e:
    print(e)

try:
    x = 5
    x.foo = 1
except AttributeError as e:
    print(e)
