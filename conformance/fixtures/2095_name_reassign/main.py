# A module that reassigns __name__ (as _pydecimal does for pickling) must still
# read the pre-populated module name before the assignment runs.
before = __name__
print("before:", before)

__name__ = "decimal"
print("after:", __name__)


def who():
    return __name__


print("who:", who())

# A conditional reassignment still leaves the earlier read valid.
if len(before) > 0:
    __name__ = "renamed"
print("final:", __name__)
