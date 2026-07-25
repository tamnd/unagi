class C:
    def __complex__(self): return 1+2j
class F:
    def __float__(self): return 3.5
class I:
    def __index__(self): return 7
class Bad:
    def __complex__(self): return "x"
class Non:
    pass

print(complex(C()))
print(complex(F()))
print(complex(I()))
try:
    complex(Bad())
except TypeError as e:
    print("TypeError:", e)
try:
    complex(Non())
except TypeError as e:
    print("TypeError:", e)
