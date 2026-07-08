# A class answers its metaclass through __class__, the type-level data
# descriptor: a class built through a metaclass reads that metaclass, a default
# class reads the type builtin, and a builtin type reads type too. Enum's
# _create_ reads metacls = cls.__class__ for the functional API.
class Meta(type):
    pass

class C(metaclass=Meta):
    pass

class D:
    pass

print(C.__class__ is Meta)
print(C.__class__.__name__)
print(D.__class__ is type)
print(D.__class__.__name__)
print(int.__class__ is type)
print(str.__class__.__name__)

# A metaclass subclass is reported, not the base metaclass.
class Sub(Meta):
    pass

class E(metaclass=Sub):
    pass

print(E.__class__ is Sub)
print(E.__class__.__name__)
