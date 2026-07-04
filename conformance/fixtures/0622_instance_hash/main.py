class Ident:
    pass

a = Ident()
b = Ident()
print(hash(a) == hash(a))
d = {a: 1, b: 2}
print(len(d))
print(d[a], d[b])

class Fixed:
    def __hash__(self):
        return 42

print(hash(Fixed()))

class NegHash:
    def __hash__(self):
        return -1

print(hash(NegHash()))

class Both:
    def __eq__(self, other):
        return True
    def __hash__(self):
        return 7

d2 = {Both(): 1}
d2[Both()] = 2
print(len(d2), d2[Both()])
print(Both() in {Both()})

class EqOnly:
    def __eq__(self, other):
        return True

try:
    hash(EqOnly())
except TypeError as e:
    print(e)
try:
    d3 = {EqOnly(): 1}
except TypeError as e:
    print(e)
try:
    s = {EqOnly()}
except TypeError as e:
    print(e)

class NoneHash:
    __hash__ = None

try:
    hash(NoneHash())
except TypeError as e:
    print(e)

class BadHash:
    def __hash__(self):
        return "x"

try:
    hash(BadHash())
except TypeError as e:
    print(e)
