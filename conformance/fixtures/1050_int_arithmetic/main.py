def calc(a: int, b: int) -> int:
    return a + b - a * b + a // b + a % b + a ** 2


for a in range(1, 6):
    for b in range(1, 4):
        print(a, b, calc(a, b))
