log = []


def mk(tag):
    log.append(("eval", tag))

    def wrap(f):
        log.append(("apply", tag))
        return f
    return wrap


@mk("a")
@mk("b")
@mk("c")
def f():
    return 1


print(log)
print(f())


def replace(g):
    return 42


@replace
def h():
    pass


print(h)


def evalfirst(fn):
    return fn


def side(tag):
    log.append(tag)
    return 0


log.clear()


@evalfirst
def withdefault(x=side("default")):
    return x


print(log, withdefault())


def twice(fn):
    def w(*a, **k):
        return fn(*a, **k) + fn(*a, **k)
    return w


@twice
def ten():
    return 10


print(ten())


def tag(c):
    c.marked = True
    return c


@tag
class C:
    kind = "demo"


print(C.marked, C.kind)


def outer():
    calls = []

    def note(fn):
        calls.append("noted")
        return fn

    @note
    def inner():
        return 7
    return inner(), calls


print(outer())


registry = {}


def register(name):
    def deco(fn):
        registry[name] = fn
        return fn
    return deco


@register("plus")
def plus(a, b):
    return a + b


@register("minus")
def minus(a, b):
    return a - b


print(registry["plus"](2, 3), registry["minus"](5, 1))


def compose():
    order = []

    def a(fn):
        order.append("a")
        return fn

    def b(fn):
        order.append("b")
        return fn

    @a
    @b
    def target():
        pass
    return order


print(compose())
