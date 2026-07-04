class Base:
    kind = "base"

    def greet(self):
        return "hi"


class Point(Base):
    dims = 2

    def __init__(self, x, y):
        self.x = x
        self.y = y


p = Point(1, 2)
print(vars(p))
print(p.__dict__)
print(hasattr(p, "__dict__"))
print(vars(p) == {"x": 1, "y": 2})

p.color = "red"
print(vars(p))

del p.x
print(vars(p))

p.x = 99
print(vars(p))

print(sorted(vars(p).keys()))

# Class attributes and methods stay out of the instance dict.
print("kind" in vars(p), "greet" in vars(p), "dims" in vars(p))

# A fresh instance with no assignments has an empty dict.
print(vars(Base()))


class Empty:
    pass


e = Empty()
print(vars(e))
e.only = 7
print(vars(e))

for bad in (5, "text", [1, 2], None):
    try:
        vars(bad)
    except TypeError as exc:
        print("TypeError:", exc)
