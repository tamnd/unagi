import _typing
TAT = _typing.TypeAliasType
T = _typing.TypeVar('T')
P = _typing.ParamSpec('P')
A = TAT('A', list[int])
print(repr(A), A.__name__, A.__module__, A.__value__, A.__type_params__, A.__parameters__, type(A).__name__)
B = TAT('B', list[T], type_params=(T,))
print(B.__type_params__, B.__parameters__, repr(B[int]), type(B[int]).__name__, B[int].__args__, B[int].__origin__ is B)
C = TAT('C', dict, type_params=(T, P))
print(C.__type_params__, C.__parameters__)
def e(f):
    try: print(f())
    except Exception as ex: print(type(ex).__name__, ex)
e(lambda: TAT())
e(lambda: TAT('X'))
e(lambda: TAT(5, int))
e(lambda: TAT('X', int, type_params=5))
e(lambda: TAT('X', int, foo=1))
e(lambda: A.__qualname__)
