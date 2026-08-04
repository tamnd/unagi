# str.format_map renders a template against a mapping passed directly, so a
# custom __getitem__ or __missing__ fires and a positional field is rejected
# since format_map takes no positional args.
from collections import defaultdict


class Missing(dict):
    def __missing__(self, key):
        return "<" + key + ">"


# A plain dict drives named fields, including a path off a mapped value.
print("{name} is {age}".format_map({"name": "grid", "age": 5}))
print("{pt[0]}/{pt[1]}".format_map({"pt": [10, 20]}))
print("{z.real}:{z.imag}".format_map({"z": 3 + 4j}))
print("{v:{w}}".format_map({"v": 42, "w": ">6"}))
print("{v!r}".format_map({"v": "hi"}))
print("{{literal}} {k}".format_map({"k": 1}))

# A mapping with __missing__ fills absent keys; defaultdict does the same.
print("{a}{b}".format_map(Missing(a=1)))
print("{x}".format_map(defaultdict(lambda: "D")))

# No fields means the mapping is never consulted.
print("plain".format_map("not a mapping"))

# Bound-method read hands back the same callable a direct call dispatches.
m = "{k}".format_map
print(m({"k": 7}), callable("x".format_map))

# Error paths match CPython's wording.
for label, thunk in [
    ("positional auto", lambda: "{}".format_map({})),
    ("positional num", lambda: "{0}".format_map({0: "z"})),
    ("missing key", lambda: "{nope}".format_map({})),
    ("list mapping", lambda: "{k}".format_map([1, 2])),
    ("arity zero", lambda: "x".format_map()),
]:
    try:
        print(label, "->", thunk())
    except Exception as e:
        print(label, "RAISES", type(e).__name__, str(e))
