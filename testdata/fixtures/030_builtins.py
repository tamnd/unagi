print(min(3, 1, 2), max(3, 1, 2))
print(min([5, 2, 9]), max("abc"))
print(min(2, 2.0), max(1, 1.0))
print(sum([1, 2, 3]), sum([1, 2], 10), sum([0.5, 0.25]))
print(sum([[1], [2]], []))
print(round(2.5), round(1.5), round(0.5), round(-0.5))
print(round(2.675, 2), round(3.14159, 3), round(2.0))
print(round(150, -2), round(250, -2), round(1234, -2), round(7, 2))
print(divmod(7, 3), divmod(-7, 3), divmod(7.5, 2))
print(pow(2, 10), pow(2, -1), pow(3, 4, 5), pow(3, -1, 7))
print(bin(10), oct(64), hex(255), bin(-5), hex(-255), bin(True))
print(ord("a"), ord("é"), chr(97), chr(233), chr(128512))
try:
    min([])
except ValueError as e:
    print("caught", e)
try:
    sum(["a", "b"])
except TypeError as e:
    print("caught", e)
try:
    chr(-1)
except ValueError as e:
    print("caught", e)
try:
    ord("ab")
except TypeError as e:
    print("caught", e)
try:
    pow(2, 3, 0)
except ValueError as e:
    print("caught", e)
