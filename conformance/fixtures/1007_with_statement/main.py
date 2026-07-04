# The with statement: enter and exit edges, the as-target, suppression, and
# jumps that leave the body all run through the same closure a try uses.
class CM:
    def __init__(self, name):
        self.name = name

    def __enter__(self):
        print("enter", self.name)
        return self.name

    def __exit__(self, et, ev, tb):
        print("exit", self.name, et)
        return False


with CM("a") as x:
    print("body", x)

# Managers enter left to right and exit in reverse.
with CM("outer") as o, CM("inner") as i:
    print("body", o, i)

# A tuple as-target unpacks the __enter__ result.
class Pair:
    def __enter__(self):
        return (1, 2)

    def __exit__(self, *a):
        return False


with Pair() as (a, b):
    print("pair", a, b)


# A truthy __exit__ return on the exception path swallows the exception.
class Swallow:
    def __enter__(self):
        return self

    def __exit__(self, et, ev, tb):
        print("swallow", et)
        return True


with Swallow():
    raise ValueError("boom")
print("survived")


# The error catalog: a manager missing either half of the protocol.
class NoExit:
    def __enter__(self):
        return self


try:
    with NoExit():
        print("unreachable")
except TypeError as e:
    print(e)


class NoEnter:
    def __exit__(self, *a):
        return False


try:
    with NoEnter():
        print("unreachable")
except TypeError as e:
    print(e)


# A return, break, and continue inside a with body still run __exit__ first.
def f():
    with CM("f"):
        for n in range(3):
            with CM("loop"):
                if n == 1:
                    return "early"
                print("n", n)
    return "done"


print(f())


def g():
    total = 0
    for n in range(4):
        with CM("g"):
            if n == 2:
                break
            if n == 0:
                continue
            total += n
    return total


print(g())


# __exit__ that raises on a clean exit replaces the outcome; caught here.
class ExitRaises:
    def __enter__(self):
        return self

    def __exit__(self, *a):
        raise RuntimeError("from exit")


try:
    with ExitRaises():
        print("clean body")
except RuntimeError as e:
    print("caught", e)
