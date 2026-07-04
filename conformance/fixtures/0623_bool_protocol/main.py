class Flag:
    def __init__(self, v):
        self.v = v
    def __bool__(self):
        return self.v
    def __repr__(self):
        return f"Flag({self.v})"

class Sized:
    def __init__(self, n):
        self.n = n
    def __len__(self):
        return self.n
    def __repr__(self):
        return f"Sized({self.n})"

# if / else consulting __bool__
print("yes" if Flag(True) else "no")
print("yes" if Flag(False) else "no")

# __len__ truthiness
print("t" if Sized(0) else "f")
print("t" if Sized(4) else "f")

# not
print(not Flag(True), not Flag(False))
print(not Sized(0), not Sized(2))

# bool()
print(bool(Flag(True)), bool(Flag(False)), bool(Sized(0)), bool(Sized(1)))

# and / or return the operand, truth via __bool__
print(Flag(False) or Flag(True))
print(Flag(True) and Flag(False))
print(Flag(True) and Sized(3) or Flag(False))

# plain if statement
if Flag(True):
    print("if-taken")
if Sized(0):
    print("not-printed")
else:
    print("else-taken")

# while loop driven by __bool__
class Countdown:
    def __init__(self, n):
        self.n = n
    def __bool__(self):
        return self.n > 0

c = Countdown(3)
seq = []
while c:
    seq.append(c.n)
    c.n -= 1
print(seq)

# comprehension filter
vals = [Flag(True), Flag(False), Flag(True)]
print(len([1 for x in vals if x]))

# assert
assert Flag(True)
try:
    assert Flag(False), "boom"
except AssertionError as e:
    print("assert:", e)

# match guard consulting __bool__
def classify(x, gate):
    match x:
        case n if gate:
            return "gated"
        case _:
            return "fallthrough"

print(classify(1, Flag(True)))
print(classify(1, Flag(False)))

# error paths
class BadBool:
    def __bool__(self):
        return 1
try:
    bool(BadBool())
except TypeError as e:
    print("badbool:", e)

class NegLen:
    def __len__(self):
        return -1
try:
    if NegLen():
        pass
except ValueError as e:
    print("neglen:", e)

class StrLen:
    def __len__(self):
        return "x"
try:
    bool(StrLen())
except TypeError as e:
    print("strlen:", e)

# __bool__ wins over __len__
class Both:
    def __bool__(self):
        return True
    def __len__(self):
        return 0
print(bool(Both()))
