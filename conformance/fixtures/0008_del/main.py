x = 10
del x
try:
    print(x)
except NameError as e:
    print("caught", str(e))

xs = [1, 2, 3, 4, 5]
del xs[1]
print(xs)
del xs[1:3]
print(xs)

d = {"a": 1, "b": 2}
del d["a"]
print(d)

def f(a):
    b = a + 1
    del b
    try:
        print(b)
    except UnboundLocalError as e:
        print("caught in f", str(e))
    b = a * 2
    return b

print(f(3))

x = 5
del x
try:
    del x
except NameError as e:
    print("caught again", str(e))
