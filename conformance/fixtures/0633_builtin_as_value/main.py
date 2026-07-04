f = len
print(f([1, 2, 3]))

funcs = [str, int, abs]
print(funcs[0](42), funcs[1]("7"), funcs[2](-5))

print(len)
print(list)
print(dict)
print(sorted)
print(range)
print(str)
print(next)
print(isinstance)

ops = {"a": abs, "s": sum}
print(ops["a"](-9), ops["s"]([1, 2, 3]))

print(sorted([3, 1, 2]))
print(max([4, 9, 2]), min([4, 9, 2]))


def apply(fn, x):
    return fn(x)


print(apply(abs, -7))
print(apply(hex, 255))

chosen = max
print(chosen([1, 5, 3]))

length = len
saved = length
print(saved("hello"))


def gen():
    yield 10
    yield 20


def star_next(*args):
    return next(*args)


g = gen()
print(star_next(g))
print(star_next(g, "done"))
print(star_next(g, "done"))
