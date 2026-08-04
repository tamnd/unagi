import array
import pickle
import warnings

# The 'u' type code is deprecated in 3.14; silence the warning so the transcript
# stays about the pickle round-trip rather than the deprecation.
warnings.simplefilter("ignore")

# array.array pickles through array.__reduce_ex__: below protocol 3 the array type
# applied to (typecode, list), and from protocol 3 up array._array_reconstructor
# applied to the raw machine bytes. Both callables and the array type itself go out
# as bare global references (array.array, array._array_reconstructor) and resolve
# back to the module's own objects, so the array round-trips at every binary protocol.
cases = [
    array.array("i", [1, 2, 3]),
    array.array("d", [1.5, -2.25, 0.0]),
    array.array("b", [-1, 0, 127]),
    array.array("B", [0, 255, 128]),
    array.array("h", [-30000, 30000]),
    array.array("l", [10 ** 9, -10 ** 9]),
    array.array("q", [10 ** 18, -10 ** 18]),
    array.array("f", [1.5, -0.5]),
    array.array("u", "wxyz"),
    array.array("i", []),
]

for a in cases:
    for proto in range(2, 6):
        data = pickle.dumps(a, protocol=proto)
        back = pickle.loads(data)
        print(proto, a.typecode, list(back), back == a, len(data))

# The array type and the reconstructor pickle as bare globals that come back as the
# very same objects, not copies.
print(pickle.loads(pickle.dumps(array.array)) is array.array)
print(pickle.loads(pickle.dumps(array._array_reconstructor)) is array._array_reconstructor)

# An array referenced twice is written once and both slots recover the identical
# object, the way the pickler's memo shares any value.
pair = pickle.loads(pickle.dumps((cases[0], cases[0])))
print(pair[0] is pair[1], list(pair[0]))

# A pickle produced at one protocol loads back the same at another, and an empty
# array keeps its typecode across the round-trip.
empty = pickle.loads(pickle.dumps(array.array("f"), protocol=5))
print(empty.typecode, len(empty), empty == array.array("f"))
