# guard runs after binding; a failed guard keeps the binding
def grade(n):
    match n:
        case x if x < 0:
            return "negative"
        case x if x == 0:
            return "zero"
        case x if x < 10:
            return "small"
        case _:
            return "big"

for n in [-5, 0, 3, 100]:
    print(n, grade(n))

# guard failure leaks the binding, matching CPython
g = "old"
match 5:
    case g if g > 100:
        pass
print("guard leak:", g)

# a partial structural match must NOT leak the earlier capture
k = "old"
match [7, 8]:
    case [k, 999]:
        pass
print("no leak on partial:", k)

# nested partial match leaves nothing bound
p = "old"
q = "old"
match [1, [2, 3]]:
    case [p, [q, 999]]:
        pass
print("nested no leak:", p, q)

# break, continue, and return inside a case body target the enclosing loop
def scan(items):
    out = []
    for it in items:
        match it:
            case 0:
                continue
            case -1:
                break
            case n:
                out.append(n)
    return out

print(scan([1, 0, 2, -1, 3]))

# return from inside a match inside a function
def find_two(v):
    match v:
        case [_, second, *_]:
            return second
    return None

print(find_two([1, 2, 3]))
print(find_two([1]))
