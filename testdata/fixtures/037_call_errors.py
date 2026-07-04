# Call-binding failures raise catchable TypeErrors with CPython wording.

def f(a, b, c):
    return a + b + c

def g(a, b=2, /, c=3, *, k, m=5):
    return a + b + c + k + m

def h(a, b, /):
    return a + b

def keywordish(color, number=1):
    return color

try:
    f(1)
except TypeError as e:
    print(e)
try:
    f()
except TypeError as e:
    print(e)
try:
    f(1, 2, 3, 4)
except TypeError as e:
    print(e)
try:
    g(1, 2, 3, 4, k=1)
except TypeError as e:
    print(e)
try:
    g(1)
except TypeError as e:
    print(e)
try:
    f(1, 2, a=3)
except TypeError as e:
    print(e)
try:
    f(1, 2, 3, d=4)
except TypeError as e:
    print(e)
try:
    keywordish("red", numbre=2)
except TypeError as e:
    print(e)
try:
    h(a=1, b=2)
except TypeError as e:
    print(e)
try:
    h(1, b=2)
except TypeError as e:
    print(e)

# Arguments still evaluate before the binding failure raises.
def loud(tag, v):
    print("eval", tag)
    return v

def one(a):
    return a

try:
    one(loud("first", 1), loud("second", 2))
except TypeError as e:
    print(e)
