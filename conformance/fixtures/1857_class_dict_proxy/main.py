class C:
    x = 1
    y = "two"

    def method(self):
        return self.x


# The class namespace is a read-only mappingproxy.
print(type(C.__dict__).__name__)
print(C.__dict__["x"], C.__dict__["y"])
print("x" in C.__dict__)
print("missing" in C.__dict__)
print(C.__dict__.get("x"), C.__dict__.get("nope", -1))

# A method read back through the proxy is the plain function; call it explicitly.
print(C.__dict__["method"](C()))

# The user-defined names survive a copy of the namespace.
snapshot = C.__dict__.copy()
print(snapshot["x"], snapshot["y"])
print(sorted(k for k in C.__dict__ if not k.startswith("__")))

# A write through the proxy is refused.
try:
    C.__dict__["z"] = 9
except TypeError as e:
    print("TypeError:", e)

# A subclass reports its own namespace, not the base's.
class D(C):
    z = 3

print("z" in D.__dict__, "x" in D.__dict__)
print(D.__dict__["z"])
