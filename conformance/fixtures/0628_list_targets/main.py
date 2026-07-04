def unpack():
    [x, y] = (10, 20)
    [a, [b, c]] = [1, (2, 3)]
    return x, y, a, b, c


print(unpack())

# A list target unpacks any iterable, just like a tuple target.
[g, h] = "hi"
print(g, h)

# Elements can be arbitrary assignment targets.
d = {}
[d["k"], m] = [7, 8]
print(d, m)

# One starred element collects the middle.
[u, *v, w] = range(5)
print(u, v, w)

# Empty and single-element list targets.
[] = []
print("empty ok")
[only] = [42]
print(only)

# del over a list target deletes each element in turn.
del [g, h]
try:
    print(g)
except NameError as e:
    print("del name", e.args[0])

box = {"k": 1, "j": 2}
del [box["k"]]
print("after del subscript", box)

# Length mismatches raise the same messages as a tuple target.
try:
    [p, q] = [1, 2, 3]
except ValueError as e:
    print("too many", e.args[0])
try:
    [r, s, t] = [1, 2]
except ValueError as e:
    print("too few", e.args[0])
try:
    [] = [1]
except ValueError as e:
    print("expected empty", e.args[0])
