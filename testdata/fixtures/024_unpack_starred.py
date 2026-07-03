a, b = 1, 2
a, b = b, a
print(a, b)

x = y = z = [1, 2]
y.append(3)
print(x, y, z)

first, *rest = [1, 2, 3, 4]
print(first, rest)

*init, last = [1, 2, 3, 4]
print(init, last)

head, *mid, tail = [10, 20, 30, 40, 50]
print(head, mid, tail)

p, *q = [1]
print(p, q)

for u, *v in [[1, 2, 3], [4, 5], [6]]:
    print(u, v)

try:
    c, *ds, e = [1]
except ValueError as err:
    print("caught", str(err))

try:
    *f, g, h = [1]
except ValueError as err:
    print("caught", str(err))
