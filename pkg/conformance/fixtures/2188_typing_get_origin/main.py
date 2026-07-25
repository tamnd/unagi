# typing.get_origin recovers the origin of every parameterized form, including a
# ParamSpec's .args / .kwargs members. Its `isinstance(tp, (_BaseGenericAlias,
# GenericAlias, ParamSpecArgs, ParamSpecKwargs))` guard needs the two member
# constructors to count as valid isinstance types, or a bare `get_origin(int)`
# raises the arg-2 TypeError before it can return None.
import typing
from typing import ParamSpec, TypeVar, get_origin, get_args

T = TypeVar("T")
P = ParamSpec("P")

print(get_origin(int))
print(get_origin(typing.List[int]), get_args(typing.List[int]))
print(get_origin(typing.Dict[str, int]), get_args(typing.Dict[str, int]))
print(get_origin(list[int]), get_args(list[int]))
print(get_origin(typing.Union[int, str]))
print(get_origin(typing.Optional[int]))
print(get_origin(P.args) is P, get_origin(P.kwargs) is P)
print(get_origin(T) is None)
print(isinstance(P.args, type(P.args)))

# TypedDict builds its fields off the class-body annotate and reads them back.
class Movie(typing.TypedDict):
    name: str
    year: int

print(Movie.__annotations__, Movie.__total__)
print(sorted(Movie.__required_keys__), sorted(Movie.__optional_keys__))

class Partial(typing.TypedDict, total=False):
    a: int
    b: str

print(Partial.__total__, sorted(Partial.__optional_keys__))
