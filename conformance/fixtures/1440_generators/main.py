def count(n):
    i = 0
    while i < n:
        yield i * i
        i += 1
    return "done"

for x in count(4):
    print(x)

print(list(count(3)))
print(tuple(count(0)))

def letters():
    yield "a"
    yield "b"
    yield "c"

a, b, c = letters()
print(a, b, c)

total = 0
for v in count(5):
    total += v
print(total)

def const():
    yield 1
    yield 2

print(sum(const()))
print(max(count(6)))
