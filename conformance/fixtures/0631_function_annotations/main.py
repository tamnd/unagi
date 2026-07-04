def add(a: int, b: int = 5) -> int:
    return a + b


def collect(*args: int, **kw: str) -> list:
    out = []
    for a in args:
        out.append(a)
    for k in sorted(kw):
        out.append(k + "=" + kw[k])
    return out


class Point:
    def move(self, dx: int, dy: int) -> "Point":
        self.x = dx
        self.y = dy
        return self


print(add(1))
print(add(1, 2))
print(collect(1, 2, 3, a="x", b="y"))
p = Point()
p.move(3, 4)
print(p.x, p.y)


def uses_undefined(a: Undefined) -> AlsoUndefined:
    return a


print(uses_undefined(42))
