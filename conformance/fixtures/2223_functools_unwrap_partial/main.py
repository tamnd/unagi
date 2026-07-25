import functools

def base(a, b):
    return a + b

# _unwrap_partial peels a partial chain down to the innermost callable.
p = functools.partial(functools.partial(base, 1), 2)
print(functools._unwrap_partial(p) is base)
print(functools._unwrap_partial(base) is base)

# _unwrap_partialmethod agrees for plain callables and partials, the values
# inspect passes it on the way to a function's code flags.
print(functools._unwrap_partialmethod(p) is base)
print(functools._unwrap_partialmethod(base) is base)

# The modules that reach this helper through inspect now import.
import pdb
import doctest
print('pdb' in dir(pdb) or hasattr(pdb, 'set_trace'))
print(hasattr(doctest, 'testmod'))
