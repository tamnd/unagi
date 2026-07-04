g = 10


def clear_global():
    global g
    del g


clear_global()
try:
    print(g)
except NameError as e:
    print("global:", e)


def nonlocal_del():
    x = 1

    def inner():
        nonlocal x
        del x

    inner()
    try:
        print(x)
    except UnboundLocalError:
        print("outer x unbound")
    x = 42
    print("rebound x =", x)


nonlocal_del()


def inner_reads_deleted():
    x = 7

    def inner():
        nonlocal x
        del x
        try:
            print(x)
        except NameError as e:
            print("inner:", e)
        x = 8

    inner()
    print("after inner x =", x)


inner_reads_deleted()


def shadow_stop():
    y = "outer"

    def mid():
        y = "mid"

        def deep():
            nonlocal y
            del y

        deep()
        try:
            print(y)
        except UnboundLocalError:
            print("mid y unbound")

    mid()
    print("outer y still:", y)


shadow_stop()


def three_levels():
    z = 0

    def a():
        def b():
            nonlocal z
            del z

        b()

    a()
    try:
        print(z)
    except UnboundLocalError:
        print("z unbound at top")


three_levels()
