# PEP 695 type parameter lists on functions and classes, the syntax typing.py
# and _pyrepl use for generic definitions like `def override[F: _Func](...)` and
# `class SupportsAbs[T](Protocol)`. unagi erases types at the boxed tier, so a
# type parameter list is accepted and dropped; the definition runs exactly as it
# would without one.


# A generic function. The [T] binds nothing at runtime, the body is unchanged.
def first[T](items):
    return items[0]


print(first([10, 20, 30]))
print(first(["a", "b"]))


# A generic class with methods, subscripted at the use site the way a plain
# generic would be. The subscription is a no-op annotation form here.
class Box[T]:
    def __init__(self, value):
        self.value = value

    def get(self):
        return self.value


b = Box(42)
print(b.get())


# A bounded parameter and a constrained (tuple bound) parameter still parse and
# run, since the bound is discarded with the rest.
def clamp[T: int](x):
    return x + 1


def pick[T: (int, str)](x):
    return x


print(clamp(4))
print(pick("y"))


# A TypeVarTuple and a ParamSpec in the list, plus a PEP 696 default, all parse.
class Router[*Ts, **P]:
    def route(self, name):
        return "route:" + name


print(Router().route("home"))


def build[T = int](factory):
    return factory()


print(build(lambda: "made"))


# A generic class whose parameter appears only in erased annotations, the
# `class SupportsAbs[T](Protocol)` shape typing uses: a plain base and the type
# parameter confined to method signatures, which never evaluate at runtime.
class Store[T]:
    def put(self, x: T) -> None:
        self.item = x

    def take(self) -> T:
        return self.item


s = Store()
s.put("kept")
print(s.take())
