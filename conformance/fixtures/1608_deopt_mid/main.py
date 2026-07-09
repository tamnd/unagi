# The overflow guard fires in the middle of the loop: the doublings stay inside
# int64 for a while and then overflow, so the static form deopts to its boxed
# twin partway through. From-top replay recomputes the whole loop boxed, so the
# result is CPython's big int.
def accum(n: int, seed: int) -> int:
    total = seed
    for i in range(n):
        total = total * 2
    return total

print(accum(100, 1))
print(accum(70, 3))
