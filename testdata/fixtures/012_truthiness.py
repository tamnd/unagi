if "":
    print("no")
if "x":
    print("nonempty string")
if []:
    print("no")
if [0]:
    print("nonempty list")
if 0:
    print("no")
if 0.0:
    print("no")
if -1:
    print("nonzero")
if None:
    print("no")
if {}:
    print("no")
if {"k": 0}:
    print("nonempty dict")
print(True and "yes")
print(False and "yes")
print(0 or "fallback")
print("first" or "second")
print(not 0)
print(not "x")
print(bool(""))
print(bool([1]))
print(None or 0 or "" or [] or "last")
print(1 and 2 and 3)
print(int("42") + 1)
print(int(3.9))
print(int(-3.9))
print(float("2.5"))
print(float(7))
print(str(3.0))
print(str(None))
print(repr([1, "a", None, True]))
print(repr((1,)))
print(repr({"k": [1, 2]}))
