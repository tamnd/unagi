def outer():
    def inner():
        return 42
    return inner()
print(outer())


def make_adder(n):
    def add(x):
        return x + n
    return add


add5 = make_adder(5)
add100 = make_adder(100)
print(add5(10), add100(10))


def make():
    def fac(n):
        return 1 if n <= 1 else n * fac(n - 1)
    return fac


print(make()(5))


def counter():
    total = 0

    def show():
        return total
    total = total + 3
    return show()


print(counter())


def three_deep():
    x = 1

    def mid():
        def inner():
            return x
        return inner()
    return mid()


print(three_deep())


def with_default():
    def g(a, b=7):
        return a + b
    return g(1), g(1, 2)


print(with_default())


def shadow():
    x = "outer"

    def inner():
        x = "inner"
        return x
    return inner(), x


print(shadow())


g = 0


def bump():
    def do():
        global g
        g = g + 10
    do()
    do()


bump()
print("global g:", g)


def free_unbound():
    def inner():
        return v
    r = inner()
    v = 1
    return r


try:
    free_unbound()
except NameError as e:
    print("free:", e)


def read_before():
    print(h)

    def h():
        pass


try:
    read_before()
except UnboundLocalError as e:
    print("unbound:", e)


def returns_closure():
    seen = []

    def record(x):
        seen.append(x)
        return len(seen)
    return record


r = returns_closure()
print(r("a"), r("b"), r("c"))
