import array
import warnings


def categories(records):
    return [r.category.__name__ for r in records]


# Building an array with the deprecated 'u' type code records exactly one
# DeprecationWarning carrying the removal message.
with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter("always")
    a = array.array("u", "hi")
    print("u build:", a.tolist(), categories(w))
    print("u message:", str(w[0].message))

# The warning fires before the initializer is validated, so a bad initializer
# still leaves the warning behind and then raises.
with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter("always")
    try:
        array.array("u", 5)
    except Exception as e:
        print("u bad init:", type(e).__name__, categories(w))

# The 'w' replacement code and a plain numeric code stay silent.
with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter("always")
    array.array("w", "abc")
    array.array("i", [1, 2, 3])
    print("w and i:", categories(w))

# A filter that promotes the warning to an error aborts the construction.
with warnings.catch_warnings():
    warnings.simplefilter("error", DeprecationWarning)
    try:
        array.array("u", "x")
        print("promotion: no error")
    except DeprecationWarning as e:
        print("promotion:", str(e))

# The 'u' array otherwise behaves normally once the warning is silenced.
with warnings.catch_warnings():
    warnings.simplefilter("ignore")
    a = array.array("u", "cafe")
    print("tounicode:", a.tounicode())
    print("typecode:", a.typecode, "itemsize:", a.itemsize)
    print("typecodes:", array.typecodes)
