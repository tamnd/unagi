# Builtin generic subscription: list[int] and its kin build a types.GenericAlias,
# the value _collections_abc opens with as `GenericAlias = type(list[int])` and
# hangs off its ABCs. What matters is the repr, the __origin__/__args__ readout,
# that calling the alias constructs the origin, and that aliases hash and compare
# by origin plus args so a set dedups them.

import types

# A subscript reprs as origin[args] with the type names bare, tuples flattened,
# and Ellipsis spelled ...
print(list[int])
print(dict[str, int])
print(tuple[int, ...])
print(set[frozenset[int]])
print(list[list[int]])

# The type of every alias is types.GenericAlias by identity.
print(type(list[int]) is types.GenericAlias)
print(type(dict[str, int]) is types.GenericAlias)

# __origin__ is the parameterized type; __args__ is always a tuple; a single
# argument still lands in a one-tuple.
print(list[int].__origin__ is list, list[int].__args__)
print(dict[str, int].__origin__ is dict, dict[str, int].__args__)
print(tuple[int, ...].__args__)
print(list[int].__parameters__)

# Calling an alias constructs its origin, arguments erased.
print(list[int]())
print(dict[str, int]([("a", 1)]))
print(tuple[int, ...]([1, 2, 3]))

# Equality and hashing key by origin and args, so a set dedups repeats and keeps
# the distinct ones.
print(list[int] == list[int], list[int] == list[str], list[int] == dict[int, int])
print(hash(list[int]) == hash(list[int]))
print(len({list[int], list[int], list[str], dict[str, int]}))

# The explicit constructor reaches the same value as the subscript.
print(types.GenericAlias(list, (int,)) == list[int])
print(types.GenericAlias(list, int))

# A builtin that is not a container type has no __class_getitem__, so it stays a
# TypeError.
try:
    len[int]
except TypeError as e:
    print("no subscript:", type(e).__name__)
