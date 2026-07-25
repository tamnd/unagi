# PEP 560: __mro_entries__ is consulted only for a base that is not a type. A
# class base that happens to define __mro_entries__ as a method (the shape
# typing._SpecialForm uses to reject subclassing one of its instances) keeps
# itself as the base rather than being called as an unbound function.


class Base:
    def __mro_entries__(self, bases):
        print("should not run")
        return (object,)


class Sub(Base):
    pass


print([c.__name__ for c in Sub.__mro__])


# A non-type instance used as a base still has __mro_entries__ consulted, and its
# result replaces the base while the original is recorded on __orig_bases__.
class Proxy:
    def __mro_entries__(self, bases):
        return (dict,)


proxy = Proxy()


class UsesProxy(proxy):
    pass


print([c.__name__ for c in UsesProxy.__mro__])
print(UsesProxy.__orig_bases__[0] is proxy)
