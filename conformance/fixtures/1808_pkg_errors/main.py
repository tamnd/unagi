# Missing dotted names: the error names the first missing prefix, adds the
# not-a-package suffix when the parent is a plain module, and every ancestor
# that does resolve still executes before the raise.
try:
    import missing.x
except ModuleNotFoundError as e:
    print(type(e).__name__, e)

try:
    import pkg.missing
except ModuleNotFoundError as e:
    print(type(e).__name__, e)

try:
    import plain.deep
except ImportError as e:
    print(type(e).__name__, e)

try:
    import pkg.sub.leaf.deeper
except ModuleNotFoundError as e:
    print(type(e).__name__, e)

try:
    from pkg import nothing
except ImportError as e:
    print(type(e).__name__, e)

try:
    from plain import deep
except ImportError as e:
    print(type(e).__name__, e)

try:
    from pkg.sub import gone
except ImportError as e:
    print(type(e).__name__, e)

print(plain.x)
