def logic(a: int, b: int) -> int:
    r = 0
    if a and b:
        r += 1
    if a or b:
        r += 2
    if not a:
        r += 4
    if a:
        r += 8
    return r


for a in range(0, 2):
    for b in range(0, 2):
        print(a, b, logic(a, b))
