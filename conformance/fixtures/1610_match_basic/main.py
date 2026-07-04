def classify(v):
    match v:
        case 0:
            return "zero"
        case 1 | 2 | 3:
            return "small"
        case None:
            return "none"
        case True:
            return "true"
        case False:
            return "false"
        case "hi":
            return "greeting"
        case 3.5:
            return "pi-ish"
        case x:
            return ("capture", x)

for v in [0, 2, None, True, False, "hi", 3.5, 99, "other"]:
    print(v, "->", classify(v))

# identity vs equality: 1 must not match True, 0 must not match False
match 1:
    case True:
        print("1 matched True (wrong)")
    case 1:
        print("1 is 1")
match 0:
    case False:
        print("0 matched False (wrong)")
    case 0:
        print("0 is 0")

# single case, no fallthrough
match 42:
    case 42:
        print("forty-two")

# wildcard only
match ["anything", 1, 2]:
    case _:
        print("wildcard")
