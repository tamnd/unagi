# The entry shim to the static form of sq guards the argument's representation.
# A float argument enters the static float form directly, but an int argument
# violates the assumed float representation, so the type guard at the boxed-to-
# static boundary fails to the boxed body. CPython never enforces the float
# annotation, so sq(3) multiplies two ints and prints the int 9, while sq(3.0)
# prints the float 9.0. Both tiers must land on the same bytes as python3.14.
def sq(x: float) -> float:
    return x * x

print(sq(3.0))
print(sq(3))
