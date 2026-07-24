# A method read off a builtin type is the unbound method, so int.bit_length then
# int.bit_length(5) dispatches the same as (5).bit_length(). statistics leans on
# _nbits = int.bit_length exactly this way.
print(int.bit_length(5))
print(int.bit_length(255))
nbits = int.bit_length
print(nbits(1023), nbits(0))

# The read binds nothing, the instance is the first call argument.
print(str.upper("ab"))
print(str.split("a,b,c", ","))
print(str.replace("banana", "a", "o"))
print(bytes.hex(b"\xde\xad"))
print(bytearray.hex(bytearray(b"\x01\x02")))

# The container types read back their methods off the type too.
print(list.count([1, 1, 2, 1], 1))
print(list.index([9, 8, 7], 7))
print(tuple.count((1, 2, 2, 2), 2))
print(dict.get({"a": 1}, "a"), dict.get({"a": 1}, "z", -1))
print(sorted(set.union({1, 2}, {3})))
print(sorted(frozenset.intersection(frozenset({1, 2, 3}), {2, 3})))

# bool shares int's methods, float carries its own.
print(bool.bit_length(True))
print(float.is_integer(3.0), float.is_integer(3.5))

# The classmethod and staticmethod forms keep routing as classmethods, not as
# unbound instance methods on the first argument.
print(dict.fromkeys(["a", "b"], 0))
print(int.from_bytes(b"\x01\x00", "big"))

# A missing first argument and a wrong first-argument type are the descriptor's
# own TypeErrors.
try:
    int.bit_length()
except TypeError as e:
    print("noarg:", e)
try:
    str.upper(5)
except TypeError as e:
    print("wrongtype:", e)

# hasattr agrees, an unknown type method is still AttributeError.
print(hasattr(int, "bit_length"), hasattr(int, "nope_method"))
print(callable(int.bit_length))
