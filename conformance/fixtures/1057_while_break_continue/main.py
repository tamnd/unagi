def collatz_steps(n: int) -> int:
    steps = 0
    while True:
        if n == 1:
            break
        if n % 2 == 0:
            n = n // 2
            steps += 1
            continue
        n = 3 * n + 1
        steps += 1
    return steps


for n in range(1, 12):
    print(n, collatz_steps(n))
