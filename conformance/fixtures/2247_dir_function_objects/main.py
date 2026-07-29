# dir() enumerates a callable's attribute directory. A Python function reports the
# function type's attribute set (the tail of which is object's own names) folded
# with any attributes the function grew in its __dict__; a builtin function or
# bound builtin method reports the fixed builtin_function_or_method set. A builtin
# type constructor (int, list) is deliberately not a function here — its dir() is a
# type's, not covered by this slice.

def f(a, b, c=3):
    "doc"
    return a

lam = lambda x: x

# Full sorted directories, exact against CPython.
print("func", sorted(dir(f)))
print("lambda", sorted(dir(lam)))
print("builtin", sorted(dir(len)))
print("bound_str", sorted(dir("".upper)))
print("bound_list", sorted(dir([].append)))

# A function's own __dict__ attributes fold into its directory.
f.tag = 1
f.other = 2
print("with_attrs_has_tag", "tag" in dir(f), "other" in dir(f))
print("attrs_sorted_tail", [n for n in sorted(dir(f)) if not n.startswith("__")])

# Membership of the names introspection tools key on.
print("func has __code__/__defaults__/__globals__:",
      all(n in dir(f) for n in ("__code__", "__defaults__", "__globals__")))
print("builtin has __self__/__text_signature__:",
      all(n in dir(len) for n in ("__self__", "__text_signature__")))
print("builtin lacks __code__:", "__code__" not in dir(len))

# dir() is sorted and duplicate-free.
d = dir(f)
print("sorted_unique", d == sorted(set(d)))
