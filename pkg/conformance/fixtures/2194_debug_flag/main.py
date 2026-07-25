print(__debug__)
print(type(__debug__).__name__)
if __debug__:
    print("asserts active")

def guarded():
    if __debug__:
        return "checked"
    return "fast"
print(guarded())

flag = __debug__ and "yes" or "no"
print(flag)

import imaplib
print("imaplib", imaplib.Debug)
