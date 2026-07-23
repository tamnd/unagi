# A function-local class whose methods read the enclosing function's variables,
# the shape inspect.RewriteSymbolics uses when its visit methods call a sibling
# nested def. Each method captures the enclosing binding by reference, so a
# second call with a different closure sees its own values while an instance
# built earlier keeps the binding it captured.


def build(prefix):
    def wrap(v):
        return prefix + ":" + str(v)

    class Handler:
        tag = "h"

        def visit(self, node):
            return wrap(node)

        def double(self, n):
            return wrap(n) + "/" + wrap(n)

    return Handler


H = build("p")
h = H()
print(H.__qualname__)
print(H.visit.__qualname__)
print(h.tag)
print(h.visit(5))
print(h.double("z"))

H2 = build("q")
print(H2().visit(1))
print(h.visit(1))
