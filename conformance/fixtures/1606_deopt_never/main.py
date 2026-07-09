# The overflow guard never fires: every doubling stays inside int64, so the
# static form runs to the end and never hands off to its boxed twin. It pins
# that a guarded static loop with no overflow matches CPython exactly.
def accum(n: int, seed: int) -> int:
    total = seed
    for i in range(n):
        total = total * 2
    return total

print(accum(3, 1))
print(accum(10, 7))
print(accum(0, 123))
