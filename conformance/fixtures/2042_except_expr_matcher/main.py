# An except clause matches on whatever the matcher expression evaluates to, not
# just a bare name: an attribute like os.error, a tuple mixing an attribute with
# a name, and a subscript into a tuple of classes all reach their value at match
# time the way CPython runs the clause. A matcher that is not an exception class
# raises the same TypeError CPython raises, chained on the in-flight exception.
import os

try:
    raise OSError("disk")
except os.error as e:
    print("attr matcher:", e)

try:
    raise ValueError("v")
except (KeyError, os.error, ValueError) as e:
    print("tuple attr matcher:", e)

excs = (KeyError, ValueError)
try:
    raise ValueError("s")
except excs[1] as e:
    print("subscript matcher:", e)

try:
    raise TypeError("t")
except 1:
    print("unreachable")
