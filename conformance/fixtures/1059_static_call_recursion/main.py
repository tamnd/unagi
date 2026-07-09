def square(x: int) -> int:
    return x * x


def poly(x: int) -> int:
    return square(x) + square(x + 1)


def fib(n: int) -> int:
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)


for x in range(0, 6):
    print(x, poly(x), fib(x))
