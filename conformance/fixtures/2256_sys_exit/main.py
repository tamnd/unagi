import sys

# sys.exit raises SystemExit carrying the code verbatim; catching it exposes
# .code, which is None when no argument is given.
for arg in ((), (0,), (3,), ("bye",), (None,)):
    try:
        sys.exit(*arg)
    except SystemExit as e:
        print(arg, "->", repr(e.code))

# It accepts at most one argument.
try:
    sys.exit(1, 2)
except TypeError as e:
    print("arity:", e)

# A bare sys.exit(code) at module scope drives the process exit status.
sys.exit(3)
