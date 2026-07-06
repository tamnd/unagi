# A metaclass= argument that is not a type is called with (name, bases, ns)
# and the class statement binds whatever it returns; a plain class wins
# determination and goes through its ordinary creation protocol instead of
# the type.__new__/__init__ pairing.

def f(name, bases, ns, **kw):
    keys = [k for k in ns if not k.startswith("__")]
    print("f called:", name, bases, keys, sorted(kw.items()))
    return type(name, bases, ns)

class C(metaclass=f, tag="t"):
    """doc"""
    x = 1

print("built:", type(C).__name__, C.__name__, C.__doc__, C.x)


def g(name, bases, ns):
    return 42

class D(metaclass=g):
    pass

print("non-class result:", D, type(D).__name__)


make_list = lambda name, bases, ns: [name, len([k for k in ns if not k.startswith("__")])]

class F(metaclass=make_list):
    y = 2
    z = 3

print("lambda result:", F)


class WithPrep:
    def __prepare__(self, name, bases, **kw):
        print("inst prepare:", name, sorted(kw.items()))
        return {"seed": 9}

    def __call__(self, name, bases, ns, **kw):
        print("inst call:", name, ns["seed"])
        return type(name, bases, dict(ns))

wp = WithPrep()

class G(metaclass=wp, level=2):
    pass

print("G:", G.seed, type(G).__name__)


class BadPrep:
    def __prepare__(self, name, bases, **kw):
        return 7

    def __call__(self, name, bases, ns, **kw):
        return type(name, bases, dict(ns))

try:
    class H(metaclass=BadPrep()):
        pass
except TypeError as e:
    print("non-mapping:", e)


class Plain:
    def __init__(self, name, bases, ns):
        self.cls_name = name
        self.n_bases = len(bases)

class P(metaclass=Plain):
    pass

print("plain class metaclass:", type(P).__name__, P.cls_name, P.n_bases)


def noargs():
    return 1

try:
    class Bad(metaclass=noargs):
        pass
except TypeError as e:
    print("arity:", e)
