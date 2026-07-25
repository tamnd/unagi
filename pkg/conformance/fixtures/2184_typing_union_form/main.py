import _typing
U = _typing.Union
T = _typing.TypeVar('T')
print(U[int, str])
print(U[int, str, int])
print(U[int])
print(U[int, None])
print(U[(int, str)])
print(U[int, U[str, bytes]])
print(U[None])
print(U[None, None])
print(U[T])
print(U[T, int], U[T, int].__parameters__)
print(U[int, str] | bytes)
print(bytes | U[int, str])
print(type(int | str) is U)
print(U.__name__, U.__qualname__)
print(U[list[int], str])
try:
    U[()]
except Exception as e:
    print(type(e).__name__, e)
