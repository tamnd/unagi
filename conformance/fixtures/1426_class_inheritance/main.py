# A subclass inherits methods and class variables from its base, overrides
# what it redefines, and reaches a base method by naming the base directly.
class Animal:
    kind = "animal"
    legs = 4

    def __init__(self, name):
        self.name = name

    def speak(self):
        return "..."

    def describe(self):
        return self.name + " the " + self.kind + " says " + self.speak()


class Dog(Animal):
    kind = "dog"

    def speak(self):
        return "woof"


class Puppy(Dog):
    def __init__(self, name):
        Animal.__init__(self, name)
        self.small = True

    def speak(self):
        # reach the overridden base method explicitly
        return Dog.speak(self) + " (tiny)"


a = Animal("Thing")
print(a.describe())
print(a.legs)

d = Dog("Rex")
print(d.describe())
print(d.kind, d.legs)

p = Puppy("Bo")
print(p.describe())
print(p.small)
print(p.legs)

# an inherited class variable is read through the instance and the subclass
print(Dog.kind, Puppy.kind)
print(Puppy.legs)
