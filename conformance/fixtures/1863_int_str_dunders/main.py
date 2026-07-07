# int and str answer their string dunder methods off the type object, with the
# own-versus-inherited split EnumType.__new__ reads when it borrows a member
# type's __repr__, __str__, and __format__ onto the enum class.

# int defines its own __repr__ and __format__ and inherits object's __str__ and
# __reduce_ex__.
print(int.__str__ is object.__str__)
print(int.__repr__ is not object.__repr__)
print(int.__format__ is not object.__format__)
print(int.__reduce_ex__ is object.__reduce_ex__)

# str defines its own __repr__, __str__, and __format__ and inherits __reduce_ex__.
print(str.__str__ is not object.__str__)
print(str.__repr__ is not object.__repr__)
print(str.__format__ is not object.__format__)
print(str.__reduce_ex__ is object.__reduce_ex__)

# Each own method reads back as one stable object.
print(int.__repr__ is int.__repr__)
print(str.__format__ is str.__format__)

# Calling them produces the type's result on the underlying value.
print(int.__repr__(5))
print(int.__format__(5, ""))
print(int.__format__(255, "x"))
print(str.__repr__("a"))
print(str.__str__("a"))
print(str.__format__("hi", ">5"))
