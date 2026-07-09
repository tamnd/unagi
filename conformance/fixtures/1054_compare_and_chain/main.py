def rank(a: int, b: int, c: int) -> int:
    total = 0
    if a < b:
        total += 1
    if a == b:
        total += 2
    if a != c:
        total += 4
    if a <= b <= c:
        total += 8
    return total


for a in range(0, 3):
    for b in range(0, 3):
        for c in range(0, 3):
            print(a, b, c, rank(a, b, c))
