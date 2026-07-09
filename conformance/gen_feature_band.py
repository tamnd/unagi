#!/usr/bin/env python3.14
# Authors the M4 feature-tag band (doc 10 items 9 and 10): a small set of
# fixtures under the control-flow band, each exercising one family of landed
# static lowerings, tagged so the coverage test can assert every landed S case
# maps to at least one fixture. The oracle is captured from the pinned
# python3.14, never hand-written. Rerun to regenerate the band in place.
import os
import sys

ROOT = os.path.join(os.path.dirname(__file__), "fixtures")

# Each entry: id, snake name, tag list, program source. The program keeps a
# typed function as the static unit the forced-static rerun lowers, driven by a
# top-level loop that prints results so the whole program has a CPython oracle.
FIXTURES = [
    (1050, "int_arithmetic", ["int-add", "int-sub", "int-mul", "int-floordiv", "int-mod", "int-pow"], '''\
def calc(a: int, b: int) -> int:
    return a + b - a * b + a // b + a % b + a ** 2


for a in range(1, 6):
    for b in range(1, 4):
        print(a, b, calc(a, b))
'''),
    (1051, "int_bitwise_shift", ["int-bitand", "int-bitor", "int-bitxor", "int-lshift", "int-rshift"], '''\
def bits(a: int, b: int) -> int:
    return (a & b) + (a | b) + (a ^ b) + (a << 1) + (a >> 1)


for a in range(0, 8):
    for b in range(0, 4):
        print(a, b, bits(a, b))
'''),
    (1052, "int_fold_identity", ["int-const-fold", "int-identity"], '''\
def folded() -> int:
    return 2 + 3 * 4 - 6 // 2


def identity(x: int) -> int:
    return x + 0 + (x * 1) - (x << 0)


print(folded())
for x in range(-3, 4):
    print(x, identity(x))
'''),
    (1053, "float_arith_coerce_repr", ["float-arith", "float-int-coerce", "float-repr"], '''\
def fcalc(a: float, b: float) -> float:
    return a * b + a / b - a


def mixed(a: float, n: int) -> float:
    return a + n * 2


print(0.1 + 0.2)
for i in range(1, 5):
    a = i * 0.5
    b = i + 0.25
    print(fcalc(a, b), mixed(a, i))
'''),
    (1054, "compare_and_chain", ["compare-int", "compare-chain"], '''\
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
'''),
    (1055, "bool_connectives", ["bool-and", "bool-or", "bool-not", "truthiness"], '''\
def logic(a: int, b: int) -> int:
    r = 0
    if a and b:
        r += 1
    if a or b:
        r += 2
    if not a:
        r += 4
    if a:
        r += 8
    return r


for a in range(0, 2):
    for b in range(0, 2):
        print(a, b, logic(a, b))
'''),
    (1056, "if_elif_else_chain", ["if-elif-else"], '''\
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
'''),
    (1057, "while_break_continue", ["while", "break-continue"], '''\
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
'''),
    (1058, "for_range_augassign", ["for-range", "augassign"], '''\
def summ(n: int) -> int:
    total = 0
    for i in range(n):
        total += i
        total -= i // 2
    return total


for n in range(0, 8):
    print(n, summ(n))
'''),
    (1059, "static_call_recursion", ["static-call", "recursion"], '''\
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
'''),
]


def toml_for(tags):
    body = ", ".join(f'"{t}"' for t in tags)
    return f"tags = [{body}]\n"


def main():
    for fid, name, tags, src in FIXTURES:
        dname = f"{fid:04d}_{name}"
        ddir = os.path.join(ROOT, dname)
        os.makedirs(ddir, exist_ok=True)
        with open(os.path.join(ddir, "main.py"), "w") as f:
            f.write(src)
        with open(os.path.join(ddir, "fixture.toml"), "w") as f:
            f.write(toml_for(tags))
        # The oracle.golden is captured separately by `unagi-conformance record`
        # so the provenance stays the pinned python3.14, never this generator.
        print(f"wrote {dname} ({len(tags)} tags)")


if __name__ == "__main__":
    main()
