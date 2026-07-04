def stub():
    ...


class Proto:
    ...


class Doc:
    """a docstring then a stub body"""
    ...


stub()
print(stub())

x = ...
print(x)
print(x is ...)
print(... is None)
print(... == ...)

d = {...: "ell", 1: "one"}
print(d[...])
print(len(d))

print([..., 1, ...])
print((...,))
print({...} == {...})


def g(a, b=...):
    if b is ...:
        return "default"
    return b


print(g(1))
print(g(1, 2))

vals = [..., None, ...]
print(vals.count(...))
