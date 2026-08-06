from array import array


def show(label, e):
    try:
        print(label, e())
    except Exception as ex:
        print(label, type(ex).__name__, ex)


# A memoryview compares equal by format-decoded elements, not raw bytes, so a
# typed view and a bytes or array holding the same logical values are equal even
# when their raw byte widths differ.
show("iB", lambda: array("i", [97, 98]) == memoryview(b"ab"))
show("Bi", lambda: memoryview(b"ab") == array("i", [97, 98]))
show("float-int-view", lambda: memoryview(array("f", [1.0, 2.0])) == memoryview(array("i", [1, 2])))
show("float-int-arr", lambda: memoryview(array("f", [1.0, 2.0])) == array("i", [1, 2]))
show("signed-unsigned", lambda: memoryview(array("H", [1, 2])) == memoryview(array("h", [1, 2])))
show("bytes-vs-typed", lambda: b"ab" == memoryview(array("i", [97, 98])))
show("typed-vs-bytes", lambda: memoryview(array("i", [97, 98])) == b"ab")

# The comparison is genuinely order sensitive where the two sides answer with
# different rules: a bytearray compares its raw bytes so a wider typed view whose
# byte length differs is unequal, while the same view on the left compares its
# decoded elements and is equal.
show("ba-vs-typed", lambda: bytearray(b"ab") == memoryview(array("i", [97, 98])))
show("typed-vs-ba", lambda: memoryview(array("i", [97, 98])) == bytearray(b"ab"))

# Different element counts or values are unequal, and a raw byte pattern that
# does not decode to the view's elements is unequal too.
show("len-diff", lambda: memoryview(b"ab") == array("i", [97]))
show("empty", lambda: memoryview(b"") == array("i", []))
show("float-raw-bytes", lambda: b"\x00\x00\x80\x3f" == memoryview(array("f", [1.0])))
show("neq-values", lambda: memoryview(b"ab") == array("b", [97, 99]))

# A non-buffer operand is simply unequal, never an error, on either side.
show("mv-vs-list", lambda: memoryview(b"ab") == [97, 98])
show("mv-vs-str", lambda: memoryview(b"ab") == "ab")
show("mv-vs-int", lambda: memoryview(b"ab") == 5)
show("ne-typed", lambda: array("i", [97, 98]) != memoryview(b"ab"))

# A released view compares equal only to the very same object, never raising.
m = memoryview(bytearray(b"ab"))
m.release()
show("released-self", lambda: m == m)
show("released-other", lambda: m == memoryview(b"ab"))
show("released-bytes", lambda: m == b"ab")
show("released-ne-self", lambda: m != m)

# The bytes/bytearray/array rules the memoryview interplay rides on stay intact:
# bytes reads only a bytes or bytearray, an array only an array, and bytearray
# any buffer by raw bytes.
show("bytes-eq-ba", lambda: b"ab" == bytearray(b"ab"))
show("bytes-ne-array", lambda: b"ab" == array("b", [97, 98]))
show("array-eq-ba", lambda: array("b", [97, 98]) == bytearray(b"ab"))
show("array-ne-bytes", lambda: array("b", [97, 98]) == b"ab")
