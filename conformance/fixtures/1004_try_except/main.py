try:
    x = 1 / 0
except ZeroDivisionError:
    print("caught zero division")
try:
    xs = [1, 2, 3]
    print(xs[7])
except IndexError as e:
    print("index:", e)
try:
    1 / 0
except ArithmeticError:
    print("arithmetic base catches")
try:
    int("nope")
except (TypeError, ValueError) as e:
    print("tuple:", e)
try:
    d = {}
    print(d["k"])
except Exception as e:
    print("exception base:", e)
try:
    print("no error here")
except ValueError:
    print("not reached")
else:
    print("else runs")
try:
    1 / 0
except ZeroDivisionError:
    print("handled")
else:
    print("else skipped")
try:
    y = [1][5]
except:
    print("bare except")
