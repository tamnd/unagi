# A function and a lambda are callable, so each carries a __call__ that forwards
# to it. What library code leans on is that getattr(fn, '__call__', None) is not
# None: callable() answers from it, and unittest.mock._callable uses it to tell a
# plain function apart from a non-callable when dispatching a mock side_effect.


def add(a, b=3):
    return a + b


square = lambda x: x * x


class C:
    def method(self, n):
        return n + 1


print("func has __call__:", getattr(add, "__call__", None) is not None)
print("lambda has __call__:", getattr(square, "__call__", None) is not None)

# Calling through __call__ matches calling the function directly, keywords too.
print("add.__call__(4):", add.__call__(4))
print("add.__call__(4, b=10):", add.__call__(4, b=10))
print("square.__call__(6):", square.__call__(6))

# A bound method is callable in the same way.
c = C()
print("method has __call__:", getattr(c.method, "__call__", None) is not None)
print("c.method.__call__(41):", c.method.__call__(41))

# callable() agrees, which is the observable users care about.
print("callable(add):", callable(add))
print("callable(square):", callable(square))
print("callable(c.method):", callable(c.method))
