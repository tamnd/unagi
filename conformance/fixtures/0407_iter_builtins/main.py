xs = [3, 1, 2]
print(sorted(xs), xs)
print(sorted("banana"), sorted((2, 1)))
print(sorted({"b": 1, "a": 2}))
for i in reversed([1, 2, 3]):
    print(i)
print(list(reversed("abc")), list(reversed(range(4))))
for i, v in enumerate(["a", "b"]):
    print(i, v)
for i, v in enumerate("xy", 10):
    print(i, v)
for a, b in zip([1, 2, 3], "ab"):
    print(a, b)
print(list(zip([1, 2], [3, 4], [5, 6])))
print(list(zip()))
print(list("abc"), list(range(3)), list((1, 2)))
print(tuple([1, 2]), tuple("ab"))
print(dict([(1, "a"), (2, "b")]))
print(list({"k": 1}), tuple({1: "x", 2: "y"}))
print(sorted(set("banana")))
print(list(), tuple(), dict())
try:
    sorted([1, "a"])
except TypeError as e:
    print("caught", e)
try:
    reversed({1, 2})
except TypeError as e:
    print("caught", e)
try:
    dict([(1, 2, 3)])
except ValueError as e:
    print("caught", e)
