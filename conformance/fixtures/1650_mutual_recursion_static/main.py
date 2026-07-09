# Two functions that only call each other prove static together through the
# greatest-fixpoint seed. The round-based partition resolver grows its
# proven-static set from an empty seed, so a plain least fixpoint leaves both
# boxed: each waits on the other and neither is ever offered to the resolver. The
# seed decides the cycle from the top instead, so is_even and is_odd emit a
# mutual recursion in the static tier. Both bodies are guard-free, the float
# subtract is total and the comparison carries no guard, so the pair proves
# static with no deopt edge. Nothing here can fall back, so the forced-static and
# forced-boxed reruns must land on the same bytes as python3.14.
def is_even(n: float) -> bool:
    if n <= 0.0:
        return True
    return is_odd(n - 1.0)

def is_odd(n: float) -> bool:
    if n <= 0.0:
        return False
    return is_even(n - 1.0)

print(is_even(0.0))
print(is_even(4.0))
print(is_even(7.0))
print(is_odd(4.0))
print(is_odd(7.0))
