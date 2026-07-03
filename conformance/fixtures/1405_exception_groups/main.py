# ExceptionGroup construction, str/repr, matching, and add_note.
# Wordings probed on 3.14; both classes share BaseExceptionGroup.__new__.

try:
    ExceptionGroup("m", [])
except ValueError as e:
    print(e)
try:
    ExceptionGroup(1, [ValueError(1)])
except TypeError as e:
    print(e)
try:
    ExceptionGroup("m", [ValueError(1), 42])
except ValueError as e:
    print(e)
try:
    ExceptionGroup("m", "abc")
except ValueError as e:
    print(e)
try:
    ExceptionGroup("m", 42)
except TypeError as e:
    print(e)
try:
    ExceptionGroup("m", [KeyboardInterrupt()])
except TypeError as e:
    print(e)
try:
    ExceptionGroup("m")
except TypeError as e:
    print(e)
try:
    raise ExceptionGroup
except TypeError as e:
    print(e)

eg = ExceptionGroup("boom", [ValueError(1), TypeError("x")])
print(str(eg))
print(repr(eg))
one = BaseExceptionGroup("solo", (KeyError("k"),))
print(str(one))
print(repr(one))
try:
    raise one
except ExceptionGroup as e:
    print("caught as ExceptionGroup:", e)
try:
    raise eg
except Exception as e:
    print("caught as Exception:", e)
base = BaseExceptionGroup("mixed", [KeyboardInterrupt(), ValueError(9)])
print(repr(base))

e2 = ValueError("v")
print(e2.add_note("n1"))
e2.add_note("n2")
try:
    e2.add_note(3)
except TypeError as e:
    print(e)
try:
    e2.add_note()
except TypeError as e:
    print(e)
try:
    e2.add_note("a", "b")
except TypeError as e:
    print(e)
