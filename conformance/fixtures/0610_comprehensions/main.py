x = 100
r = [x * 2 for x in range(4)]
print(r, x)

y = 0
vals = [y := i for i in range(3)]
print(vals, y)

m = [i + j for i in range(3) for j in range(2)]
print(m)

e = [i for i in range(10) if i % 2 == 0 if i > 2]
print(e)

n = [[j for j in range(i)] for i in range(4)]
print(n)

pairs = [(a, b) for a, b in [(1, 2), (3, 4)]]
print(pairs)

star = [a for *a, b in [[1, 2, 3], [4, 5]]]
print(star)

dup = [i for i in range(2) for i in range(3)]
print(dup, len(dup))

w = [v for i in range(5) if (v := i * i) > 4]
print(w, v)

fs = [lambda: i for i in range(3)]
print([g() for g in fs])

outer = [x for x in [x]]
print(outer)

total = 0
seen = [total := total + n for n in [1, 2, 3]]
print(seen, total)

sx = {c for c in "hello"}
print(sorted(sx))

evens = {i % 3 for i in range(9)}
print(sorted(evens))

d = {k: k * k for k in range(4)}
print(d)

d2 = {v: k for k, v in d.items()}
print(d2)

dd = {k: k for k in [1, 1, 2, 2]}
print(dd)

chars = [c + "!" for c in "ab"]
print(chars)

flat = [v for row in [[1, 2], [3], []] for v in row]
print(flat)

def squares(nums, cut):
    return [n * n for n in nums if n >= cut]

print(squares([1, 2, 3, 4], 3))
print(squares(range(6), 0))

empty = [q for q in []]
print(empty, len(empty))

cond = {k: ("big" if k > 1 else "small") for k in range(3)}
print(cond)
