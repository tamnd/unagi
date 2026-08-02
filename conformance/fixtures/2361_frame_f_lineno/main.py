import sys

# f_lineno on a frame tracks the line currently executing, advanced as each
# statement runs the way CPython updates it, so sys._getframe().f_lineno reads
# the live line rather than the def line. A stdlib stacklevel walk (warnings.warn,
# gettext) reads it off the frame it lands on to attribute a call to the right
# line. A nested body shares the enclosing frame and does not push its own, so its
# line never leaks onto that frame.


def caller_line():
    return sys._getframe(1).f_lineno


def run():
    first = caller_line()
    second = caller_line()
    third = caller_line()
    return first, second, third


print("caller lines:", run())
print("module line:", sys._getframe(0).f_lineno)

import warnings


def emit():
    warnings.warn("boom", stacklevel=2)


with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter("always")
    emit()
    rec = w[0]
    print("warn file:", rec.filename.split("/")[-1])
    print("warn line:", rec.lineno)
    print("warn category:", rec.category.__name__)
