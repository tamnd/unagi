# isinstance walks the instance's MRO and issubclass walks the class's, so an
# ancestor several levels up still matches and a sibling class does not. The
# tuple second argument is an "any of these" check, and a non-class second
# argument raises the TypeError the builtins spell.
class Animal:
    pass


class Dog(Animal):
    pass


class Cat(Animal):
    pass


class Puppy(Dog):
    pass


d = Puppy()
print(isinstance(d, Puppy))
print(isinstance(d, Dog))
print(isinstance(d, Animal))
print(isinstance(d, Cat))
print(isinstance(d, (Cat, Dog)))
print(isinstance(d, (Cat, Animal)))

print(issubclass(Puppy, Dog))
print(issubclass(Puppy, Animal))
print(issubclass(Dog, Cat))
print(issubclass(Animal, Dog))
print(issubclass(Puppy, (Cat, Animal)))

try:
    isinstance(d, 5)
except TypeError as e:
    print(e)

try:
    isinstance(d, (Cat, 5))
except TypeError as e:
    print(e)

try:
    issubclass(5, Dog)
except TypeError as e:
    print(e)

try:
    issubclass(Dog, 5)
except TypeError as e:
    print(e)
