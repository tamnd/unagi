# PEP 560: a class statement may name a subscripted generic as a base, like
# `class C(Mapping[str, str])`. That base is not a type, so it cannot go in the
# MRO directly. Python calls its __mro_entries__ to get the real bases to use,
# the origin, and records the original tuple as __orig_bases__. The stdlib leans
# on this, for instance _colorize deriving a theme section from
# collections.abc.Mapping[str, str], so traceback and logging reach it.

from collections.abc import Mapping


class S(Mapping[str, str]):
    def __getitem__(self, k):
        return "v"

    def __iter__(self):
        return iter(())

    def __len__(self):
        return 0


# The alias base collapsed to its origin for the actual bases and the MRO, so the
# class is a real Mapping subclass its abstract methods complete.
print(S.__bases__ == (Mapping,))
print(isinstance(S(), Mapping))
print(len(S()))

# The parameterized base survives as __orig_bases__, so typing can recover it.
print(S.__orig_bases__[0].__origin__ is Mapping)
print(S.__orig_bases__[0].__args__)


# A subscripted builtin container works the same way: list[int] resolves to list.
class L(list[int]):
    pass


print(L.__bases__ == (list,))
print(L([1, 2, 3]) == [1, 2, 3])


# A plain type base is untouched: types carry no __mro_entries__, so nothing is
# rewritten and no __orig_bases__ is recorded.
class P(dict):
    pass


print(hasattr(P, "__orig_bases__"))
print(hasattr(int, "__mro_entries__"))

# The C types.GenericAlias __mro_entries__ always names the origin, without
# deduplicating against the bases it is handed.
ga = Mapping[str, str]
print(ga.__mro_entries__((ga,)) == (Mapping,))
print(ga.__mro_entries__((object,)) == (Mapping,))
