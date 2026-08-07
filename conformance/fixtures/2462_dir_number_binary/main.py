# dir() of a numbers or binary-data builtin value reports the fixed attribute
# set its type carries, the sorted list CPython builds by walking the type's
# MRO. bool derives from int and adds nothing, a big int is still an int, and
# every listed name resolves through getattr so hasattr agrees with dir().
for label, value in [
    ("int", 7),
    ("bigint", 10 ** 80),
    ("float", 1.5),
    ("complex", 1 + 2j),
    ("bool", True),
    ("bytes", b"abc"),
    ("bytearray", bytearray(b"abc")),
    ("memoryview", memoryview(b"abc")),
]:
    names = dir(value)
    print(label, len(names), "sorted" if names == sorted(names) else "UNSORTED")

print("bool-eq-int", dir(True) == dir(int()))
print("bigint-eq-int", dir(10 ** 80) == dir(0))

# a spot check that the listed names are the real attribute surface
print("int-attrs", "bit_length" in dir(5), "to_bytes" in dir(5), "__index__" in dir(5))
print("float-attrs", "hex" in dir(1.5), "is_integer" in dir(1.5), "fromhex" in dir(1.5))
print("complex-attrs", "conjugate" in dir(1j), "real" in dir(1j), "imag" in dir(1j))
print("bytes-attrs", "hex" in dir(b""), "decode" in dir(b""), "fromhex" in dir(b""))
print("bytearray-attrs", "append" in dir(bytearray()), "insert" in dir(bytearray()))
print("mv-attrs", "cast" in dir(memoryview(b"")), "suboffsets" in dir(memoryview(b"")))

# dir() folds in nothing extra for a plain value: no per-instance dict
print("no-dict-int", "__dict__" in dir(5))
