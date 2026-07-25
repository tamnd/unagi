# import typing with no cwd shim: the native _typing module backs it, so the
# vendored typing.py runs unmodified and the Generic bootstrap behaves.
from typing import (
    Generic, TypeVar, Optional, Union, List, Dict, Callable, Tuple,
    Protocol, runtime_checkable, get_type_hints,
)

T = TypeVar("T")
K = TypeVar("K")
V = TypeVar("V")


class Box(Generic[T]):
    def __init__(self, v: T) -> None:
        self.v = v


print(Box.__parameters__, Box[int], Box[int].__origin__, Box[int].__args__)


class Pair(Generic[K, V]):
    pass


print(Pair.__parameters__, Pair[int, str])


class IntBox(Box[int]):
    pass


print(IntBox.__orig_bases__, Box in IntBox.__mro__, IntBox.__parameters__)

print(Optional[int], Union[int, str], type(int | str) is Union)
print(List[int], Dict[str, int], Callable[[int, str], bool], Tuple[int, ...])


@runtime_checkable
class Sized(Protocol):
    def __len__(self) -> int: ...


class HasLen:
    def __len__(self):
        return 3


print(isinstance(HasLen(), Sized), isinstance(5, Sized))


def f(x: int, y: str) -> bool:
    return True


print(get_type_hints(f))
