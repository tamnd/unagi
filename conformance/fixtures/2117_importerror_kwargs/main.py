# The ImportError family accepts name and path keyword arguments and exposes
# them as instance attributes that default to None, the shape importlib's
# bootstrap relies on when it raises and re-reads a failed import. Every other
# builtin exception still takes no keyword arguments.

e = ImportError("boom", name="foo", path="/tmp/foo.py")
print(e.args, e.name, e.path)

# Unset name and path read back as None rather than raising.
plain = ImportError("plain")
print(plain.args, plain.name, plain.path)

# A bare ImportError still defaults both attributes.
bare = ImportError()
print(bare.args, bare.name, bare.path)

# ModuleNotFoundError inherits the keyword handling from ImportError.
m = ModuleNotFoundError("gone", name="mod")
print(m.args, m.name, m.path, isinstance(m, ImportError))

# The attributes survive a raise and an except binding.
try:
    raise ImportError("late", name="pkg.sub", path="/x")
except ImportError as exc:
    print(exc.name, exc.path)

# A non-ImportError builtin exception rejects any keyword argument.
try:
    ValueError("x", foo=1)
except TypeError as t:
    print(t)

# An unexpected keyword on ImportError itself is a TypeError too.
try:
    ImportError("x", bogus=1)
except TypeError as t:
    print(t)
