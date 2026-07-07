def plain():
    return 1


# The three descriptor constructors are types.
print(type(staticmethod) is type)
print(type(classmethod) is type)
print(type(property) is type)
print(repr(staticmethod))
print(repr(classmethod))
print(repr(property))

# isinstance and issubclass accept them.
sm = staticmethod(plain)
cm = classmethod(plain)
pr = property(plain)
print(isinstance(sm, staticmethod))
print(isinstance(cm, classmethod))
print(isinstance(pr, property))
print(isinstance(sm, (classmethod, staticmethod)))
print(isinstance(plain, staticmethod))
print(issubclass(staticmethod, object))
print(issubclass(bool, int))

# The wrapped callable reads back through __func__ and __wrapped__.
print(sm.__func__ is plain)
print(cm.__func__ is plain)
print(sm.__wrapped__ is plain)

# The EnumDict.__setitem__ shape: unwrap a staticmethod, pass everything else through.
def unwrap(value):
    return value.__func__ if isinstance(value, staticmethod) else value


print(unwrap(sm) is plain)
print(unwrap(42))
