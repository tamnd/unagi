def describe(v):
    match v:
        case []:
            return "empty"
        case [a]:
            return ("one", a)
        case [a, b]:
            return ("two", a, b)
        case [first, *rest]:
            return ("head-tail", first, rest)

for v in [[], [1], [1, 2], [1, 2, 3, 4]]:
    print(describe(v))

# tuple subject matches list pattern, both are sequences
match (10, 20):
    case [a, b]:
        print("tuple->seq", a, b)

# star in the middle
match [1, 2, 3, 4, 5]:
    case [head, *mid, tail]:
        print("mid", head, mid, tail)

# range is a sequence
match range(3):
    case [a, b, c]:
        print("range", a, b, c)

# str is NOT a sequence pattern subject
match "ab":
    case [x, y]:
        print("str as seq (wrong)")
    case _:
        print("str not a sequence")

# nested sequences
match [1, [2, 3], 4]:
    case [a, [b, c], d]:
        print("nested", a, b, c, d)

# length mismatch falls through
match [1, 2, 3]:
    case [a, b]:
        print("two (wrong)")
    case _:
        print("no two")

# star can match zero elements
match [9]:
    case [only, *tail]:
        print("star-zero", only, tail)
