# Subclassing types.GenericAlias, the shape _collections_abc uses for
# _CallableGenericAlias so collections.abc.Callable[[int], str] carries a custom
# repr and __args__ flattening. unagi keeps the parameterized generic a subclass
# instance wraps as its payload: __origin__/__args__/__parameters__ read through
# it, super().__new__ builds it from the constructor arguments, and
# super().__repr__ prints it in the base list[int] shape. This drives that
# surface without the TypeVar substitution __getitem__ path, which needs a
# generic with free parameters and is a later slice.
from types import GenericAlias


class _CallableGenericAlias(GenericAlias):
    __slots__ = ()

    def __new__(cls, origin, args):
        if not (isinstance(args, tuple) and len(args) == 2):
            raise TypeError("Callable must be used as Callable[[arg, ...], result].")
        t_args, t_result = args
        if isinstance(t_args, (tuple, list)):
            args = (*t_args, t_result)
        return super().__new__(cls, origin, args)

    def __repr__(self):
        args = self.__args__
        return "Callable[[%s], %s]" % (
            ", ".join(a.__name__ for a in args[:-1]),
            args[-1].__name__,
        )


# __new__ flattens the argument list, so __args__ is (int, str, float).
c = _CallableGenericAlias(list, ([int, str], float))
print("type:", type(c).__name__)
print("origin:", c.__origin__)
print("args:", c.__args__)
print("params:", c.__parameters__)
print("repr:", repr(c))
print("is GenericAlias:", isinstance(c, GenericAlias))

# The constructor guard raises the same TypeError the real class does.
try:
    _CallableGenericAlias(list, (int,))
except TypeError as e:
    print("guard:", e)


# A subclass with no __new__ wraps the two constructor arguments directly, and a
# __repr__ override reaches super().__repr__ for the base list[int] shape. Only
# dunder names are overridden, the way _CallableGenericAlias does, because
# GenericAlias forwards other attribute reads to the origin.
class Plain(GenericAlias):
    __slots__ = ()

    def __repr__(self):
        return "plain(" + super().__repr__() + ")"


p = Plain(dict, (str, int))
print("plain args:", p.__args__)
print("plain origin:", p.__origin__)
print("plain repr:", repr(p))
