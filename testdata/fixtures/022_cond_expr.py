x = 5
y = x if x > 3 else -x
print(y)
print("big" if x > 10 else "small")

def sign(n):
    return "pos" if n > 0 else ("zero" if n == 0 else "neg")

print(sign(4), sign(0), sign(-2))

def loud(tag):
    print("eval", tag)
    return tag

r = loud("then") if x > 0 else loud("else")
print(r)

n = 0
label = "zero" if n == 0 else str(1 // n)
print(label)

vals = [3, 0, -7]
for v in vals:
    print(v, "even" if v % 2 == 0 else "odd")
