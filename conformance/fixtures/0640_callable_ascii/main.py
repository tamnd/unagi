class C:
    def __call__(self):
        return "called"


class D:
    pass


def fn():
    pass


# callable is True for functions, builtins, classes and instances with
# __call__, and False for plain instances and non-callables.
print(callable(fn), callable(len), callable(C), callable(C()))
print(callable(D()), callable(1), callable("s"), callable(lambda: 0))

# A callable instance really calls.
print(C()())

# ascii escapes every non-ASCII rune of the repr, ASCII passes through.
print(ascii("héllo"), ascii("a"))
print(ascii(["x", "ü"]), ascii({"k": "café"}))
print(ascii(123), ascii("\n\t"))
print(ascii("€"), ascii("Ā"))

# Passed around as values.
c, a = callable, ascii
print(c(fn), a("é"))
