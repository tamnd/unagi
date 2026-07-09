def bucket(n: int) -> int:
    if n < 0:
        return -1
    elif n == 0:
        return 0
    elif n < 10:
        return 1
    else:
        return 2


for n in [-5, 0, 3, 9, 10, 42]:
    print(n, bucket(n))
