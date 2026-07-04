a = [1, 2, 3]
b = (4, 5)
print([*a, 99, *b])
print((*a, *b))
print((*a,))
s = {*a, 7, *[7, 8]}
print(sorted(s))
print([*range(3), *"xy"])
x = *a, 100
print(x)
c, *rest = [*a, *b]
print(c, rest)
print(f"{[*a, *b]}")
try:
    bad = [*5]
except TypeError as e:
    print("TypeError:", e)
