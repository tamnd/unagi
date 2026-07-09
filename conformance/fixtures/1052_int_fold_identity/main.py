def folded() -> int:
    return 2 + 3 * 4 - 6 // 2


def identity(x: int) -> int:
    return x + 0 + (x * 1) - (x << 0)


print(folded())
for x in range(-3, 4):
    print(x, identity(x))
