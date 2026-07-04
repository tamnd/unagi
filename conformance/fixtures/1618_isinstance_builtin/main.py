# isinstance and issubclass reach the builtin type values, and match
# class patterns gate on them with the self-match rules.


def show(label, value):
    print(label, value)


# isinstance over the builtin subtype lattice.
show("int", isinstance(5, int))
show("bool-in-int", isinstance(True, int))
show("bool", isinstance(True, bool))
show("int-not-bool", isinstance(5, bool))
show("float-not-int", isinstance(1.0, int))
show("str", isinstance("x", str))
show("list", isinstance([1], list))
show("tuple", isinstance((1,), tuple))
show("dict", isinstance({1: 2}, dict))
show("set", isinstance({1}, set))
show("frozenset", isinstance(frozenset(), frozenset))
show("bytes", isinstance(b"x", bytes))
show("float", isinstance(1.0, float))

# A tuple of types matches when any element does.
show("tuple-hit", isinstance(5, (str, float, int)))
show("tuple-miss", isinstance(5, (str, float)))

# type membership: a type value is an instance of type, a plain value is not.
show("int-is-type", isinstance(int, type))
show("type-is-type", isinstance(type, type))
show("value-not-type", isinstance(5, type))


class Animal:
    pass


class Dog(Animal):
    pass


# User instances never match a builtin type, and builtin values never a user class.
show("dog-not-int", isinstance(Dog(), int))
show("five-not-dog", isinstance(5, Dog))
show("dog-cls-is-type", isinstance(Dog, type))
show("dog-is-animal", isinstance(Dog(), Animal))

# issubclass over builtins.
show("bool-sub-int", issubclass(bool, int))
show("int-sub-int", issubclass(int, int))
show("int-not-sub-bool", issubclass(int, bool))
show("int-not-sub-float", issubclass(int, float))
show("int-not-sub-type", issubclass(int, type))
show("type-sub-type", issubclass(type, type))
show("bool-sub-tuple", issubclass(bool, (str, int)))
show("dog-sub-animal", issubclass(Dog, Animal))
show("dog-not-sub-int", issubclass(Dog, int))
show("int-not-sub-dog", issubclass(int, Dog))

# Argument errors keep CPython's wording and order.
try:
    issubclass(5, int)
except TypeError as e:
    print("issub-arg1", e)
try:
    isinstance(5, 3)
except TypeError as e:
    print("isinst-arg2", e)


# match self-patterns over builtins, bool tried before int so the order matters.
def classify(v):
    match v:
        case bool():
            return "bool"
        case int(x):
            return "int:" + str(x)
        case str(s):
            return "str:" + s
        case float():
            return "float"
        case _:
            return "other"


for v in [5, True, "hi", 1.0, [1]]:
    print("classify", classify(v))


# A non-self-match builtin rejects a positional sub-pattern.
def bad_range(v):
    match v:
        case range(x):
            return x
    return "none"


try:
    bad_range(range(3))
except TypeError as e:
    print("range-match", e)


# Too many positionals on a self-match builtin.
def bad_int(v):
    match v:
        case int(a, b):
            return (a, b)
    return "none"


try:
    bad_int(5)
except TypeError as e:
    print("int-arity", e)
