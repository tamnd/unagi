# Built-in exception types read as first-class class values: they print as
# classes, expose __name__ and __bases__, answer issubclass and isinstance, and
# report their own class through type().
print(ValueError)
print(Exception)
print(BaseException)
print(ValueError.__name__)
print(ValueError.__bases__)
print(Exception.__bases__)
print(BaseException.__bases__)

print(issubclass(ValueError, Exception))
print(issubclass(ValueError, BaseException))
print(issubclass(Exception, ValueError))
print(issubclass(KeyError, LookupError))
print(issubclass(IndexError, LookupError))
print(issubclass(ZeroDivisionError, ArithmeticError))

# The two OSError aliases resolve to the OSError class object itself.
print(IOError is OSError)
print(EnvironmentError is OSError)
print(OSError)

# A built-in exception class passes around like any other value.
x = ValueError
print(x is ValueError)
alias = {"err": TypeError}
print(alias["err"] is TypeError)

try:
    raise ValueError("boom")
except Exception as e:
    print(isinstance(e, ValueError))
    print(isinstance(e, TypeError))
    print(isinstance(e, (TypeError, ValueError)))
    print(isinstance(e, Exception))
    print(isinstance(e, BaseException))
    print(type(e) is ValueError)
    print(type(e))
    print(type(e).__name__)

try:
    raise KeyError("missing")
except LookupError as e:
    print("caught via base", isinstance(e, KeyError))
