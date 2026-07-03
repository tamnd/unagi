x = 1
x += 4
x -= 2
x *= 6
x //= 4
x %= 3
x **= 5
print(x)
f = 2.0
f /= 8
print(f)
xs = [1, 2, 3]
xs[1] += 10
print(xs)
total = 0
for i in range(100):
    if i % 2 == 0:
        continue
    if i > 10:
        break
    total += i
print(total)
s = ""
s += "ab"
s += "c"
print(s)
