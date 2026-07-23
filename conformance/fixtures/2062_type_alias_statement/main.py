# PEP 695 `type Name = value` alias statements, the form typing.py uses for
# lines like `type _Func = Callable[..., Any]`. The value is evaluated lazily,
# only when TypeAliasType.__value__ is read, so an alias that names itself
# resolves the binding that is already in place by then.


# A plain alias. The name reprs as itself and carries the alias metadata; the
# value is the evaluated right-hand side, produced on first access.
type Alias = int
print(repr(Alias))
print(Alias.__name__)
print(Alias.__value__)
print(Alias.__type_params__)
print(type(Alias).__name__)


# The value really is lazy: reading __value__ twice yields the same object, and
# it only runs the right-hand side the first time.
type Counter = dict[str, int]
first = Counter.__value__
second = Counter.__value__
print(first == second)
print(Counter.__value__)


# A recursive alias names itself in its own value. The subscription resolves the
# name that is bound before __value__ is ever forced.
type Rec = list[Rec]
print(Rec.__name__)
print(Rec.__value__)


# An alias defined inside a function binds locally and still works.
def scope():
    type Local = str
    return Local.__value__


print(scope())
