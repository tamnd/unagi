class Base:
    def greet(self, name="world", punct="!"):
        return "hi " + name + punct

class Sub(Base):
    def greet(self, name="world", punct="!"):
        return "[" + super().greet(name=name, punct=punct) + "]"

s = Sub()
print(s.greet())
print(s.greet(name="sam"))
print(s.greet(punct="?"))
print(Base.greet(s, name="via class"))

try:
    s.greet(bogus=1)
except TypeError as e:
    print(e)

try:
    [1].append(x=2)
except TypeError as e:
    print(e)
