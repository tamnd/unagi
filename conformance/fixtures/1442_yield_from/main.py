def inner():
    yield 1
    yield 2
    return "inner-done"

def middle():
    r = yield from inner()
    print("got", r)
    yield 3

print(list(middle()))

def flatten(seqs):
    for s in seqs:
        yield from s

print(list(flatten([[1, 2], [3], [], [4, 5]])))

def chain():
    yield from range(3)
    yield from "ab"
    yield 99

print(list(chain()))

def collatz(n):
    while n != 1:
        yield n
        if n % 2 == 0:
            n = n // 2
        else:
            n = 3 * n + 1
    yield 1

def run():
    yield from collatz(6)

print(list(run()))
