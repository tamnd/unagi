def fcalc(a: float, b: float) -> float:
    return a * b + a / b - a


def mixed(a: float, n: int) -> float:
    return a + n * 2


print(0.1 + 0.2)
for i in range(1, 5):
    a = i * 0.5
    b = i + 0.25
    print(fcalc(a, b), mixed(a, i))
