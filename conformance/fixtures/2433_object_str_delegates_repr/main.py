# object.__str__ delegates to the object's __repr__, so an explicit
# object.__str__(x) call honors a user __repr__ override and reads a builtin as
# its repr rather than the default object address, matching CPython.
class C:
    def __repr__(self):
        return "custom-repr"


class D:
    def __repr__(self):
        return "d-repr"

    def __str__(self):
        return "d-str"


class E:
    pass


print(object.__str__(C()))
print(object.__str__(D()))
print(object.__str__(5))
print(object.__str__([1, 2, 3]))
print(object.__str__("hi"))
print(object.__str__((1, 2)))
print(str(D()))
print(object.__str__(E()).startswith("<__main__.E object at 0x"))
