import box

print(type(box).__name__)
print(box.__name__)
print(box.Thing)
print(repr(box))

box.stored = "new"
print(box.read_stored())

box.injected = 42
print(box.read_injected())
print(box.injected)

del box.stored
try:
    print(box.stored)
except AttributeError as e:
    print("read:", e)
try:
    del box.stored
except AttributeError as e:
    print("del:", e)
try:
    del box.never
except AttributeError as e:
    print("del2:", e)

box.stored = "back"
print(box.read_stored())

def probe():
    try:
        return box.gone
    except AttributeError as e:
        return "func: " + str(e)

print(probe())
