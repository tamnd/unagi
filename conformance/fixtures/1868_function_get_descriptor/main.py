# A function is a descriptor: it answers __get__, so hasattr(f, '__get__') is
# True while __set__ and __delete__ are absent. This is the protocol a class
# body uses to tell a method from data, the check enum's _is_descriptor runs so
# a method like Flag._get_value is not mistaken for a member.


def f(self, x):
    return (self.tag, x)


# The descriptor slots: a function binds but does not set or delete.
print(hasattr(f, "__get__"))
print(hasattr(f, "__set__"))
print(hasattr(f, "__delete__"))


class C:
    tag = "c"


c = C()

# Bound to an instance it yields a bound method carrying __func__ and __self__,
# and calling it prepends the instance as self.
b = f.__get__(c, C)
print(type(b).__name__)
print(b.__func__ is f)
print(b.__self__ is c)
print(b(10))

# Bound to None it hands back the function itself, unbound.
print(f.__get__(None, C) is f)

# The owner argument is optional.
b2 = f.__get__(c)
print(b2.__self__ is c, b2(20))
