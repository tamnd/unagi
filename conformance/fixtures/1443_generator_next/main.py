def nums():
    yield 10
    yield 20

g = nums()
print(next(g))
print(next(g))
print(next(g, "default"))
print(next(g, "default"))

def empty():
    return
    yield

e = empty()
try:
    next(e)
except StopIteration:
    print("empty stopped")

def with_return():
    yield 1
    return 42

w = with_return()
print(next(w))
try:
    next(w)
except StopIteration as ex:
    print("value", ex.value)

try:
    next([1, 2, 3])
except TypeError as ex:
    print("err", ex)
