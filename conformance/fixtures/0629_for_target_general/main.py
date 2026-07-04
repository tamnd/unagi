a = [0, 0]
for a[0] in range(3):
    pass
print("sub", a)

class C:
    pass
c = C()
for c.x in ["p", "q"]:
    pass
print("attr", c.x)

for [g, h] in [[1, 2], [3, 4]]:
    print("list", g, h)

for d, [e, f] in [(1, (2, 3)), (4, (5, 6))]:
    print("nested", d, e, f)

box = {}
for [box["k"], y] in [(10, 20)]:
    print("subkey", box, y)

pairs = [[1, 2], [3, 4]]
print("comp", [x + y for [x, y] in pairs])
