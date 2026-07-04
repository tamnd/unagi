def outer():
    x = 1

    def inner():
        nonlocal x
        x = 99
    inner()
    return x


print(outer())


def counter():
    n = 0

    def inc():
        nonlocal n
        n += 1
        return n
    return inc


c = counter()
d = counter()
print(c(), c(), c(), d())


def shared():
    x = 0

    def get():
        return x

    def put(v):
        nonlocal x
        x = v
    put(42)
    return get()


print(shared())


def multi():
    a, b = 1, 2

    def upd():
        nonlocal a, b
        a, b = a + 10, b + 20
    upd()
    return a, b


print(multi())


def nearest():
    x = "a"

    def b():
        x = "b"

        def c():
            nonlocal x
            x = "c"
        c()
        return x
    return b(), x


print(nearest())


def pass_through():
    x = "outer"

    def b():
        def c():
            nonlocal x
            x = "set"
        c()
    b()
    return x


print(pass_through())


def readback():
    x = 5

    def step():
        nonlocal x
        x = x + 1
        return x
    return step(), step()


print(readback())


def in_loop():
    total = 0

    def add(v):
        nonlocal total
        total = total + v
    for i in range(4):
        add(i)
    return total


print(in_loop())
