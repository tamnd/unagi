if (n := 10) > 5:
    print("n is", n)
print("after if", n)

xs = [1, 2, 3]
while (m := len(xs)) > 0:
    print("len", m, xs)
    del xs[0]
print("drained", m)

val = (t := 3) + t * 2
print(val, t)

def half(k):
    return k // 2

data = [1, 4, 9]
for d in data:
    if (h := half(d)) > 0:
        print(d, "halves to", h)
