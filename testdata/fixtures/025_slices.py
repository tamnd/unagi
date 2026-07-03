xs = [0, 1, 2, 3, 4, 5]
print(xs[1:4])
print(xs[:3])
print(xs[3:])
print(xs[:])
print(xs[::2])
print(xs[::-1])
print(xs[4:1:-1])
print(xs[-3:])
print(xs[10:20])

s = "hello world"
print(s[0:5])
print(s[::-1])
print(s[6:])

t = (1, 2, 3, 4)
print(t[1:3])
print(t[::-1])

ys = [1, 2, 3, 4, 5]
ys[1:3] = [9]
print(ys)
ys[0:0] = [7, 8]
print(ys)
ys[:] = [1, 2, 3]
print(ys)
ys[::2] = [10, 30]
print(ys)
del ys[0:1]
print(ys)

zs = [1, 2, 3]
zs[1:] += [4]
print(zs)

try:
    print(xs[::0])
except ValueError as e:
    print("caught", str(e))
