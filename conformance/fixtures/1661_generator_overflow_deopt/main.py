def scaled(base: int, n: int):
    i = 0
    while i < n:
        i = i + 1
        yield base * i


def total(base: int, n: int) -> int:
    s = 0
    for v in scaled(base, n):
        s = s + v
    return s


def last(base: int, n: int) -> int:
    r = 0
    for v in scaled(base, n):
        r = v
    return r


print(total(3, 5))
print(last(3, 5))
print(total(1000000000000000000, 20))
print(last(1000000000000000000, 20))
