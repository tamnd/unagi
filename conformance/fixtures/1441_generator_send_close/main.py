def echo(start):
    current = start
    while True:
        got = yield current
        if got is None:
            current = current + 1
        else:
            current = got

g = echo(10)
print(next(g))
print(g.send(100))
print(next(g))
print(g.send(200))
g.close()
g.close()

def accumulate():
    total = 0
    while True:
        x = yield total
        total += x

a = accumulate()
print(next(a))
print(a.send(5))
print(a.send(10))
print(a.send(3))

def once():
    yield 1

o = once()
print(next(o))
try:
    print(o.send("x"))
except StopIteration as e:
    print("stop", e.value)

fresh = once()
try:
    fresh.send(5)
except TypeError as e:
    print("type", e)
