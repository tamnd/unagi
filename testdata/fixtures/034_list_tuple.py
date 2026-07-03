xs = [1, 2, 3]
ys = xs.copy()
ys.append(4)
print(xs, ys)
xs.clear()
print(xs, len(xs))
zs = [1, 2, 1, 2, 1]
print(zs.index(2), zs.index(2, 2), zs.index(1, 1, 3))
t = (1, 2, 2, 3, 2)
print(t.count(2), t.count(9), t.index(2), t.index(2, 2), t.index(3, 1, 4))
print((True, 1, 1.0).count(1))
try:
    zs.index(9)
except ValueError as e:
    print("caught", e)
try:
    t.index(9)
except ValueError as e:
    print("caught", e)
try:
    t.count(1, 2)
except TypeError as e:
    print("caught", e)
