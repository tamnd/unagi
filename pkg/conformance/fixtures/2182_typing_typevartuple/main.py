import _typing
TVT = _typing.TypeVarTuple
Ts = TVT('Ts')
print(repr(Ts), Ts.__name__, Ts.__module__)
print(Ts.has_default(), repr(Ts.__default__))
print(Ts.__reduce__())
for a in ('__qualname__', '__bound__', '__covariant__', '__contravariant__'):
    print(a, hasattr(Ts, a))
Ds = TVT('Ds', default=(int, str))
print(Ds.has_default(), Ds.__default__)
def err(fn):
    try: fn()
    except Exception as e: print(type(e).__name__, e)
err(lambda: Ts.__typing_subst__(int))
err(lambda: TVT())
err(lambda: TVT('a', 'b'))
err(lambda: TVT(5))
err(lambda: TVT('a', bound=int))
