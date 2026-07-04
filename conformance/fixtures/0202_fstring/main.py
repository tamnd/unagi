name = "world"
n = 7
print(f"hello {name}")
print(f"{n} squared is {n * n}")
print(f"{name!r} and {name!s}")
print(f"{{braces}} {n}")
print(f"{n=}")
print(f"{n = }")
print(f"{n=:>4}")
print("plain " f"and {n}" " tail")
print(f"{'a' + 'b'}")
print(f"same quote {"inner"}")
print(f"{(w := 3)} then {w}")
print(f"{'héllo'!a}")
xs = [1, 2]
print(f"list {xs} len {len(xs)}")
print(f"cond {'yes' if n > 5 else 'no'}")
