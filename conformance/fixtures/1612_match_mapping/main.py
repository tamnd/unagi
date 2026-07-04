def route(v):
    match v:
        case {}:
            return "empty-or-any-dict"

for v in [{}, {"a": 1}]:
    print(route(v))

# keyed values, extra keys ignored
match {"type": "point", "x": 1, "y": 2}:
    case {"type": "point", "x": x, "y": y}:
        print("point", x, y)

# **rest captures the leftovers, preserving insertion order
match {"a": 1, "b": 2, "c": 3, "d": 4}:
    case {"a": a, **rest}:
        print("rest", a, rest)

# missing key falls through
match {"a": 1}:
    case {"a": a, "b": b}:
        print("has both (wrong)")
    case {"a": a}:
        print("only a", a)

# value sub-pattern must match too
match {"k": [1, 2]}:
    case {"k": [x, y]}:
        print("nested value", x, y)

# non-mapping subject does not match a mapping pattern
match [1, 2]:
    case {"a": a}:
        print("list matched map (wrong)")
    case _:
        print("list is not a mapping")

# literal value patterns as mapping values
match {"status": "ok", "code": 200}:
    case {"status": "ok", "code": 200}:
        print("ok 200")
    case _:
        print("other")
