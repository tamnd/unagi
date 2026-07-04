try:
    print("body")
finally:
    print("finally after clean body")
try:
    1 / 0
except ZeroDivisionError:
    print("handled")
finally:
    print("finally after handled")
def f():
    try:
        return "returned from try"
    finally:
        print("finally before return")
print(f())
def g():
    try:
        raise ValueError("gone")
    finally:
        print("finally on the way out")
try:
    g()
except ValueError as e:
    print("caught after finally:", e)
for i in range(5):
    try:
        if i == 1:
            continue
        if i == 3:
            break
        print("loop body", i)
    finally:
        print("loop finally", i)
print("after loop")
