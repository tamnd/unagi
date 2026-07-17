def ramp(n: float):
    x = 0.0
    while x < n:
        yield x
        x = x + 1.0


def total(n: float) -> float:
    s = 0.0
    for v in ramp(n):
        s = s + v
    return s


def last(n: float) -> float:
    r = -1.0
    for v in ramp(n):
        r = v
    return r


print(total(5.0))
print(total(0.0))
print(last(4.0))
print(last(1.0))
