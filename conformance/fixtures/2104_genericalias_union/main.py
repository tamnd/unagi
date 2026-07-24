# A parameterized generic on either side of | builds a PEP 604 union, so
# dict[str, str] | None is a typing.Union with the alias kept as a member.
# configparser writes exactly this annotation at module scope: cursect:
# dict[str, str] | None = None.
u = dict[str, str] | None
print(u)
print(u.__args__)

# The alias can sit on either side and mix with plain types.
print(list[int] | str | None)
print(None | dict[int, int])
print((int | None) | list[str])

# The member renders with its own repr, not the bare type name.
v = tuple[int, ...] | bytes
print(v)
print(v.__args__)

# The annotation shape configparser writes at module scope.
cursect: dict[str, str] | None = None
print(cursect)
