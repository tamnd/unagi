# get_x reads a fixed-shape attribute. Its static form guards the argument's
# class at the boxed-to-static boundary, then reads the x slot as a plain Go
# struct field. A Point argument matches the shape the guard assumes, so the
# read runs on the flat struct. A Tag argument carries an x slot of its own but
# is a different class, so the shape guard fails and the call deopts to the
# boxed body, which reads x back through the object protocol. CPython enforces
# no annotation and reads x off whatever object it is handed, so it prints 7
# then 9; both unagi tiers must land on those same bytes as python3.14.
class Point:
    __slots__ = ("x", "y")
    x: int
    y: int

    def __init__(self, x, y):
        self.x = x
        self.y = y


class Tag:
    __slots__ = ("x",)
    x: int

    def __init__(self, x):
        self.x = x


def get_x(p: Point) -> int:
    return p.x


print(get_x(Point(7, 2)))
print(get_x(Tag(9)))
