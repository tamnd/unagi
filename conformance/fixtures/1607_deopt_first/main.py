# The overflow guard fires on the first doubling: the seed is a valid int64 but
# total * 2 overflows immediately, so the static form deopts to its boxed twin
# on the first loop iteration. The result must be CPython's big int.
def accum(n: int, seed: int) -> int:
    total = seed
    for i in range(n):
        total = total * 2
    return total

print(accum(1, 5000000000000000000))
print(accum(4, 5000000000000000000))
