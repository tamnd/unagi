# _io.text_encoding is the helper the higher-level text APIs use to settle a
# possibly-None encoding argument. Given None it returns the default text
# encoding, otherwise it hands back what it was passed. The default is "locale"
# outside UTF-8 mode, which the oracle (and this build) always is. stacklevel
# only positions a possible EncodingWarning, which is off by default, so it is
# validated as an int and otherwise ignored. This is sub-slice 5h-5 of the _io
# arc (Spec 2076 stdlib S0_io_arc.md); open/open_code are deferred with FileIO
# (5f) since they build a raw file stream and need real fds.
import _io

# None settles to the default text encoding; a concrete encoding passes through.
print(repr(_io.text_encoding(None)))
print(repr(_io.text_encoding("utf-8")))
print(repr(_io.text_encoding("ascii")))

# stacklevel is accepted positionally and does not change the result.
print(repr(_io.text_encoding(None, 1)))
print(repr(_io.text_encoding("latin-1", 3)))
print(repr(_io.text_encoding(None, True)))

# a non-None encoding is returned unchanged without a type check.
print(_io.text_encoding([1, 2]))

# too few or too many arguments.
try:
    _io.text_encoding()
except TypeError as e:
    print("no args:", e)
try:
    _io.text_encoding(None, 2, 3)
except TypeError as e:
    print("too many:", e)

# stacklevel must be an integer.
try:
    _io.text_encoding(None, None)
except TypeError as e:
    print("stacklevel None:", e)
try:
    _io.text_encoding(None, 2.0)
except TypeError as e:
    print("stacklevel float:", e)
