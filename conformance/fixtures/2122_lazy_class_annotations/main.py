# PEP 649: a class body no longer evaluates its variable annotations as it runs.
# CPython 3.14 stashes them in a deferred __annotate__ and only evaluates them
# when C.__annotations__ is first read. That is what lets a class annotate a name
# it imports only under `if TYPE_CHECKING:` (which never runs) without raising at
# class-definition time, the pattern the stdlib leans on (e.g. _colorize's
# `__dataclass_fields__: ClassVar[...]` where ClassVar is a typing-only import).

# A forward reference and a not-yet-defined name are both fine at definition; the
# annotation is a deferred thunk, not an eager lookup.
class C:
    x: int
    y: "Fwd"
    z: str = "hi"


# The value with an annotation is still assigned eagerly.
print(C.z)

# __annotations__ realizes the thunks in order on first read, as a plain dict.
print(C.__annotations__ == {"x": int, "y": "Fwd", "z": str})
print(list(C.__annotations__))

# The realized dict lives off the class dict, so it is not a __dict__ key, the
# PEP 649 shape 3.14 reports, and repeated reads hand back the same object.
print("__annotations__" in C.__dict__)
print(C.__annotations__ is C.__annotations__)


# An annotation naming something never defined costs nothing at definition and
# raises only when the annotations are accessed.
class D:
    a: Nope


print("D defined")
try:
    D.__annotations__
    print("no error")
except NameError:
    print("NameError on access")


# The TYPE_CHECKING pattern: the name exists only under a guard that never runs.
if False:
    from typing import ClassVar


class E:
    q: ClassVar[int]


print("E defined")

# A class that declared no annotations still answers with an empty dict.
class F:
    pass


print(F.__annotations__ == {})
