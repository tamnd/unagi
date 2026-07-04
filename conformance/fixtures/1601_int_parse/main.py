print(int("  +1_2_3  "), int("-0"), int("００７"))
print(int("0x_1f", 0), int("0b101", 0), int("0o17", 0), int("00", 0))
print(int("ff", 16), int("0xff", 16), int("z", 36), int("12", base=8), int("019"))
print(int("99999999999999999999"), int("-99999999999999999999", 10))
print(float(" 1_000.5 "), float("infinity"), float("-InF"), float("1e309"), float("nan"))
print(float("１２.５"))
for bad in ["", "1__2", "_12", "12_", "1.5", "0x1f"]:
    try:
        int(bad)
    except ValueError as e:
        print("ve:", e)
try:
    int("019", 0)
except ValueError as e:
    print("ve0:", e)
try:
    int("12", 1)
except ValueError as e:
    print("base:", e)
try:
    int("12", 2**100)
except ValueError as e:
    print("base2:", e)
try:
    int(1.5, 16)
except TypeError as e:
    print("te:", e)
try:
    int([1])
except TypeError as e:
    print("te2:", e)
try:
    int(base=10)
except TypeError as e:
    print("kw:", e)
try:
    int(x="12")
except TypeError as e:
    print("kw2:", e)
try:
    int("12", base=None)
except TypeError as e:
    print("kw3:", e)
try:
    float("x")
except ValueError as e:
    print("fe:", e)
try:
    float([1])
except TypeError as e:
    print("fe2:", e)
