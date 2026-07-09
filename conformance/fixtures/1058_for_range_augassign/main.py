def summ(n: int) -> int:
    total = 0
    for i in range(n):
        total += i
        total -= i // 2
    return total


for n in range(0, 8):
    print(n, summ(n))
