# A class body runs as ordinary statements, not just method and variable
# definitions: control flow, augmented assignment, tuple unpacking, and a with
# block all execute against the class namespace. A name the body binds reads
# back its live value even when a branch or a loop bound it, while a name the
# namespace never held falls through to the enclosing module scope.

flag = False
base = 100


class CM:
    def __enter__(self):
        return 7

    def __exit__(self, *rest):
        return False


class C:
    if flag:
        sep = "\\"
    else:
        sep = "/"

    kinds = []
    for k in ("a", "b", "c"):
        kinds.append(k * 2)

    total = 0
    n = 0
    while n < 5:
        total += n
        n += 1

    try:
        bad = 1 // 0
    except ZeroDivisionError:
        bad = -1

    with CM() as got:
        seen = got + 1

    left, right = 3, 4
    span = left + right
    shifted = base + span


print(C.sep)
print(C.kinds)
print(C.total)
print(C.n)
print(C.bad)
print(C.seen)
print(C.span)
print(C.shifted)
