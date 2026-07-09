def bits(a: int, b: int) -> int:
    return (a & b) + (a | b) + (a ^ b) + (a << 1) + (a >> 1)


for a in range(0, 8):
    for b in range(0, 4):
        print(a, b, bits(a, b))
