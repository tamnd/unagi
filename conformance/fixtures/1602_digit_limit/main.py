big = 2**20000
try:
    print(big)
except ValueError as e:
    print("p:", e)
try:
    s = f"{big}"
except ValueError as e:
    print("f:", e)
try:
    s = "%d" % big
except ValueError as e:
    print("pct:", e)
try:
    s = str([big])
except ValueError as e:
    print("lst:", e)
try:
    n = int("1" * 4301)
except ValueError as e:
    print("in:", e)
print(len(hex(big)), len(f"{big:x}"), len(bin(big)))
print(len(str(10**4299)))
print(format(2**100, "010,d"), f"{2**100:.2e}")
print("%x" % -2**100, "%o" % 2**100)
try:
    s = "%c" % 2**100
except OverflowError as e:
    print("c:", e)
try:
    s = "%*d" % (2**100, 5)
except OverflowError as e:
    print("w:", e)
try:
    s = "%.*f" % (2**100, 1.5)
except OverflowError as e:
    print("prec:", e)
try:
    s = "abc".zfill(2**100)
except OverflowError as e:
    print("z:", e)
try:
    s = f"{2**11000:.2f}"
except OverflowError as e:
    print("ff:", e)
