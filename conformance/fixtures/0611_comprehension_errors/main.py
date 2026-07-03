try:
    r = [x for x in 5]
except TypeError as e:
    print("A:", e)

try:
    r = [y for y in range(3) for z in y]
except TypeError as e:
    print("B:", e)

try:
    s = {[i] for i in range(2)}
except TypeError as e:
    print("C:", e)

try:
    d = {[i]: i for i in range(2)}
except TypeError as e:
    print("D:", e)

try:
    d = {i: [i] for i in range(2)}
    print("E:", d)
except TypeError as e:
    print("E?", e)

try:
    p = [(a, b) for a, b in [(1,)]]
except ValueError as e:
    print("F:", e)

try:
    p = [(a, b) for a, b in [(1, 2, 3)]]
except ValueError as e:
    print("G:", e)

try:
    p = [(a, b) for a, b in [7]]
except TypeError as e:
    print("H:", e)

def boom():
    raise ValueError("mid-iter")

try:
    r = [boom() for i in range(3)]
except ValueError as e:
    print("I:", e)

try:
    r = [i for i in range(3) if boom()]
except ValueError as e:
    print("J:", e)

try:
    d = {boom(): i for i in range(1)}
except ValueError as e:
    print("K:", e)

order = []
key = lambda i: (order.append("k" + str(i)), i)[1]
val = lambda i: (order.append("v" + str(i)), i)[1]
d = {key(i): val(i) for i in range(2)}
print(order)
print(d)

steps = []
mark = lambda tag, v: (steps.append(tag), v)[1]
r = [mark("elt" + str(i), i) for i in mark("outer", range(2))]
print(steps, r)

part = []
try:
    r = [part.append(i) or i for i in [0, 1, "x", 3] if i < 2]
except TypeError as e:
    print("L:", e, part)
