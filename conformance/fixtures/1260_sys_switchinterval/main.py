import sys

# a fresh interpreter reports CPython's 5ms default
print(sys.getswitchinterval())

# setting a value round-trips through the getter
sys.setswitchinterval(0.010)
print(sys.getswitchinterval())

# an int argument coerces to a float
sys.setswitchinterval(1)
print(sys.getswitchinterval())

# True is an int, so it stores as 1.0
sys.setswitchinterval(True)
print(sys.getswitchinterval())

# zero and negatives are rejected, the stored value is untouched
try:
    sys.setswitchinterval(0)
except ValueError as e:
    print("ve", e)
try:
    sys.setswitchinterval(-1.5)
except ValueError as e:
    print("ve", e)
print(sys.getswitchinterval())

# a non-number is a TypeError
try:
    sys.setswitchinterval("x")
except TypeError as e:
    print("te", e)
