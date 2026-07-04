class Temperature:
    scale = "celsius"

    def __init__(self, celsius):
        self._celsius = celsius

    @property
    def celsius(self):
        return self._celsius

    @celsius.setter
    def celsius(self, value):
        self._celsius = value

    @property
    def fahrenheit(self):
        return self._celsius * 9 / 5 + 32

    @staticmethod
    def freezing():
        return 0

    @classmethod
    def describe(cls):
        return cls.scale


class Kelvin(Temperature):
    scale = "kelvin"


t = Temperature(25)
print(t.celsius)
print(t.fahrenheit)
t.celsius = 30
print(t.celsius)
print(t.fahrenheit)

print(Temperature.freezing())
print(t.freezing())

print(Temperature.describe())
print(t.describe())
print(Kelvin(0).describe())

try:
    t.fahrenheit = 100
except AttributeError as e:
    print("error:", e)

p = property()
print(p.fget)
