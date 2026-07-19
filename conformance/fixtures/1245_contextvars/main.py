import contextvars

# A ContextVar with a default returns it before any value is set; an explicit
# argument to get wins over the default.
level = contextvars.ContextVar("level", default=0)
print("default", level.get())
print("default arg", level.get(5))

# set returns a token; get reflects the set; the token's old_value is MISSING
# when the variable held no value under this context before.
name = contextvars.ContextVar("name")
tok = name.set("root")
print("after set", name.get())
print("old is missing", tok.old_value is contextvars.Token.MISSING)
print("token var is name", tok.var is name)

# A second set records the prior value; reset restores it.
tok2 = name.set("child")
print("nested", name.get())
print("old2", tok2.old_value)
name.reset(tok2)
print("after reset", name.get())

# get with no value and no default raises LookupError; a default argument
# stands in when the variable is unset.
missing = contextvars.ContextVar("missing")
try:
    missing.get()
except LookupError:
    print("lookup raised")
print("get default when unset", missing.get("fallback"))

# Context.run isolates changes: a set inside run does not leak to the caller's
# context, but stays visible on the Context object afterwards.
ctx = contextvars.Context()


def inside():
    level.set(42)
    return level.get()


print("inside run", ctx.run(inside))
print("outside after run", level.get())
print("ctx sees it", ctx.get(level))
print("ctx get default", ctx.get(name, "none"))


# run forwards positional and keyword arguments to the callable.
def add(a, b, c=0):
    return a + b + c


print("run args", ctx.run(add, 1, 2, c=3))

# copy_context snapshots the current context; a later edit is not in the copy.
name.set("before-copy")
snap = contextvars.copy_context()
name.set("after-copy")
print("snapshot", snap.get(name))
print("current", name.get())

# reusing a token raises RuntimeError.
t3 = level.set(7)
level.reset(t3)
try:
    level.reset(t3)
except RuntimeError:
    print("reuse raised")

# entering a context that is already active raises RuntimeError.
loop_ctx = contextvars.Context()


def reenter():
    return loop_ctx.run(lambda: "nope")


try:
    loop_ctx.run(reenter)
except RuntimeError:
    print("reenter raised")
