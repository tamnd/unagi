# CPython 3.14 deprecates bitwise inversion of a bool: ~b still returns the
# bitwise inversion of the underlying int, but warns that this is rarely what a
# ~ on a bool is meant to do. Captured through the warnings machinery so the
# category, message, and the correct int result are all observable.
import warnings

with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter("always")
    r_false = ~False
    r_true = ~True
    x = True
    r_var = ~x

print(r_false, r_true, r_var)
print(len(caught))
for w in caught:
    print(w.category.__name__, str(w.message))

# assertWarns-style: the warning fires as a DeprecationWarning.
with warnings.catch_warnings():
    warnings.simplefilter("error", DeprecationWarning)
    try:
        ~False
    except DeprecationWarning as e:
        print("promoted:", type(e).__name__)

# A plain int inversion warns nothing and spells the integer verbatim.
with warnings.catch_warnings(record=True) as caught2:
    warnings.simplefilter("always")
    print(~5, ~0, len(caught2))
