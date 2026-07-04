class Mgr:
    def __init__(self, name, value):
        self.name = name
        self.value = value

    def __enter__(self):
        print("enter", self.name)
        return self.value

    def __exit__(self, exc_type, exc, tb):
        print("exit", self.name)
        return False


# Parenthesized items with as targets, exits run in reverse order.
with (Mgr("a", 1) as a, Mgr("b", 2) as b):
    print("body", a, b)

# Parenthesized managers without targets.
with (Mgr("c", 3), Mgr("d", 4)):
    print("body2")

# Line spanning list with a trailing comma.
with (
    Mgr("e", 5) as e,
    Mgr("f", 6) as f,
):
    print("body3", e + f)

# A single parenthesized item is one manager.
with (Mgr("solo", 7) as s):
    print("solo", s)

# One item with a trailing comma is still one manager.
with (Mgr("t", 8) as t,):
    print("trail", t)

# A grouped expression around one manager keeps working.
with (Mgr("g", 9)) as g:
    print("group", g)

# The unparenthesized comma form is unchanged.
with Mgr("h", 10) as h, Mgr("i", 11) as i:
    print("plain", h, i)


# An exception inside a parenthesized block still unwinds every manager.
class Quiet:
    def __enter__(self):
        print("enter quiet")
        return self

    def __exit__(self, exc_type, exc, tb):
        print("exit quiet", exc_type is not None)
        return True


with (Quiet() as q, Mgr("j", 12) as j):
    print("before raise", j)
    raise ValueError("stop")
print("after block")
