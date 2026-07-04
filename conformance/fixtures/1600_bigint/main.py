big = 2**200
print(big)
print(big + 1, big - 1, big * 3, -big)
print(-(-2**63), 2**63, -2**63)
print(9223372036854775807 + 1, -9223372036854775808 - 1)
print(big % 7, (-big) % 7, big % -7, big // 7 * 7 + big % 7 == big)
print(divmod(big, 7), divmod(-big, 7))
print(2**100 * 2**100 == 2**200)
print(abs(-2**100), min(2**100, 2**101), max(-2**100, -2**101))
print(pow(3, -1, 7), pow(2, 3, -5), pow(2**100, 2, 7), pow(2, -3, 7919), pow(2**100, -1, 9))
print(pow(2, -1), 2**-2)
print(1 << 200, 1 >> 200, -1 >> 200, big >> 199)
print(big == 2.0**200, 2**53 + 1 == 2.0**53, 2**53 == 2.0**53)
print(big > 1e59, -big < -1e59, 10**16 / 3)
print(int(2.0**100), int(1e308) == 10**308)
print(float(2**100), round(2**100), round(1, 2**100))
print(round(9 * 10**18, -19), round(-2**100, -3))
print(sum([2**100, 2**100, 1]))
print([big, -big], (big, None))
print([1, 2, 3][-2**100:2**100], "abc".find("b", -2**100, 2**100))
print(hex(-2**100), oct(2**100), bin(2**64))
try:
    float(10**400)
except OverflowError as e:
    print("of:", e)
try:
    0**-1
except ZeroDivisionError as e:
    print("zd:", e)
try:
    1 << 2**100
except OverflowError as e:
    print("sh:", e)
try:
    [1, 2][2**100]
except IndexError as e:
    print("ix:", e)
try:
    "abc" * 2**100
except OverflowError as e:
    print("rep:", e)
try:
    chr(2**100)
except ValueError as e:
    print("chr:", e)
try:
    [].pop(2**100)
except OverflowError as e:
    print("pop:", e)
