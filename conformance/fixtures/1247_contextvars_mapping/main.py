import contextvars

a = contextvars.ContextVar("a")
b = contextvars.ContextVar("b", default=99)
c = contextvars.ContextVar("c")

ctx = contextvars.copy_context()

def setup():
    a.set(1)
    b.set(2)

ctx.run(setup)

print("len", len(ctx))
print("a in", a in ctx)
print("c in", c in ctx)
print("get a", ctx[a])
print("get b", ctx[b])
print("get default via method", ctx.get(c, "fallback"))

print("keys", sorted(k.name for k in ctx.keys()))
print("values", sorted(ctx.values()))
print("items", sorted((k.name, v) for k, v in ctx.items()))
print("iter", sorted(k.name for k in ctx))
print("len keys", len(ctx.keys()), "len values", len(ctx.values()), "len items", len(ctx.items()))
print("a in keys", a in ctx.keys())
print("2 in values", 2 in ctx.values())

try:
    ctx[c]
except KeyError:
    print("KeyError on unset")

try:
    ctx["notavar"]
except TypeError as e:
    print("TypeError bad key:", e)

try:
    "notavar" in ctx
except TypeError as e:
    print("TypeError bad in:", e)

# empty context
empty = contextvars.Context()
print("empty len", len(empty), "empty keys", list(empty.keys()))
