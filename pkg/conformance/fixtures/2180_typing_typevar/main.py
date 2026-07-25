import _typing
TypeVar = _typing.TypeVar

T = TypeVar('T')
print(repr(T), T.__name__, T.__bound__, T.__constraints__)
print(T.__covariant__, T.__contravariant__, T.__infer_variance__, T.has_default(), repr(T.__default__))
print(repr(TypeVar('T_co', covariant=True)))
print(repr(TypeVar('T_contra', contravariant=True)))
print(repr(TypeVar('T', infer_variance=True)))

B = TypeVar('B', bound=int)
print(B.__bound__, B.__constraints__)
C = TypeVar('C', int, str)
print(C.__constraints__, C.__bound__)
D = TypeVar('D', default=int)
print(D.has_default(), D.__default__)
E = TypeVar('E', default=None)
print(E.has_default(), repr(E.__default__))

print(T.__typing_subst__(int))
print(T.__reduce__())
print(repr(T | int))
print(repr(int | T))
print(T.__module__)

def err(fn):
    try:
        fn()
    except Exception as e:
        print(type(e).__name__, e)

err(lambda: TypeVar('X', int, str, bound=float))
err(lambda: TypeVar('Y', int))
err(lambda: TypeVar('Z', covariant=True, contravariant=True))
err(lambda: TypeVar('Q', infer_variance=True, covariant=True))
err(lambda: TypeVar(5))
err(lambda: TypeVar('W', weird=1))
err(lambda: T.__mro_entries__((T,)))
