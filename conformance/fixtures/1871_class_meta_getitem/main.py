# Subscripting a class resolves through the type's type, so a metaclass
# __getitem__ runs with the class as self; a class on the default metatype
# stays not-subscriptable unless it defines __class_getitem__.

class Meta(type):
    def __getitem__(cls, key):
        return cls._store[key]


class C(metaclass=Meta):
    _store = {"a": 1, "b": 2}


print(C["a"], C["b"])

try:
    C["z"]
except KeyError:
    print("missing key")


class Plain:
    pass


try:
    Plain[0]
except TypeError as e:
    print(e)


class Generic:
    def __class_getitem__(cls, item):
        return (cls.__name__, item)


print(Generic[int])
