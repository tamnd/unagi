# float.__getformat__(typestr) is the classmethod test.support keys its
# requires_IEEE_754 decorator on: it reports the host storage format of a C
# double or C float, which on every supported platform is an IEEE 754 value in
# the machine's byte order. Both type arguments answer the same order; anything
# else is a ValueError, wrong arity a TypeError. Read off the type, off a bound
# reference, and through getattr — all resolve to the one classmethod.

fmt = float.__getformat__("double")
print("double", fmt)
print("float", float.__getformat__("float"))
print("agree", float.__getformat__("double") == float.__getformat__("float"))
print("is IEEE", fmt.startswith("IEEE, ") and fmt.endswith("-endian"))

# A bound reference and a getattr lookup are the same classmethod.
m = float.__getformat__
print("bound", m("double") == fmt)
print("getattr", getattr(float, "__getformat__")("float") == fmt)

# Bad type string is a ValueError with the exact message.
try:
    float.__getformat__("quad")
except ValueError as e:
    print("ValueError", e)

# Wrong arity is a TypeError reporting the count given.
try:
    float.__getformat__()
except TypeError as e:
    print("TypeError0", e)
try:
    float.__getformat__("double", "float")
except TypeError as e:
    print("TypeError2", e)
