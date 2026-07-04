def pick(v):
    match v:
        case [x] | (x, _):
            return ("or-seq", x)
        case ("a" | "b" | "c") as letter:
            return ("letter", letter)
        case _:
            return "none"

print(pick([7]))
print(pick((8, 9)))
print(pick("b"))
print(pick("z"))

# as binds the whole subject after the inner match
match [1, 2, 3]:
    case [1, *_] as whole:
        print("as-whole", whole)

# or with the same capture from either branch
def first_num(v):
    match v:
        case [n] | [n, _] | [n, _, _]:
            return n
    return None

print(first_num([5]))
print(first_num([6, 7]))
print(first_num([8, 9, 10]))
print(first_num([1, 2, 3, 4]))

# nested as inside a sequence
match [10, 20]:
    case [a, (20 as b)]:
        print("nested-as", a, b)
