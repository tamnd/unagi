# _contextvars is the accelerator the public contextvars package imports its
# types from, and modules that skip the package reach for it directly. It
# carries the same surface as contextvars: ContextVar with get/set/reset,
# copy_context, Context and Token.
import _contextvars
import contextvars

v = _contextvars.ContextVar('warnings_context')
try:
    v.get()
except LookupError:
    print("no default")

tok = v.set(42)
print(v.get())
v.reset(tok)
print("after reset:", v.get(1))

d = _contextvars.ContextVar('d', default='x')
print(d.get())
print(d.name)

print(_contextvars.Context.__name__)
print(callable(_contextvars.copy_context))
print(callable(_contextvars.Context))

# The public package exposes the same names.
w = contextvars.ContextVar('w', default=0)
print(w.get())
