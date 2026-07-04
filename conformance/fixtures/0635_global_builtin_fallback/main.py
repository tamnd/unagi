def measure():
    return len([1, 2, 3])


cond = False
if cond:
    len = 99

print(measure())

len = 5
print(len)
del len
print(len([9, 8]))


def grab():
    global tag
    return tag


tag = "set"
print(grab())
del tag
try:
    grab()
except NameError as e:
    print("NameError:", e)


max = 100
print(max)
del max
print(max(3, 7))
