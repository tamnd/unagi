print("loading util")

count = 0

def bump():
    global count
    count = count + 1
    return count

def greet(name, punct="!"):
    return "hi " + name + punct

class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def norm(self):
        return self.x * self.x + self.y * self.y
