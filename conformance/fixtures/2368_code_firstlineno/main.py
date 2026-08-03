# __code__.co_firstlineno is the source line the def header sits on. The values
# below are line numbers inside this file, so they are the same wherever the file
# is compiled.


def module_func(a, b):
    return a + b


class C:
    def method(self):
        return 1


square = lambda n: n * n


def outer():
    def inner():
        return 3
    return inner


print("module_func:", module_func.__code__.co_firstlineno)
print("method:", C.method.__code__.co_firstlineno)
print("lambda:", square.__code__.co_firstlineno)
print("nested inner:", outer().__code__.co_firstlineno)

# A method reached through an instance reports the same firstlineno as the one
# reached through the class, since both share the one function object.
print("bound method:", C().method.__code__.co_firstlineno)

# co_firstlineno is a read-only code attribute, so assigning to it raises.
try:
    module_func.__code__.co_firstlineno = 999
except (AttributeError, TypeError) as e:
    print("read-only:", type(e).__name__)
