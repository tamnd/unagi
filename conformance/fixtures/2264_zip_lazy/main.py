"""zip is lazy: it pulls one value per input per row and stops at the shortest
without consuming past it. Covers the tail-not-consumed guarantee, bounded
iteration over an unbounded generator, and the strict= mismatch reports."""

# Stopping at the shortest input leaves the longer one's tail untouched.
it = iter([5, 1, 9, 3, 7])
pairs = list(zip(range(3), it))
print("pairs:", pairs)
print("tail remaining:", list(it))

# Even with the short input second, the shared iterator keeps what zip skipped.
shared = iter([1, 2, 3, 4, 5])
list(zip(shared, [10, 20]))
print("shared remaining:", list(shared))


# zip never fully drains an unbounded generator -- it stops when range does.
def naturals():
    n = 0
    while True:
        yield n
        n += 1


print("bounded by range:", list(zip(range(4), naturals())))

# A generator with a side effect proves how many times zip pulled it: two rows
# emitted, so exactly two pulls, no speculative third.
pulled = []


def counting():
    n = 0
    while True:
        pulled.append(n)
        yield n
        n += 1


list(zip([0, 0], counting()))
print("pull count:", len(pulled))

# Multiple inputs stop at the first to run dry.
print("three uneven:", list(zip([1, 2, 3], "ab", (9, 8, 7, 6))))


# strict= turns a length mismatch into a ValueError, worded by which input ran
# short and how many precede it.
def show(fn):
    try:
        fn()
        print("no raise")
    except ValueError as e:
        print("ValueError:", e)


show(lambda: list(zip([1, 2, 3], [1, 2], strict=True)))            # 2nd shorter
show(lambda: list(zip([1, 2], [1, 2, 3], strict=True)))            # 2nd longer
show(lambda: list(zip([1, 2, 3], [1, 2, 3], [1, 2], strict=True)))  # 3rd shorter
show(lambda: list(zip([1, 2], [1, 2, 3], [1, 2, 3], strict=True)))  # 2nd longer
print("strict equal:", list(zip([1, 2], [3, 4], strict=True)))

# Empty and single-input zips, and the eager not-iterable check.
print("no args:", list(zip()))
print("single:", list(zip("abc")))
try:
    zip([1, 2], 5)
except TypeError as e:
    print("not iterable:", e)
