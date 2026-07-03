n = 42
neg = -7
big = 1234567
pi = 3.14159
s = "hi"
print(f"[{n:>6}] [{n:<6}] [{n:^6}]")
print(f"[{n:*>6}] [{n:*<6}] [{n:*^7}]")
print(f"[{n:06}] [{neg:06}] [{neg:=6}]")
print(f"[{n:+d}] [{neg:+d}] [{n: d}]")
print(f"[{n:x}] [{n:X}] [{n:#x}] [{n:o}] [{n:#o}] [{n:b}] [{n:#b}]")
print(f"[{big:,}] [{big:_}] [{big:15,}]")
print(f"[{n:c}]")
print(f"[{pi:.2f}] [{pi:10.3f}] [{pi:e}] [{pi:E}] [{pi:g}] [{pi:.0f}] [{pi:%}]")
print(f"[{n:.2f}] [{n:e}]")
print(f"[{s:>6}] [{s:<6}|] [{s:^6}] [{s:.1}] [{s:*^6}]")
print(f"[{True}] [{True:d}] [{True:>6}] [{None}]")
print(f"[{s!r:>6}]")
try:
    print(f"{s:d}")
except ValueError as e:
    print("caught", str(e))
try:
    print(f"{None:d}")
except TypeError as e:
    print("caught", str(e))
try:
    print(f"{s:,}")
except ValueError as e:
    print("caught", str(e))
